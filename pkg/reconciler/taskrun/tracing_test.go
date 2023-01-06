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
	"encoding/json"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInitTracing(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	testcases := []struct {
		name              string
		taskRun           *v1beta1.TaskRun
		tracerProvider    trace.TracerProvider
		expectAnnotations bool
		expectSpanContext bool
		parentTraceID     string
	}{{
		name: "with-tracerprovider-no-parent-trace",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
			},
		},
		tracerProvider:    tracesdk.NewTracerProvider(),
		expectAnnotations: true,
		expectSpanContext: true,
	}, {
		name: "with-tracerprovider-with-parent-trace",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
				Annotations: map[string]string{
					"tekton.dev/taskrunSpanContext": "{\"traceparent\":\"00-0f57e147e992b304d977436289d10628-73d5909e31793992-01\"}",
				},
			},
		},
		tracerProvider:    tracesdk.NewTracerProvider(),
		expectAnnotations: true,
		expectSpanContext: true,
		parentTraceID:     "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01",
	}, {
		name: "without-tracerprovider",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
			},
		},
		tracerProvider:    trace.NewNoopTracerProvider(),
		expectAnnotations: false,
		expectSpanContext: false,
	}, {
		name: "without-tracerprovider-existing-annotations",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
				Annotations: map[string]string{
					"test": "test",
				},
			},
		},
		tracerProvider:    trace.NewNoopTracerProvider(),
		expectAnnotations: true,
		expectSpanContext: false,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tr := tc.taskRun
			ctx := initTracing(context.Background(), tc.tracerProvider, tr)

			if ctx == nil {
				t.Fatalf("returned nil context from initTracing")
			}

			if tc.expectAnnotations && tr.Annotations == nil {
				t.Fatalf("annotations are empty after initializing tracing")
			}

			if tc.expectSpanContext {
				if _, e := tr.Annotations["tekton.dev/taskrunSpanContext"]; !e {
					t.Fatalf("spanContext not added to annotations")
				}

				sc := tr.Annotations["tekton.dev/taskrunSpanContext"]
				carrier := make(map[string]string)
				err := json.Unmarshal([]byte(sc), &carrier)
				if err != nil {
					t.Errorf("Unable to unmarshal spancontext, err: %s", err)
				}

				parentID := carrier["traceparent"]
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
