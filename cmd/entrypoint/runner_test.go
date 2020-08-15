package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestRealRunnerSignalForwarding will artificially put an interrupt signal (SIGINT) in the rr.signals chan.
// The chan will not be reinitialized in the runner considering we have already initialized it here.
// Once the sleep process starts, if the signal is successfully received by the parent process, it
// will interrupt and stop the sleep command.
func TestRealRunnerSignalForwarding(t *testing.T) {
	rr := realRunner{
		signals: make(chan os.Signal, 1),
	}
	rr.signals <- syscall.SIGINT
	if err := rr.Run(context.Background(), "sleep", "3600"); err.Error() == "signal: interrupt" {
		t.Logf("SIGINT forwarded to Entrypoint")
	} else {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestRealRunnerStdoutAndStderrPaths(t *testing.T) {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.RemoveAll(tmp)

	expectedString := "hello world"
	rr := realRunner{
		stdoutPath: filepath.Join(tmp, "stdout"),
		stderrPath: filepath.Join(tmp, "stderr"),
	}
	if err := rr.Run(context.Background(), "sh", "-c", fmt.Sprintf("echo %s && echo %s >&2", expectedString, expectedString)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for _, file := range []string{"stdout", "stderr"} {
		if got, err := ioutil.ReadFile(filepath.Join(tmp, file)); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		} else if gotString := string(got[:len(got)-1]); gotString != expectedString {
			t.Errorf("%v: got: %v, wanted: %v", file, gotString, expectedString)
		}
	}
}

func TestRealRunnerStdoutAndStderrSamePath(t *testing.T) {
	tmp, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.RemoveAll(tmp.Name())
	defer tmp.Close()

	expectedString := "hello world"
	rr := realRunner{
		stdoutPath: tmp.Name(),
		stderrPath: tmp.Name(),
	}
	if err := rr.Run(context.Background(), "sh", "-c", fmt.Sprintf("echo %s && echo %s >&2", expectedString, expectedString)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Since writes to stdout and stderr might be racy, we only check for lengths here.
	expectedSize := (len(expectedString) + 1) * 2
	if stat, err := tmp.Stat(); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	} else if gotSize := int(stat.Size()); gotSize != expectedSize {
		t.Errorf("got: %v, wanted: %v", gotSize, expectedSize)
	}
}

func TestRealRunnerStdoutPathWithSignal(t *testing.T) {
	tmp, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	signals := make(chan os.Signal, 1)
	rr := realRunner{
		signals:    signals,
		stdoutPath: tmp.Name(),
	}

	expectedString := "hello world"
	expectedError := "signal: interrupt"
	go func() {
		timer := time.Tick(100 * time.Millisecond)
		for {
			if stat, err := tmp.Stat(); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			} else if int(stat.Size()) > len(expectedString) {
				break
			}
			<-timer
		}
		signals <- syscall.SIGINT
	}()

	if err := rr.Run(context.Background(), "sh", "-c", fmt.Sprintf("echo %s && sleep 3600", expectedString)); err.Error() != expectedError {
		t.Errorf("Expected error %v but got %v", expectedError, err)
	}
	if got, err := ioutil.ReadAll(tmp); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	} else if gotString := string(got[:len(got)-1]); gotString != expectedString {
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
