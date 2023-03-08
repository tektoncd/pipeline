//go:build !windows
// +build !windows

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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
	"github.com/tektoncd/pipeline/pkg/pod"
)

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// realRunner actually runs commands.
type realRunner struct {
	sync.Mutex
	signals       chan os.Signal
	signalsClosed bool
	stdoutPath    string
	stderrPath    string
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

	// if a standard output file is specified
	// create the log file and add to the std multi writer
	if rr.stdoutPath != "" {
		stdout, err := newStdLogWriter(rr.stdoutPath)
		if err != nil {
			return err
		}
		defer stdout.Close()
		cmd.Stdout = io.MultiWriter(os.Stdout, stdout)
	} else {
		cmd.Stdout = os.Stdout
	}
	if rr.stderrPath != "" {
		stderr, err := newStdLogWriter(rr.stderrPath)
		if err != nil {
			return err
		}
		defer stderr.Close()
		cmd.Stderr = io.MultiWriter(os.Stderr, stderr)
	} else {
		cmd.Stderr = os.Stderr
	}

	// dedicated PID group used to forward signals to
	// main process and all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if os.Getenv("TEKTON_RESOURCE_NAME") == "" && os.Getenv(pod.TektonHermeticEnvVar) == "1" {
		dropNetworking(cmd)
	}

	// Start defined command
	if err := cmd.Start(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return context.DeadlineExceeded
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
	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return context.DeadlineExceeded
		}
		return err
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
