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

package customrun

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

func TestInitTracing(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	const parentTraceParent = "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01"
	parentSpanContextJSON, err := json.Marshal(map[string]string{
		"traceparent": parentTraceParent,
	})
	if err != nil {
		t.Fatalf("failed to marshal span context: %v", err)
	}

	testcases := []struct {
		name                   string
		customRun              *v1beta1.CustomRun
		tracerProvider         trace.TracerProvider
		expectValidSpanContext bool
		parentTraceParent      string
	}{{
		name: "with-tracerprovider-no-parent-trace",
		customRun: &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
			},
		},
		tracerProvider:         tracesdk.NewTracerProvider(),
		expectValidSpanContext: true,
	}, {
		name: "with-tracerprovider-with-parent-trace-from-annotation",
		customRun: &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
				Annotations: map[string]string{
					SpanContextAnnotation: string(parentSpanContextJSON),
				},
			},
		},
		tracerProvider:         tracesdk.NewTracerProvider(),
		expectValidSpanContext: true,
		parentTraceParent:      parentTraceParent,
	}, {
		name: "without-tracerprovider",
		customRun: &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
			},
		},
		tracerProvider:         trace.NewNoopTracerProvider(),
		expectValidSpanContext: false,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := logging.WithLogger(t.Context(), logging.FromContext(context.Background()))
			returnedCtx := initTracing(ctx, tc.tracerProvider, tc.customRun)

			if returnedCtx == nil {
				t.Fatalf("returned nil context from initTracing")
			}

			spanCtx := trace.SpanContextFromContext(returnedCtx)
			if tc.expectValidSpanContext {
				if !spanCtx.IsValid() {
					t.Fatalf("span context is invalid after initializing tracing")
				}

				if tc.parentTraceParent != "" {
					carrier := propagation.MapCarrier{}
					otel.GetTextMapPropagator().Inject(returnedCtx, carrier)
					if got := carrier["traceparent"]; got != tc.parentTraceParent {
						t.Fatalf("expected traceparent %q, got %q", tc.parentTraceParent, got)
					}
				}
			} else if spanCtx.IsValid() {
				t.Fatalf("expected invalid span context with noop tracer provider")
			}
		})
	}
}
