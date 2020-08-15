package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// realRunner actually runs commands.
type realRunner struct {
	signals    chan os.Signal
	stdoutPath string
	stderrPath string
}

var _ entrypoint.Runner = (*realRunner)(nil)

func (rr *realRunner) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return nil
	}
	name, args := args[0], args[1:]

	// Receive system signals on "rr.signals"
	if rr.signals == nil {
		rr.signals = make(chan os.Signal, 1)
	}
	defer close(rr.signals)
	signal.Notify(rr.signals)
	defer signal.Reset()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	var stdoutFile *os.File
	if rr.stdoutPath != "" {
		var err error
		if stdoutFile, err = os.Create(rr.stdoutPath); err != nil {
			return err
		}
		defer stdoutFile.Close()
		// We use os.Pipe in newAsyncWriter to copy stdout instead of cmd.StdoutPipe or providing an
		// io.Writer directly because otherwise Go would wait for the underlying fd to be closed by the
		// child process before returning from cmd.Wait even if the process is no longer running. This
		// would cause a deadlock if the child spawns a long running descendant process before exiting.
		stopCh := make(chan struct{}, 1)
		defer close(stopCh)
		if cmd.Stdout, err = AsyncWriter(io.MultiWriter(os.Stdout, stdoutFile), stopCh); err != nil {
			return err
		}
	}

	cmd.Stderr = os.Stderr
	var stderrFile *os.File
	if rr.stderrPath != "" {
		var err error
		if rr.stderrPath == rr.stdoutPath {
			fd, err := syscall.Dup(int(stdoutFile.Fd()))
			if err != nil {
				return err
			}
			stderrFile = os.NewFile(uintptr(fd), rr.stderrPath)
		} else if stderrFile, err = os.Create(rr.stderrPath); err != nil {
			return err
		}
		defer stderrFile.Close()
		// We use os.Pipe in newAsyncWriter to copy stderr instead of cmd.StderrPipe or providing an
		// io.Writer directly because otherwise Go would wait for the underlying fd to be closed by the
		// child process before returning from cmd.Wait even if the process is no longer running. This
		// would cause a deadlock if the child spawns a long running descendant process before exiting.
		stopCh := make(chan struct{}, 1)
		defer close(stopCh)
		if cmd.Stderr, err = AsyncWriter(io.MultiWriter(os.Stderr, stderrFile), stopCh); err != nil {
			return err
		}
	}

	// dedicated PID group used to forward signals to
	// main process and all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
