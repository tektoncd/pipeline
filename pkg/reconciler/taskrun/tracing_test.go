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

package taskrun

import (
	"context"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInitTracing(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	testcases := []struct {
		name                    string
		taskRun                 *v1.TaskRun
		tracerProvider          trace.TracerProvider
		expectSpanContextStatus bool
		expectValidSpanContext  bool
		parentTraceID           string
	}{{
		name: "with-tracerprovider-no-parent-trace",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
			},
		},
		tracerProvider:          tracesdk.NewTracerProvider(),
		expectSpanContextStatus: true,
		expectValidSpanContext:  true,
	}, {
		name: "with-tracerprovider-with-parent-trace",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
				Annotations: map[string]string{
					"tekton.dev/taskrunSpanContext": "{\"traceparent\":\"00-0f57e147e992b304d977436289d10628-73d5909e31793992-01\"}",
				},
			},
		},
		tracerProvider:          tracesdk.NewTracerProvider(),
		expectSpanContextStatus: true,
		expectValidSpanContext:  true,
		parentTraceID:           "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01",
	}, {
		name: "without-tracerprovider",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
			},
		},
		tracerProvider:          trace.NewNoopTracerProvider(),
		expectSpanContextStatus: false,
		expectValidSpanContext:  false,
	}, {
		name: "without-tracerprovider-existing-annotations",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
				Annotations: map[string]string{
					"test": "test",
				},
			},
		},
		tracerProvider:          trace.NewNoopTracerProvider(),
		expectSpanContextStatus: false,
		expectValidSpanContext:  false,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tr := tc.taskRun
			ctx := initTracing(context.Background(), tc.tracerProvider, tr)

			if ctx == nil {
				t.Fatalf("returned nil context from initTracing")
			}

			if tc.expectSpanContextStatus && tr.Status.SpanContext == nil {
				t.Fatalf("spanContext is empty after initializing tracing")
			}

			if !tc.expectSpanContextStatus && len(tr.Status.SpanContext) > 0 {
				t.Fatalf("spanContext is not empty")
			}

			if tc.expectValidSpanContext {
				if len(tr.Status.SpanContext) == 0 {
					t.Fatalf("spanContext not added to annotations")
				}

				parentID := tr.Status.SpanContext["traceparent"]
				if len(parentID) != 55 {
					t.Errorf("invalid trace Id")
				}

				if tc.parentTraceID != "" && parentID != tc.parentTraceID {
					t.Errorf("invalid trace Id propagated, %s", parentID)
				}
			}
		})
	}
}
