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

package pipelinerun

import (
	"context"
	"encoding/json"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestCreateChildPipelineRunPropagatesSpanContext(t *testing.T) {
	ctx, reconciler := tracingPropagationTestContext(t)
	pr := &v1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "parent", Namespace: "foo"}}
	rpt := &resources.ResolvedPipelineTask{PipelineTask: &v1.PipelineTask{
		Name:         "child-pipeline",
		PipelineSpec: &v1.PipelineSpec{},
	}}

	child, err := reconciler.createChildPipelineRun(ctx, "parent-child-pipeline", nil, rpt, pr, &resources.PipelineRunFacts{})
	if err != nil {
		t.Fatalf("createChildPipelineRun() = %v", err)
	}
	assertSpanContextAnnotation(t, child.Annotations, SpanContextAnnotation)
}

func TestCreateCustomRunPropagatesSpanContext(t *testing.T) {
	ctx, reconciler := tracingPropagationTestContext(t)
	pr := &v1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "parent", Namespace: "foo"}}
	rpt := &resources.ResolvedPipelineTask{PipelineTask: &v1.PipelineTask{
		Name: "custom-task",
		TaskRef: &v1.TaskRef{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
		},
	}}

	customRun, err := reconciler.createCustomRun(ctx, "parent-custom-task", nil, rpt, pr, &resources.PipelineRunFacts{})
	if err != nil {
		t.Fatalf("createCustomRun() = %v", err)
	}
	assertSpanContextAnnotation(t, customRun.Annotations, TaskRunSpanContextAnnotation)
}

func tracingPropagationTestContext(t *testing.T) (context.Context, *Reconciler) {
	t.Helper()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tracerProvider := tracesdk.NewTracerProvider()
	ctx, span := tracerProvider.Tracer(TracerName).Start(t.Context(), "parent")
	t.Cleanup(func() { span.End() })
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })
	ctx = logging.WithLogger(ctx, logtesting.TestLogger(t))

	return ctx, &Reconciler{
		PipelineClientSet: fakepipelineclientset.NewSimpleClientset(),
		tracerProvider:    tracerProvider,
	}
}

func assertSpanContextAnnotation(t *testing.T, annotations map[string]string, key string) {
	t.Helper()
	encoded := annotations[key]
	if encoded == "" {
		t.Fatalf("annotation %q not set: %v", key, annotations)
	}
	var carrier map[string]string
	if err := json.Unmarshal([]byte(encoded), &carrier); err != nil {
		t.Fatalf("annotation %q is not valid JSON: %v", key, err)
	}
	if traceparent := carrier["traceparent"]; len(traceparent) != 55 {
		t.Fatalf("traceparent length = %d, want 55 (%q)", len(traceparent), traceparent)
	}
}
