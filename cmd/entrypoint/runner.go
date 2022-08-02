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
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
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

	// dedicated PID group used to forward signals to
	// main process and all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if os.Getenv("TEKTON_RESOURCE_NAME") == "" && os.Getenv(pod.TektonHermeticEnvVar) == "1" {
		dropNetworking(cmd)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
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

	wg := new(sync.WaitGroup)
	if rr.stdoutPath != "" {
		if err := tee(stdout, rr.stdoutPath, wg); err != nil {
			return err
		}
	}
	if rr.stderrPath != "" {
		if err := tee(stderr, rr.stderrPath, wg); err != nil {
			return err
		}
	}

	// Wait for stdout/err buffers to finish reading before returning.
	wg.Wait()

	// Wait for command to exit
	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return context.DeadlineExceeded
		}
		return err
	}

	return nil
}

// tee copies data from a Reader into a file specified by path.
// The file is opened with os.O_WRONLY|os.O_CREATE|os.O_APPEND, and will not
// override any existing content in the path. This means that the same file can
// be used for multiple streams if desired.
// This func will spawn a goroutine that will copy data into the file
// asynchronously, and will call WaitGroup.Done() once the Reader is closed and
// the copy is complete.
func tee(in io.ReadCloser, path string, wg *sync.WaitGroup) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		wg.Done()
		return fmt.Errorf("error opening %s: %w", path, err)
	}

	wg.Add(1)
	go func() {
		if _, err := io.Copy(f, in); err != nil {
			log.Printf("error copying to %s: %v", path, err)
		}
		f.Close()
		wg.Done()
	}()

	return nil
}
