/*
Copyright 2019 The Tekton Authors

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
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

const testWaitPollingInterval = 50 * time.Millisecond

func TestRealWaiterWaitMissingFile(t *testing.T) {
	// Create a temp file and then immediately delete it to get
	// a legitimate tmp path and ensure the file doesnt exist
	// prior to testing Wait().
	tmp, err := os.CreateTemp("", "real_waiter_test_file")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmp.Name(), false, false)
		if err != nil {
			t.Errorf("error waiting on tmp file %q", tmp.Name())
		}
		close(doneCh)
	}()

	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-delay.C:
		// Success
	case <-doneCh:
		t.Errorf("did not expect Wait() to have detected a file at path %q", tmp.Name())
		if !delay.Stop() {
			<-delay.C
		}
	}
}

func TestRealWaiterWaitWithFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmp.Name(), false, false)
		if err != nil {
			t.Errorf("error waiting on tmp file %q", tmp.Name())
		}
		close(doneCh)
	}()
	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-doneCh:
		// Success
	case <-delay.C:
		t.Errorf("expected Wait() to have detected the file's existence by now")
	}
}

func TestRealWaiterWaitMissingContent(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmp.Name(), true, false)
		if err != nil {
			t.Errorf("error waiting on tmp file %q", tmp.Name())
		}
		close(doneCh)
	}()
	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-delay.C:
		// Success
	case <-doneCh:
		t.Errorf("no data was written to tmp file, did not expect Wait() to have detected a non-zero file size and returned")
		if !delay.Stop() {
			<-delay.C
		}
	}
}

func TestRealWaiterWaitWithContent(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmp.Name(), true, false)
		if err != nil {
			t.Errorf("error waiting on tmp file %q", tmp.Name())
		}
		close(doneCh)
	}()
	if err := os.WriteFile(tmp.Name(), []byte("😺"), 0700); err != nil {
		t.Errorf("error writing content to temp file: %v", err)
	}
	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-doneCh:
		// Success
	case <-delay.C:
		t.Errorf("expected Wait() to have detected a non-zero file size by now")
	}
}

func TestRealWaiterWaitWithErrorWaitfile(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file*.err")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	tmpFileName := strings.Replace(tmp.Name(), ".err", "", 1)
	defer os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		// error of type skipError is returned after encountering a error waitfile
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmpFileName, false, false)
		if err == nil {
			t.Errorf("expected skipError upon encounter error waitfile")
		}
		var skipErr entrypoint.SkipError
		if errors.As(err, &skipErr) {
			close(doneCh)
		} else {
			t.Errorf("unexpected error type %T", err)
		}
	}()
	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-doneCh:
		// Success
	case <-delay.C:
		t.Errorf("expected Wait() to have detected a non-zero file size by now")
	}
}

func TestRealWaiterWaitWithBreakpointOnFailure(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file*.err")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	tmpFileName := strings.Replace(tmp.Name(), ".err", "", 1)
	defer os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		// When breakpoint on failure is enabled skipError shouldn't be returned for a error waitfile
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmpFileName, false, true)
		if err != nil {
			t.Errorf("error waiting on tmp file %q", tmp.Name())
		}
		close(doneCh)
	}()
	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-doneCh:
		// Success
	case <-delay.C:
		t.Errorf("expected Wait() to have detected a non-zero file size by now")
	}
}

func TestRealWaiterWaitWithContextCanceled(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	ctx, cancel := context.WithCancel(context.Background())
	rw := realWaiter{}
	errCh := make(chan error)
	go func() {
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(ctx, tmp.Name(), true, false)
		if err == nil {
			t.Errorf("expected context canceled error")
		}
		errCh <- err
	}()
	cancel()
	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case err := <-errCh:
		if !errors.Is(err, entrypoint.ErrContextCanceled) {
			t.Errorf("expected ErrContextCanceled, got %T", err)
		}
	case <-delay.C:
		t.Errorf("expected Wait() to have a ErrContextCanceled")
	}
}

func TestRealWaiterWaitWithTimeout(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	rw := realWaiter{}
	errCh := make(chan error)
	go func() {
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(ctx, tmp.Name(), true, false)
		if err == nil {
			t.Errorf("expected context deadline error")
		}
		errCh <- err
	}()
	delay := time.NewTimer(2 * time.Second)
	select {
	case err := <-errCh:
		if !errors.Is(err, entrypoint.ErrContextDeadlineExceeded) {
			t.Errorf("expected ErrContextDeadlineExceeded, got %T", err)
		}
	case <-delay.C:
		t.Errorf("expected Wait() to have a ErrContextDeadlineExceeded")
	}
}

func TestRealWaiterWaitContextWithBreakpointOnFailure(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file*.err")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	tmpFileName := strings.Replace(tmp.Name(), ".err", "", 1)
	defer os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		// When breakpoint on failure is enabled skipError shouldn't be returned for a error waitfile
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmpFileName, false, true)
		if err != nil {
			t.Errorf("error waiting on tmp file %q", tmp.Name())
		}
		close(doneCh)
	}()
	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-doneCh:
		// Success
	case <-delay.C:
		t.Errorf("expected Wait() to have detected a non-zero file size by now")
	}
}

func TestRealWaiterWaitContextWithErrorWaitfile(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file*.err")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	tmpFileName := strings.Replace(tmp.Name(), ".err", "", 1)
	defer os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		// error of type skipError is returned after encountering a error waitfile
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmpFileName, false, false)
		if err == nil {
			t.Errorf("expected skipError upon encounter error waitfile")
		}
		var skipErr entrypoint.SkipError
		if errors.As(err, &skipErr) {
			close(doneCh)
		} else {
			t.Errorf("unexpected error type %T", err)
		}
	}()
	delay := time.NewTimer(10 * testWaitPollingInterval)
	select {
	case <-doneCh:
		// Success
	case <-delay.C:
		t.Errorf("expected Wait() to have detected a non-zero file size by now")
	}
}

func TestRealWaiterWaitContextWithContent(t *testing.T) {
	tmp, err := os.CreateTemp("", "real_waiter_test_file")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmp.Name(), true, false)
		if err != nil {
			t.Errorf("error waiting on tmp file %q", tmp.Name())
		}
		close(doneCh)
	}()
	if err := os.WriteFile(tmp.Name(), []byte("😺"), 0700); err != nil {
		t.Errorf("error writing content to temp file: %v", err)
	}
	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-doneCh:
		// Success
	case <-delay.C:
		t.Errorf("expected Wait() to have detected a non-zero file size by now")
	}
}

func TestRealWaiterWaitContextMissingFile(t *testing.T) {
	// Create a temp file and then immediately delete it to get
	// a legitimate tmp path and ensure the file doesnt exist
	// prior to testing Wait().
	tmp, err := os.CreateTemp("", "real_waiter_test_file")
	if err != nil {
		t.Errorf("error creating temp file: %v", err)
	}
	os.Remove(tmp.Name())
	rw := realWaiter{}
	doneCh := make(chan struct{})
	go func() {
		err := rw.setWaitPollingInterval(testWaitPollingInterval).Wait(context.Background(), tmp.Name(), false, false)
		if err != nil {
			t.Errorf("error waiting on tmp file %q", tmp.Name())
		}
		close(doneCh)
	}()

	delay := time.NewTimer(2 * testWaitPollingInterval)
	select {
	case <-delay.C:
		// Success
	case <-doneCh:
		t.Errorf("did not expect Wait() to have detected a file at path %q", tmp.Name())
		if !delay.Stop() {
			<-delay.C
		}
	}
}
