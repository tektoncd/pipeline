package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// realRunner actually runs commands.
type realRunner struct {
	signals chan os.Signal
}

var _ entrypoint.Runner = (*realRunner)(nil)

func (rr *realRunner) Run(timeout string, taskRunDeadline string, args ...string) error {
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

	// Add timeout to context if a non-zero timeout is specified for a step
	ctx := context.Background()
	var cancel context.CancelFunc

	if timeout != "" || taskRunDeadline != "" {
		timeoutDuration := soonestTimeout(timeout, taskRunDeadline)
		ctx, cancel = context.WithTimeout(ctx, timeoutDuration)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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

// soonestTimeout accepts a step timeout duration and taskrun timeout deadline in string format
// and returns whichever one is non-zero (if any) or that which will occur soonest.
func soonestTimeout(stepTimeout, taskRunDeadline string) time.Duration {
	taskRunEnd, _ := time.Parse(time.UnixDate, taskRunDeadline)
	taskRunDuration := time.Until(taskRunEnd)

	stepDuration, _ := time.ParseDuration(stepTimeout)

	if stepTimeout == "" || taskRunDeadline != "" && taskRunEnd.Before(time.Now().Add(stepDuration)) {
		return taskRunDuration
	}

	return stepDuration
}
