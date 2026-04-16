/*
Copyright 2022 The Tekton Authors

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

package customrun_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications/customrun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInitTracing(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	testcases := []struct {
		name                   string
		customRun              *v1beta1.CustomRun
		tracerProvider         trace.TracerProvider
		expectValidSpanContext bool
		parentTraceID          string
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
		name: "with-tracerprovider-with-parent-trace",
		customRun: &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
				Annotations: map[string]string{
					"tekton.dev/customrunSpanContext": "{\"traceparent\":\"00-0f57e147e992b304d977436289d10628-73d5909e31793992-01\"}",
				},
			},
		},
		tracerProvider:         tracesdk.NewTracerProvider(),
		expectValidSpanContext: true,
		parentTraceID:          "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01",
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
	}, {
		name: "without-tracerprovider-existing-annotations",
		customRun: &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
				Annotations: map[string]string{
					"test": "test",
				},
			},
		},
		tracerProvider:         trace.NewNoopTracerProvider(),
		expectValidSpanContext: false,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cr := tc.customRun
			returnedCtx := customrun.InitTracing(ctx, tc.tracerProvider, cr)

			if returnedCtx == nil {
				t.Fatalf("returned nil context from InitTracing")
			}

			// Create a span from the returned context to verify tracing is working
			span := trace.SpanFromContext(returnedCtx)
			if tc.expectValidSpanContext && !span.SpanContext().IsValid() {
				t.Fatalf("span context is invalid after initializing tracing")
			}
		})
	}
}
