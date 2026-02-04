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
	TektonHermeticEnvVar               = "TEKTON_HERMETIC"
	secretMaskingDelayWarningThreshold = 64 * 1024
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
	secretMaskFile string
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

	secrets, err := loadSecretsForMasking(rr.secretMaskFile)
	if err != nil {
		return err
	}
	maskingEnabled := hasMaskableSecrets(secrets)
	var maskingWriters []*maskingWriter
	maxSecretLen := 0

	// if a standard output file is specified
	// create the log file and add to the std multi writer
	if rr.stdoutPath != "" {
		stdout, err := newStdLogWriter(rr.stdoutPath)
		if err != nil {
			return err
		}
		defer stdout.Close()
		if maskingEnabled {
			stdoutBase := newMaskingWriter(os.Stdout, secrets)
			stdoutFile := newMaskingWriter(stdout, secrets)
			maxSecretLen = stdoutBase.MaxSecretLen()
			maskingWriters = append(maskingWriters, stdoutBase, stdoutFile)
			cmd.Stdout = io.MultiWriter(stdoutBase, stdoutFile)
		} else {
			cmd.Stdout = io.MultiWriter(os.Stdout, stdout)
		}
	} else {
		if maskingEnabled {
			stdoutBase := newMaskingWriter(os.Stdout, secrets)
			maxSecretLen = stdoutBase.MaxSecretLen()
			maskingWriters = append(maskingWriters, stdoutBase)
			cmd.Stdout = stdoutBase
		} else {
			cmd.Stdout = os.Stdout
		}
	}
	if rr.stderrPath != "" {
		stderr, err := newStdLogWriter(rr.stderrPath)
		if err != nil {
			return err
		}
		defer stderr.Close()
		if maskingEnabled {
			stderrBase := newMaskingWriter(os.Stderr, secrets)
			stderrFile := newMaskingWriter(stderr, secrets)
			maskingWriters = append(maskingWriters, stderrBase, stderrFile)
			cmd.Stderr = io.MultiWriter(stderrBase, stderrFile)
		} else {
			cmd.Stderr = io.MultiWriter(os.Stderr, stderr)
		}
	} else {
		if maskingEnabled {
			stderrBase := newMaskingWriter(os.Stderr, secrets)
			maskingWriters = append(maskingWriters, stderrBase)
			cmd.Stderr = stderrBase
		} else {
			cmd.Stderr = os.Stderr
		}
	}

	if shouldWarnSecretMaskingDelay(maxSecretLen) {
		if _, err := io.WriteString(os.Stderr, secretMaskingDelayWarning(maxSecretLen)); err != nil {
			return err
		}
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
	waitErr := cmd.Wait()
	var flushErr error
	if maskingEnabled {
		flushErr = flushMaskingWriters(maskingWriters)
	}
	if waitErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return entrypoint.ErrContextDeadlineExceeded
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return entrypoint.ErrContextCanceled
		}
		return waitErr
	}
	if flushErr != nil {
		return flushErr
	}

	return nil
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

func loadSecretsForMasking(filePath string) ([]string, error) {
	if filePath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var secrets []string
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(line)
		if err != nil {
			return nil, fmt.Errorf("decode secret mask entry %d: %w", i+1, err)
		}
		secrets = append(secrets, string(decoded))
	}
	return secrets, nil
}

func flushMaskingWriters(maskingWriters []*maskingWriter) error {
	for _, w := range maskingWriters {
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func hasMaskableSecrets(secrets []string) bool {
	for _, s := range secrets {
		if len(s) >= 3 {
			return true
		}
	}
	return false
}

func shouldWarnSecretMaskingDelay(maxSecretLen int) bool {
	if maxSecretLen <= 0 {
		return false
	}
	return maxSecretLen-1 >= secretMaskingDelayWarningThreshold
}

func secretMaskingDelayWarning(maxSecretLen int) string {
	return fmt.Sprintf("Warning: secret masking enabled; largest secret is %d bytes; log output may be delayed by up to %d bytes.\n", maxSecretLen, maxSecretLen-1)
}
