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
	"io"
	"log"
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
	stopCh := make(chan struct{}, 1)
	defer close(stopCh)

	cmd.Stdout = os.Stdout
	var stdoutFile *os.File
	if rr.stdoutPath != "" {
		var err error
		var doneCh <-chan error
		// Create directory if it doesn't already exist
		if err = os.MkdirAll(filepath.Dir(rr.stdoutPath), os.ModePerm); err != nil {
			return err
		}
		if stdoutFile, err = os.Create(rr.stdoutPath); err != nil {
			return err
		}
		// We use os.Pipe in asyncWriter to copy stdout instead of cmd.StdoutPipe or providing an
		// io.Writer directly because otherwise Go would wait for the underlying fd to be closed by the
		// child process before returning from cmd.Wait even if the process is no longer running. This
		// would cause a deadlock if the child spawns a long running descendant process before exiting.
		if cmd.Stdout, doneCh, err = asyncWriter(io.MultiWriter(os.Stdout, stdoutFile), stopCh); err != nil {
			return err
		}
		go func() {
			if err := <-doneCh; err != nil {
				log.Fatalf("Copying stdout: %v", err)
			}
			stdoutFile.Close()
		}()
	}

	cmd.Stderr = os.Stderr
	var stderrFile *os.File
	if rr.stderrPath != "" {
		var err error
		var doneCh <-chan error
		if rr.stderrPath == rr.stdoutPath {
			fd, err := syscall.Dup(int(stdoutFile.Fd()))
			if err != nil {
				return err
			}
			stderrFile = os.NewFile(uintptr(fd), rr.stderrPath)
		} else {
			// Create directory if it doesn't already exist
			if err = os.MkdirAll(filepath.Dir(rr.stderrPath), os.ModePerm); err != nil {
				return err
			}
			if stderrFile, err = os.Create(rr.stderrPath); err != nil {
				return err
			}
		}
		// We use os.Pipe in asyncWriter to copy stderr instead of cmd.StderrPipe or providing an
		// io.Writer directly because otherwise Go would wait for the underlying fd to be closed by the
		// child process before returning from cmd.Wait even if the process is no longer running. This
		// would cause a deadlock if the child spawns a long running descendant process before exiting.
		if cmd.Stderr, doneCh, err = asyncWriter(io.MultiWriter(os.Stderr, stderrFile), stopCh); err != nil {
			return err
		}
		go func() {
			if err := <-doneCh; err != nil {
				log.Fatalf("Copying stderr: %v", err)
			}
			stderrFile.Close()
		}()
	}

	// dedicated PID group used to forward signals to
	// main process and all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if os.Getenv("TEKTON_RESOURCE_NAME") == "" && os.Getenv(pod.TektonHermeticEnvVar) == "1" {
		dropNetworking(cmd)
	}

	// Start defined command
	if err := cmd.Start(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
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
		if ctx.Err() == context.DeadlineExceeded {
			return context.DeadlineExceeded
		}
		return err
	}

	return nil
}
