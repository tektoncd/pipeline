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

package k8sevent

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestTraceAnnotations verifies that traceAnnotations extracts a W3C trace
// context from the context using the globally configured propagator, and
// returns nil when no valid span context is present.
func TestTraceAnnotations(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })

	t.Run("no span context", func(t *testing.T) {
		if got := traceAnnotations(context.Background()); got != nil {
			t.Errorf("expected nil annotations without a span context, got %v", got)
		}
	})

	t.Run("with span context", func(t *testing.T) {
		traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
		if err != nil {
			t.Fatalf("invalid trace id: %v", err)
		}
		spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
		if err != nil {
			t.Fatalf("invalid span id: %v", err)
		}
		sc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
			Remote:     true,
		})
		ctx := trace.ContextWithSpanContext(context.Background(), sc)

		got := traceAnnotations(ctx)
		if got == nil {
			t.Fatal("expected annotations for a valid span context, got nil")
		}
		want := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		if got["traceparent"] != want {
			t.Errorf("traceparent = %q, want %q", got["traceparent"], want)
		}
	})
}
