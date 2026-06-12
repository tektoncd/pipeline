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
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
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
			ctx := initTracing(t.Context(), tc.tracerProvider, tr)

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

func TestReconcilerApplyPathsEmitSpans(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	exporter := tracetest.NewInMemoryExporter()
	tp := tracesdk.NewTracerProvider(tracesdk.WithSyncer(exporter))

	taskSpec := &v1.TaskSpec{
		Steps: []v1.Step{{Name: "s", Image: "foo"}},
	}
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "tr", Namespace: "ns"},
		Spec: v1.TaskRunSpec{
			TaskSpec: taskSpec,
		},
	}

	rtr := &resources.ResolvedTask{
		TaskName: "my-task",
		TaskSpec: taskSpec,
	}

	c := &Reconciler{
		tracerProvider: tp,
	}

	ctx := t.Context()
	_, err := applyParamsContextsResultsAndWorkspaces(ctx, c, tr, rtr, map[string]corev1.Volume{})
	if err != nil {
		t.Fatalf("applyParamsContextsResultsAndWorkspaces: %v", err)
	}

	if err := tp.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}

	spans := exporter.GetSpans()
	byName := make(map[string]*tracetest.SpanStub, len(spans))
	for i := range spans {
		byName[spans[i].Name] = &spans[i]
	}

	parent, ok := byName["applyParamsContextsResultsAndWorkspaces"]
	if !ok {
		t.Fatal("applyParamsContextsResultsAndWorkspaces span not exported — reconciler is not using configured tracerProvider")
	}

	helpers := []string{
		"ApplyParameters", "ApplyContexts", "ApplyWorkspaces",
		"ApplyResults", "ApplyArtifacts",
		"ApplyStepExitCodePath",
	}
	for _, name := range helpers {
		child, ok := byName[name]
		if !ok {
			t.Errorf("%s: span not exported — noopTracer is still being passed at this call site", name)
			continue
		}
		if child.Parent.SpanID() != parent.SpanContext.SpanID() {
			t.Errorf("%s: wrong parent span — got %v, want %v", name, child.Parent.SpanID(), parent.SpanContext.SpanID())
		}
		if child.SpanContext.TraceID() != parent.SpanContext.TraceID() {
			t.Errorf("%s: wrong trace ID — got %v, want %v", name, child.SpanContext.TraceID(), parent.SpanContext.TraceID())
		}
	}
}
