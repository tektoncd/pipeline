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

	// Build a list of tee readers that we'll read from after the command is
	// started. If we are not configured to tee stdout/stderr this will be
	// empty and contents will not be copied.
	var readers []*namedReader
	if rr.stdoutPath != "" {
		stdout, err := newTeeReader(cmd.StdoutPipe, rr.stdoutPath)
		if err != nil {
			return err
		}
		readers = append(readers, stdout)
	} else {
		// This needs to be set in an else since StdoutPipe will fail if cmd.Stdout is already set.
		cmd.Stdout = os.Stdout
	}
	if rr.stderrPath != "" {
		stderr, err := newTeeReader(cmd.StderrPipe, rr.stderrPath)
		if err != nil {
			return err
		}
		readers = append(readers, stderr)
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
	for _, r := range readers {
		wg.Add(1)
		// Read concurrently so that we can pipe stdout and stderr at the same
		// time.
		go func(r *namedReader) {
			defer wg.Done()
			if _, err := io.ReadAll(r); err != nil {
				log.Printf("error reading to %s: %v", r.name, err)
			}
		}(r)
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

// newTeeReader creates a new Reader that copies data from the given pipe function
// (i.e. cmd.StdoutPipe, cmd.StderrPipe) into a file specified by path.
// The file is opened with os.O_WRONLY|os.O_CREATE|os.O_APPEND, and will not
// override any existing content in the path. This means that the same file can
// be used for multiple streams if desired.
// The behavior of the Reader is the same as io.TeeReader - reads from the pipe
// will be written to the file.
func newTeeReader(pipe func() (io.ReadCloser, error), path string) (*namedReader, error) {
	in, err := pipe()
	if err != nil {
		return nil, fmt.Errorf("error creating pipe: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating parent directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", path, err)
	}

	return &namedReader{
		name:   path,
		Reader: io.TeeReader(in, f),
	}, nil
}

// namedReader is just a helper struct that lets us give a reader a name for
// logging purposes.
type namedReader struct {
	io.Reader
	name string
}
