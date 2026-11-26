/*
Copyright 2026 The Tekton Authors
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

package bundle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"syscall"
	"testing"
	"time"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// Like the net package's timeout errors, this one also matches context.DeadlineExceeded,
// which is why go-containerregistry's default predicate doesn't retry it.
func (timeoutError) Is(target error) bool { return target == context.DeadlineExceeded }

type temporaryError struct{ temporary bool }

func (e temporaryError) Error() string   { return "temporary" }
func (e temporaryError) Temporary() bool { return e.temporary }

func urlError(err error) error {
	return &url.Error{Op: "Get", URL: "https://registry.example.com/v2/", Err: err}
}

func TestIsRetryableError(t *testing.T) {
	dialTimeout := urlError(&net.OpError{Op: "dial", Net: "tcp", Err: timeoutError{}})
	if !errors.Is(dialTimeout, context.DeadlineExceeded) {
		t.Fatal("expected the dial timeout to match context.DeadlineExceeded")
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "dial timeout", err: dialTimeout, want: true},
		{name: "tls handshake timeout", err: urlError(fmt.Errorf("net/http: TLS handshake timeout: %w", timeoutError{})), want: true},
		{name: "connection reset", err: urlError(&net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}), want: true},
		{name: "broken pipe", err: urlError(&net.OpError{Op: "write", Net: "tcp", Err: syscall.EPIPE}), want: true},
		{name: "eof", err: urlError(io.EOF), want: true},
		{name: "unexpected eof", err: urlError(io.ErrUnexpectedEOF), want: true},
		{name: "closed connection", err: urlError(net.ErrClosed), want: true},
		{name: "temporary error", err: urlError(temporaryError{temporary: true}), want: true},
		{name: "permanent error", err: urlError(temporaryError{temporary: false}), want: false},
		{name: "plain error", err: errors.New("manifest unknown"), want: false},
		{name: "deadline exceeded", err: fmt.Errorf("fetch: %w", context.DeadlineExceeded), want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableError(context.Background(), tc.err); got != tc.want {
				t.Errorf("isRetryableError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsRetryableErrorStopsWhenContextIsDone(t *testing.T) {
	dialTimeout := urlError(&net.OpError{Op: "dial", Net: "tcp", Err: timeoutError{}})

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if isRetryableError(canceled, dialTimeout) {
		t.Error("expected no retry after the context was canceled")
	}

	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	if isRetryableError(expired, urlError(io.EOF)) {
		t.Error("expected no retry after the context deadline passed")
	}
}
