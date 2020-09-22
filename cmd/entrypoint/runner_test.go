package main

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

// TestRealRunnerSignalForwarding will artificially put an interrupt signal (SIGINT) in the rr.signals chan.
// The chan will not be reinitialized in the runner considering we have already initialized it here.
// Once the sleep process starts, if the signal is successfully received by the parent process, it
// will interrupt and stop the sleep command.
func TestRealRunnerSignalForwarding(t *testing.T) {
	rr := realRunner{}
	rr.signals = make(chan os.Signal, 1)
	rr.signals <- syscall.SIGINT
	if err := rr.Run("", "", "sleep", "3600"); err.Error() == "signal: interrupt" {
		t.Logf("SIGINT forwarded to Entrypoint")
	} else {
		t.Fatalf("Unexpected error received: %v", err)
	}

}

// TestRealRunnerTimeout tests whether cmd is killed after a millisecond even though it's supposed to sleep for 10 milliseconds.
func TestRealRunnerTimeout(t *testing.T) {
	rr := realRunner{}
	timeout := "1ms"
	deadline := time.Now().Add(time.Second * 10)
	if err := rr.Run(timeout, deadline.Format(time.UnixDate), "sleep", "0.01"); err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("unexpected error received: %v", err)
		}
	} else {
		t.Fatalf("step didn't timeout")
	}
}

// TestRealRunnerValidDeadline tests whether cmd is killed after nanosecond * 1000, set by TaskRun deadline, even though it's supposed to sleep for 10 milliseconds.
func TestRealRunnerValidDeadline(t *testing.T) {
	rr := realRunner{}
	timeout := "1s"
	taskRunDeadline := time.Now().Add(time.Nanosecond * 100).Format(time.UnixDate)
	if err := rr.Run(timeout, taskRunDeadline, "sleep", "0.01"); err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("unexpected error received: %v", err)
		}
	} else {
		t.Fatalf("step didn't timeout")
	}
}

// TestRealRunnerEmptyDeadline tests with valid Step timeout but empty TaskRun deadline.
func TestRealRunnerEmptyDeadline(t *testing.T) {
	rr := realRunner{}
	timeout := "1ms"
	taskRunDeadline := ""
	if err := rr.Run(timeout, taskRunDeadline, "sleep", "0.01"); err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("unexpected error received: %v", err)
		}
	} else {
		t.Fatalf("step didn't timeout")
	}
}

// TestRealRunnerNegativeStepTimeout tests with negative Step timeout
func TestRealRunnerNegativeStepTimeout(t *testing.T) {
	rr := realRunner{}
	timeout := "-1ms"
	taskRunDeadline := time.Now().Add(time.Nanosecond * 100).Format(time.UnixDate)
	if err := rr.Run(timeout, taskRunDeadline, "sleep", "0.01"); err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("unexpected error received: %v", err)
		}
	} else {
		t.Fatalf("step didn't timeout")
	}
}

// TestRealRunnerEmptyStepTimeout tests with empty Step timeout and valid TaskRun deadline
func TestRealRunnerEmptyStepTimeout(t *testing.T) {
	rr := realRunner{}
	timeout := ""
	taskRunDeadline := time.Now().Add(time.Nanosecond * 100).Format(time.UnixDate)
	if err := rr.Run(timeout, taskRunDeadline, "sleep", "0.01"); err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("unexpected error received: %v", err)
		}
	} else {
		t.Fatalf("step didn't timeout")
	}
}
