package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	rr.signal(syscall.SIGINT)
	if err := rr.Run(context.Background(), "sleep", "3600"); err.Error() == "signal: interrupt" {
		t.Logf("SIGINT forwarded to Entrypoint")
	} else {
		t.Fatalf("Unexpected error received: %v", err)
	}
}

func TestRealRunnerStdoutAndStderrPaths(t *testing.T) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.RemoveAll(tmp)

	expectedString := "hello world"
	rr := realRunner{
		stdoutPath: filepath.Join(tmp, "stdout"),
		stderrPath: filepath.Join(tmp, "subpath/stderr"),
	}
	if err := rr.Run(context.Background(), "sh", "-c", fmt.Sprintf("echo %s && echo %s >&2", expectedString, expectedString)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for _, path := range []string{"stdout", "subpath/stderr"} {
		if got, err := os.ReadFile(filepath.Join(tmp, path)); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		} else if gotString := strings.TrimSpace(string(got)); gotString != expectedString {
			t.Errorf("%v: got: %v, wanted: %v", path, gotString, expectedString)
		}
	}
}

func TestRealRunnerStdoutAndStderrSamePath(t *testing.T) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.RemoveAll(tmp)

	path := filepath.Join(tmp, "logs")
	expectedString := "hello world"
	rr := realRunner{
		stdoutPath: path,
		stderrPath: path,
	}
	if err := rr.Run(context.Background(), "sh", "-c", fmt.Sprintf("echo %s && echo %s >&2", expectedString, expectedString)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Since writes to stdout and stderr might be racy, we only check for lengths here.
	expectedSize := (len(expectedString) + 1) * 2
	if got, err := os.ReadFile(path); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	} else if gotSize := len(got); gotSize != expectedSize {
		t.Errorf("got: %v, wanted: %v", gotSize, expectedSize)
	}
}

func TestRealRunnerStdoutPathWithSignal(t *testing.T) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.RemoveAll(tmp)

	path := filepath.Join(tmp, "stdout")
	rr := realRunner{
		signals:    make(chan os.Signal, 1),
		stdoutPath: path,
	}

	expectedString := "hello world"
	expectedError := "signal: interrupt"
	go func() {
		timer := time.Tick(100 * time.Millisecond)
		for {
			if stat, err := os.Stat(path); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					t.Errorf("Unexpected error: %v", err)
					return
				}
			} else if int(stat.Size()) > len(expectedString) {
				break
			}
			<-timer
		}
		rr.signal(syscall.SIGINT)
	}()

	if err := rr.Run(context.Background(), "sh", "-c", fmt.Sprintf("echo %s && sleep 20", expectedString)); err == nil || err.Error() != expectedError {
		t.Fatalf("Expected error %v but got %v", expectedError, err)
	}
	if got, err := os.ReadFile(path); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	} else if gotString := strings.TrimSpace(string(got)); gotString != expectedString {
		t.Errorf("got: %v, wanted: %v", gotString, expectedString)
	}
}

// TestRealRunnerTimeout tests whether cmd is killed after a millisecond even though it's supposed to sleep for 10 milliseconds.
func TestRealRunnerTimeout(t *testing.T) {
	rr := realRunner{}
	timeout := time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := rr.Run(ctx, "sleep", "0.01"); err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("unexpected error received: %v", err)
		}
	} else {
		t.Fatalf("step didn't timeout")
	}
}
