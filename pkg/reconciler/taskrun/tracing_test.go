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
	"maps"
	"testing"

	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInitTracing(t *testing.T) {
	oldPropagator := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(oldPropagator) })

	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := tracesdk.NewTracerProvider(tracesdk.WithSyncer(exporter))
	defer func() { _ = tracerProvider.Shutdown(t.Context()) }()

	testcases := []struct {
		name                    string
		taskRun                 *v1.TaskRun
		tracerProvider          trace.TracerProvider
		exporter                *tracetest.InMemoryExporter
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
		tracerProvider:          tracerProvider,
		exporter:                exporter,
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
			ctx, span := initTracing(t.Context(), tc.tracerProvider, tr)

			if tc.exporter != nil {
				if got := tc.exporter.GetSpans(); len(got) != 0 {
					t.Fatalf("root span exported before caller ended it: got %d spans", len(got))
				}
			}

			span.End()
			if tc.exporter != nil {
				spans := tc.exporter.GetSpans()
				if len(spans) != 1 {
					t.Fatalf("exported spans = %d, want 1", len(spans))
				}
				if spans[0].Name != "TaskRun:Reconciler" {
					t.Fatalf("exported span name = %q, want TaskRun:Reconciler", spans[0].Name)
				}
			}

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
	exporter := tracetest.NewInMemoryExporter()
	tp := tracesdk.NewTracerProvider(tracesdk.WithSyncer(exporter))
	t.Cleanup(func() {
		_ = tp.Shutdown(t.Context())
	})

	taskSpec := &v1.TaskSpec{
		Steps: []v1.Step{{Name: "s", Image: "foo"}},
	}

	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: "ns",
		},
		Spec: v1.TaskRunSpec{
			TaskSpec: taskSpec,
		},
	}

	rtr := &resources.ResolvedTask{
		TaskName: "my-task",
		TaskSpec: taskSpec,
	}

	_, err := applyParamsContextsResultsAndWorkspaces(
		t.Context(),
		tp.Tracer(TracerName),
		tr,
		rtr,
		map[string]corev1.Volume{},
	)
	if err != nil {
		t.Fatalf("applyParamsContextsResultsAndWorkspaces() = %v", err)
	}

	seen := map[string]struct{}{}
	for _, s := range exporter.GetSpans() {
		seen[s.Name] = struct{}{}
	}

	expectedSpanNames := []string{
		"applyParamsContextsResultsAndWorkspaces",
		"ApplyParameters",
		"ApplyWorkspaces",
	}

	for _, spanName := range expectedSpanNames {
		if _, ok := seen[spanName]; !ok {
			t.Fatalf("expected span %q to be exported; got spans: %v", spanName, maps.Keys(seen))
		}
	}
}
