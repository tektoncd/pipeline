/*
Copyright 2023 The Tekton Authors

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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// TestRealRunnerSignalForwarding will artificially put an interrupt signal (SIGINT) in the rr.signals chan.
// The chan will not be reinitialized in the runner considering we have already initialized it here.
// Once the sleep process starts, if the signal is successfully received by the parent process, it
// will interrupt and stop the sleep command.
func TestRealRunnerSignalForwarding(t *testing.T) {
	rr := realRunner{}
	rr.signals = make(chan os.Signal, 1)
	rr.signal(syscall.SIGINT)
	if err := rr.Run(t.Context(), "sleep", "3600"); err.Error() == "signal: interrupt" {
		t.Logf("SIGINT forwarded to Entrypoint")
	} else {
		t.Fatalf("Unexpected error received: %v", err)
	}
}

func TestRealRunnerStdoutAndStderrPaths(t *testing.T) {
	tmp := t.TempDir()

	expectedString := "hello world"
	rr := realRunner{
		stdoutPath: filepath.Join(tmp, "stdout"),
		stderrPath: filepath.Join(tmp, "subpath/stderr"),
	}

	// capture the std{out/err} output to verify whether we print log in the std
	oldStdout := os.Stdout // keep backup of the real stdout
	outReader, outWriter, _ := os.Pipe()
	os.Stdout = outWriter

	oldStderr := os.Stderr
	errReader, errWriter, _ := os.Pipe()
	os.Stderr = errWriter

	if err := rr.Run(t.Context(), "sh", "-c", fmt.Sprintf("echo %s && echo %s >&2", expectedString, expectedString)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	outC := make(chan string)
	errC := make(chan string)
	// copy the output in a separate goroutine so realRunner command can't block indefinitely
	go func() {
		var stdOutBuf bytes.Buffer
		io.Copy(&stdOutBuf, outReader)
		outC <- stdOutBuf.String()

		var stdErrBuf bytes.Buffer
		io.Copy(&stdErrBuf, errReader)
		errC <- stdErrBuf.String()
	}()
	// back to normal state
	outWriter.Close()
	errWriter.Close()
	os.Stdout = oldStdout // restoring the real stdout
	os.Stderr = oldStderr // restoring the real stderr
	stdOut := <-outC
	stdErr := <-errC

	// echo command will auto add \n in end, so we should remove trailing newline
	if strings.TrimSuffix(stdOut, "\n") != expectedString {
		t.Fatalf("Unexpected stdout output: %s, wanted stdout output: %s", stdOut, expectedString)
	}
	if strings.TrimSuffix(stdErr, "\n") != expectedString {
		t.Fatalf("Unexpected stderr output: %s, wanted stderr output: %s", stdErr, expectedString)
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
	tmp := t.TempDir()

	path := filepath.Join(tmp, "logs")
	expectedString := "hello world"
	rr := realRunner{
		stdoutPath: path,
		stderrPath: path,
	}
	if err := rr.Run(t.Context(), "sh", "-c", fmt.Sprintf("echo %s && echo %s >&2", expectedString, expectedString)); err != nil {
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
	tmp := t.TempDir()

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

	if err := rr.Run(t.Context(), "sh", "-c", fmt.Sprintf("echo %s && sleep 20", expectedString)); err == nil || err.Error() != expectedError {
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
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	if err := rr.Run(ctx, "sleep", "0.01"); err != nil {
		if !errors.Is(err, entrypoint.ErrContextDeadlineExceeded) {
			t.Fatalf("unexpected error received: %v", err)
		}
	} else {
		t.Fatalf("step didn't timeout")
	}
}

func TestRealRunnerCancel(t *testing.T) {
	testCases := []struct {
		name    string
		timeout time.Duration
		wantErr error
	}{
		{
			name:    "cancel before cmd wait",
			timeout: 0,
			wantErr: entrypoint.ErrContextCanceled,
		},
		{
			name:    "cancel on cmd wait",
			timeout: time.Second * time.Duration(rand.Intn(3)),
			wantErr: entrypoint.ErrContextCanceled,
		},
		{
			name:    "cancel after cmd wait",
			timeout: time.Second * 4,
			wantErr: nil,
		},
	}
	for _, tc := range testCases {
		rr := realRunner{}
		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			time.Sleep(tc.timeout)
			cancel()
		}()
		err := rr.Run(ctx, "sleep", "3")
		if tc.wantErr != nil {
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("unexpected error received: %v", err)
			}
		} else {
			if err != nil {
				t.Fatalf("unexpected error received: %v", err)
			}
		}
	}
}

func TestShouldWarnSecretMaskingDelay(t *testing.T) {
	testCases := []struct {
		name         string
		maxSecretLen int
		want         bool
	}{
		{name: "no secrets", maxSecretLen: 0, want: false},
		{name: "below threshold", maxSecretLen: secretMaskingDelayWarningThreshold, want: false},
		{name: "at threshold", maxSecretLen: secretMaskingDelayWarningThreshold + 1, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldWarnSecretMaskingDelay(tc.maxSecretLen)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasMaskableSecrets(t *testing.T) {
	testCases := []struct {
		name    string
		secrets []string
		want    bool
	}{
		{name: "no secrets", secrets: nil, want: false},
		{name: "all too short", secrets: []string{"", "ab"}, want: false},
		{name: "at least one maskable", secrets: []string{"ab", "abc"}, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := hasMaskableSecrets(tc.secrets)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRealRunnerSecretMaskingDelayWarning(t *testing.T) {
	tmpDir := t.TempDir()
	secret := strings.Repeat("x", secretMaskingDelayWarningThreshold+5)
	maskFile := filepath.Join(tmpDir, "secret-mask")
	content := base64.StdEncoding.EncodeToString([]byte(secret)) + "\n"
	if err := os.WriteFile(maskFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed writing secret mask file: %v", err)
	}

	rr := realRunner{
		secretMaskFile: maskFile,
	}

	oldStderr := os.Stderr
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed creating pipe: %v", err)
	}
	os.Stderr = stderrWriter
	defer func() {
		_ = stderrWriter.Close()
		os.Stderr = oldStderr
		_ = stderrReader.Close()
	}()

	if err := rr.Run(t.Context(), "sh", "-c", "echo step-output >&2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("failed closing writer: %v", err)
	}
	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stderrReader); err != nil {
		t.Fatalf("failed reading stderr: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Warning: secret masking enabled; largest secret is") {
		t.Fatalf("expected warning in stderr, got %q", got)
	}
	if strings.Count(got, "Warning: secret masking enabled;") != 1 {
		t.Fatalf("expected warning exactly once, got %q", got)
	}
	if !strings.Contains(got, "step-output") {
		t.Fatalf("expected step stderr output, got %q", got)
	}
}
