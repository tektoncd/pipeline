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

package resolutionrequest

import (
	"context"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"
)

func TestReconcileKindCreatesSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	fakeClock := clock.NewFakePassiveClock(time.Now())
	r := &Reconciler{
		clock:          fakeClock,
		tracerProvider: tp,
	}

	rr := &v1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-rr",
			Namespace:         "default",
			CreationTimestamp: metav1.Now(),
		},
	}

	_ = r.ReconcileKind(context.Background(), rr)

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one span to be recorded, got none")
	}

	var found bool
	for _, s := range spans {
		if s.Name() == "ReconcileKind" {
			found = true
			attrs := spanAttrs(s)
			if got := attrs["resolutionrequest.name"]; got != "test-rr" {
				t.Errorf("resolutionrequest.name: got %q, want %q", got, "test-rr")
			}
			if got := attrs["resolutionrequest.namespace"]; got != "default" {
				t.Errorf("resolutionrequest.namespace: got %q, want %q", got, "default")
			}
		}
	}
	if !found {
		t.Errorf("ReconcileKind span not found; recorded spans: %v", endedSpanNames(spans))
	}
}

func TestReconcileKindSpanOutcomeAttributes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		data        string
		wantOutcome string
	}{{
		name:        "succeeded",
		data:        "cmVzb2x2ZWQ=",
		wantOutcome: "succeeded",
	}, {
		name:        "in-progress",
		data:        "",
		wantOutcome: "in-progress",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := tracetest.NewSpanRecorder()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

			r := &Reconciler{
				clock:          clock.NewFakePassiveClock(time.Now()),
				tracerProvider: tp,
			}

			rr := &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-rr",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
				},
			}
			rr.Status.Data = tc.data

			_ = r.ReconcileKind(context.Background(), rr)

			var outcome string
			for _, s := range recorder.Ended() {
				if s.Name() == "ReconcileKind" {
					outcome = spanAttrs(s)["resolutionrequest.outcome"]
				}
			}
			if outcome != tc.wantOutcome {
				t.Errorf("resolutionrequest.outcome: got %q, want %q", outcome, tc.wantOutcome)
			}
		})
	}
}

// spanAttrs converts span attributes to a string map for easy lookup.
func spanAttrs(s sdktrace.ReadOnlySpan) map[string]string {
	m := make(map[string]string, len(s.Attributes()))
	for _, a := range s.Attributes() {
		m[string(a.Key)] = a.Value.AsString()
	}
	return m
}

// endedSpanNames returns just the names of a slice of spans for diagnostic output.
func endedSpanNames(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name()
	}
	return names
}
