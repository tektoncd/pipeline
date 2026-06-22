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
	"maps"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tknreconciler "github.com/tektoncd/pipeline/pkg/reconciler"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestReconcileKindWriteIntentSpanContext exercises the write-intent bookkeeping
// ReconcileKind does around the real initTracing. initTracing persists span
// context into the status, and that write must be reported rather than hidden as
// a no-op, which is why the baseline restores the pre-initTracing span context.
// The capture sequence here mirrors ReconcileKind.
func TestReconcileKindWriteIntentSpanContext(t *testing.T) {
	const traceparent = "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01"

	tests := []struct {
		name        string
		annotations map[string]string
		spanContext map[string]string
		want        string
	}{{
		name:        "initTracing persists span context, a status-only write",
		annotations: map[string]string{SpanContextAnnotation: `{"traceparent":"` + traceparent + `"}`},
		want:        "status-only",
	}, {
		name:        "span context already stored, nothing to write",
		spanContext: map[string]string{"traceparent": traceparent},
		want:        "no-op",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := tracetest.NewSpanRecorder()
			tp := tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder))

			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pr", Namespace: "foo", Annotations: tc.annotations},
			}
			pr.Status.SpanContext = tc.spanContext

			oldSpanContext := maps.Clone(pr.Status.SpanContext)
			ctx := initTracing(t.Context(), tp, pr)
			_, span := tp.Tracer(TracerName).Start(ctx, "PipelineRun:ReconcileKind")
			if !span.IsRecording() {
				t.Fatal("expected the test span to be recording")
			}
			oldStatus := pr.Status.DeepCopy()
			oldStatus.SpanContext = oldSpanContext
			tknreconciler.RecordWriteIntent(span, *oldStatus, pr.Status, pr.Labels, pr.Labels, pr.Annotations, pr.Annotations)
			span.End()

			ended := recorder.Ended()
			if len(ended) != 1 {
				t.Fatalf("recorded %d spans, want 1", len(ended))
			}
			got := ""
			for _, attr := range ended[0].Attributes() {
				if string(attr.Key) == "reconcile.write_intent" {
					got = attr.Value.AsString()
				}
			}
			if got != tc.want {
				t.Errorf("reconcile.write_intent = %q, want %q", got, tc.want)
			}
		})
	}
}
