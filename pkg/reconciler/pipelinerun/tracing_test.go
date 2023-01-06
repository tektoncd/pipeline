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

package pipelinerun

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
		pipelineRun       *v1beta1.PipelineRun
		tracerProvider    trace.TracerProvider
		expectAnnotations bool
		expectSpanContext bool
		parentTraceID     string
	}{{
		name: "with-tracerprovider-no-parent-trace",
		pipelineRun: &v1beta1.PipelineRun{
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
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testns",
				Annotations: map[string]string{
					"tekton.dev/pipelinerunSpanContext": "{\"traceparent\":\"00-0f57e147e992b304d977436289d10628-73d5909e31793992-01\"}",
				},
			},
		},
		tracerProvider:    tracesdk.NewTracerProvider(),
		expectAnnotations: true,
		expectSpanContext: true,
		parentTraceID:     "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01",
	}, {
		name: "without-tracerprovider",
		pipelineRun: &v1beta1.PipelineRun{
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
		pipelineRun: &v1beta1.PipelineRun{
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
			pr := tc.pipelineRun
			ctx := initTracing(context.Background(), tc.tracerProvider, pr)

			if ctx == nil {
				t.Fatalf("returned nil context from initTracing")
			}

			if tc.expectAnnotations && pr.Annotations == nil {
				t.Fatalf("annotations are empty after initializing tracing")
			}

			if tc.expectSpanContext {
				if _, e := pr.Annotations["tekton.dev/pipelinerunSpanContext"]; !e {
					t.Fatalf("spanContext not added to annotations")
				}

				sc := pr.Annotations["tekton.dev/pipelinerunSpanContext"]
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
					t.Errorf("invalid trace Id propagated")
				}
			}
		})

	}

}
