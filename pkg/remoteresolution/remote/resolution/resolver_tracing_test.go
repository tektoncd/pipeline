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

package resolution

import (
	"errors"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	remoteresource "github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	test "github.com/tektoncd/pipeline/test/remoteresolution"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testOwner() *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"},
	}
}

// getSpan returns the span named "Get" from the recorded spans, or fails.
func getSpan(t *testing.T, spans []sdktrace.ReadOnlySpan) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, s := range spans {
		if s.Name() == "Get" {
			return s
		}
	}
	t.Fatalf("Get span not found in recorded spans")
	return nil
}

func spanHasException(s sdktrace.ReadOnlySpan) bool {
	for _, e := range s.Events() {
		if e.Name == "exception" {
			return true
		}
	}
	return false
}

func TestGetCreatesSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	ctx, parent := tp.Tracer("test").Start(t.Context(), "parent")

	requester := &test.Requester{
		ResolvedResource: &test.ResolvedResource{ResolvedData: pipelineBytes},
	}
	resolver := NewResolver(requester, testOwner(), "git", remoteresource.ResolverPayload{})

	if _, _, err := resolver.Get(ctx, "foo", "bar"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parent.End()

	span := getSpan(t, recorder.Ended())
	var name string
	for _, a := range span.Attributes() {
		if a.Key == "resolver.name" {
			name = a.Value.AsString()
		}
	}
	if name != "git" {
		t.Errorf("resolver.name: got %q, want %q", name, "git")
	}
	if span.Status().Code == codes.Error {
		t.Errorf("expected no error status on successful Get, got %q", span.Status().Description)
	}
	if spanHasException(span) {
		t.Errorf("expected no recorded error on successful Get")
	}
}

func TestGetSpanErrorRecording(t *testing.T) {
	genericErr := errors.New("uh oh something bad happened")
	for _, tc := range []struct {
		name          string
		submitErr     error
		wantSpanError bool
	}{{
		// ErrRequestInProgress is the normal async-wait signal, not a failure,
		// so it must not be recorded as a span error (see the errors.Is guard).
		name:          "in-progress is not a span error",
		submitErr:     common.ErrRequestInProgress,
		wantSpanError: false,
	}, {
		name:          "real error is recorded",
		submitErr:     genericErr,
		wantSpanError: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := tracetest.NewSpanRecorder()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
			ctx, parent := tp.Tracer("test").Start(t.Context(), "parent")

			requester := &test.Requester{SubmitErr: tc.submitErr}
			resolver := NewResolver(requester, testOwner(), "git", remoteresource.ResolverPayload{})

			_, _, _ = resolver.Get(ctx, "foo", "bar")
			parent.End()

			span := getSpan(t, recorder.Ended())
			gotSpanError := span.Status().Code == codes.Error
			if gotSpanError != tc.wantSpanError {
				t.Errorf("span error status: got %v (code=%q), want %v", gotSpanError, span.Status().Code, tc.wantSpanError)
			}
			if got := spanHasException(span); got != tc.wantSpanError {
				t.Errorf("span exception event: got %v, want %v", got, tc.wantSpanError)
			}
		})
	}
}
