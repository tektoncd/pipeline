//go:build !windows

/*
Copyright 2021 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

const (
	TektonHermeticEnvVar = "TEKTON_HERMETIC"
)

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// realRunner actually runs commands.
type realRunner struct {
	sync.Mutex
	signals        chan os.Signal
	signalsClosed  bool
	stdoutPath     string
	stderrPath     string
	secretMaskDir  string
	redactPatterns string
}

var _ entrypoint.Runner = (*realRunner)(nil)

// close closes the signals channel which is used to receive system signals.
func (rr *realRunner) close() {
	rr.Lock()
	defer rr.Unlock()
	if rr.signals != nil && !rr.signalsClosed {
		close(rr.signals)
		rr.signalsClosed = true
	}
}

// signal allows the caller to simulate the sending of a system signal.
func (rr *realRunner) signal(signal os.Signal) {
	rr.Lock()
	defer rr.Unlock()
	if rr.signals != nil && !rr.signalsClosed {
		rr.signals <- signal
	}
}

// Run executes the entrypoint.
func (rr *realRunner) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return nil
	}
	name, args := args[0], args[1:]

	// Receive system signals on "rr.signals"
	if rr.signals == nil {
		rr.signals = make(chan os.Signal, 1)
	}
	defer rr.close()
	signal.Notify(rr.signals)
	defer signal.Reset()

	cmd := exec.CommandContext(ctx, name, args...)

	// Load redaction config: regex patterns and exact secret values.
	regexPatterns, exactSecrets := rr.loadMaskingConfig()
	makeMW := func(w io.Writer) *maskingWriter {
		if len(regexPatterns) > 0 || len(exactSecrets) > 0 {
			return newMaskingWriterWithConfig(w, regexPatterns, exactSecrets)
		}
		return newMaskingWriter(w)
	}

	stdoutMask := makeMW(os.Stdout)
	defer stdoutMask.Flush()
	if rr.stdoutPath != "" {
		stdout, err := newStdLogWriter(rr.stdoutPath)
		if err != nil {
			return err
		}
		defer stdout.Close()
		stdoutFileMask := makeMW(stdout)
		defer stdoutFileMask.Flush()
		cmd.Stdout = io.MultiWriter(stdoutMask, stdoutFileMask)
	} else {
		cmd.Stdout = stdoutMask
	}
	stderrMask := makeMW(os.Stderr)
	defer stderrMask.Flush()
	if rr.stderrPath != "" {
		stderr, err := newStdLogWriter(rr.stderrPath)
		if err != nil {
			return err
		}
		defer stderr.Close()
		stderrFileMask := makeMW(stderr)
		defer stderrFileMask.Flush()
		cmd.Stderr = io.MultiWriter(stderrMask, stderrFileMask)
	} else {
		cmd.Stderr = stderrMask
	}

	// dedicated PID group used to forward signals to
	// main process and all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if os.Getenv("TEKTON_RESOURCE_NAME") == "" && os.Getenv(TektonHermeticEnvVar) == "1" {
		dropNetworking(cmd)
	}

	// Start defined command
	if err := cmd.Start(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return entrypoint.ErrContextDeadlineExceeded
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return entrypoint.ErrContextCanceled
		}
		return err
	}

	// Goroutine for signals forwarding
	go func() {
		for s := range rr.signals {
			// Forward signal to main process and all children
			if s != syscall.SIGCHLD {
				_ = syscall.Kill(-cmd.Process.Pid, s.(syscall.Signal))
			}
		}
	}()

	// Wait for command to exit
	// as os.exec [note](https://github.com/golang/go/blob/ee522e2cdad04a43bc9374776483b6249eb97ec9/src/os/exec/exec.go#L897-L906)
	// cmd.Wait prefer Process error over context error
	// but we want to return context error instead
	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return entrypoint.ErrContextDeadlineExceeded
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return entrypoint.ErrContextCanceled
		}
		return err
	}

	return nil
}

// loadMaskingConfig reads redaction configuration from the flags set by the
// controller. It returns regex patterns (decoded from base64) and exact secret
// values (read from files under secretMaskDir).
func (rr *realRunner) loadMaskingConfig() (regexPatterns []string, exactSecrets []string) {
	if rr.redactPatterns != "" {
		decoded, err := base64.StdEncoding.DecodeString(rr.redactPatterns)
		if err != nil {
			log.Printf("Warning: failed to decode redact_patterns: %v", err)
		} else {
			for _, line := range strings.Split(string(decoded), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					regexPatterns = append(regexPatterns, line)
				}
			}
		}
	}

	if rr.secretMaskDir != "" {
		exactSecrets = loadSecretsFromDir(rr.secretMaskDir)
	}

	return regexPatterns, exactSecrets
}

// loadSecretsFromDir walks the secret mask directory and reads every file's
// content as an exact string to redact. Each mounted Secret becomes a
// subdirectory; each key in the Secret becomes a file.
func loadSecretsFromDir(dir string) []string {
	var secrets []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// Skip Kubernetes-internal symlinks (e.g. ..data, ..2024_01_01)
		if strings.HasPrefix(info.Name(), "..") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Warning: failed to read secret file %s: %v", path, err)
			return nil
		}
		val := strings.TrimSpace(string(data))
		if val != "" {
			secrets = append(secrets, val)
		}
		return nil
	})
	if err != nil {
		log.Printf("Warning: failed to walk secret mask dir %s: %v", dir, err)
	}
	return secrets
}

// newStdLogWriter create a new file writer that used for collecting std log
// the file is opened with os.O_WRONLY|os.O_CREATE|os.O_APPEND, and will not
// override any existing content in the path. This means that the same file can
// be used for multiple streams if desired. note that close after use
func newStdLogWriter(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating parent directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", path, err)
	}

	return f, nil
}
