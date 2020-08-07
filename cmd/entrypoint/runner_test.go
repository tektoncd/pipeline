package main

import (
	"context"
	"os"
	"syscall"
	"testing"
)

// TestRealRunnerSignalForwarding will artificially put an interrupt signal (SIGINT) in the rr.signals chan.
// The chan will not be reinitialized in the runner considering we have already initialized it here.
// Once the sleep process starts, if the signal is successfully received by the parent process, it
// will interrupt and stop the sleep command.
func TestRealRunnerSignalForwarding(t *testing.T) {
	rr := realRunner{}
	rr.signals = make(chan os.Signal, 1)
	rr.signals <- syscall.SIGINT
	if err := rr.Run("", "sleep", "3600"); err.Error() == "signal: interrupt" {
		t.Logf("SIGINT forwarded to Entrypoint")
	} else {
		t.Fatalf("Unexpected error received: %v", err)
	}
}

// TestRealRunnerTimeout tests whether cmd is killed after a millisecond even though it's supposed to sleep for 10 milliseconds.
func TestRealRunnerTimeout(t *testing.T) {
	rr := realRunner{}
	timeout := "1ms"
	if err := rr.Run(timeout, "sleep", "0.01"); err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("unexpected error received: %v", err)
		}
	} else {
		t.Fatalf("step didn't timeout")
	}
}
