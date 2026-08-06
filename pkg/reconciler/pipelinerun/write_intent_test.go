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
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/pipelinerunmetrics"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TestReconcileKindRecordsWriteIntent drives the real ReconcileKind against a
// recording tracer and reads the attribute back off the span it produced.
//
// A completed PipelineRun is used because that path returns early and skips the
// labels and annotations update, so the only thing left to change is the span
// context initTracing persists. That is a real write and must not be reported
// as a no-op, which is why the baseline is captured before initTracing.
func TestReconcileKindRecordsWriteIntent(t *testing.T) {
	tests := []struct {
		name        string
		spanContext map[string]string
		want        string
	}{{
		name: "only the span context initTracing persists changed",
		want: "status-only",
	}, {
		name:        "span context already stored, so nothing changed",
		spanContext: map[string]string{"traceparent": "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01"},
		want:        "no-op",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pipelinerun-done", Namespace: "foo"},
				Status: v1.PipelineRunStatus{
					Status: duckv1.Status{Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
						Reason: v1.PipelineRunReasonSuccessful.String(),
					}}},
				},
			}
			pr.Status.SpanContext = tc.spanContext

			ctx, _ := ttesting.SetupFakeContext(t)
			clients, informers := test.SeedTestData(t, ctx, test.Data{PipelineRuns: []*v1.PipelineRun{pr}})
			metrics, err := pipelinerunmetrics.NewRecorder(ctx)
			if err != nil {
				t.Fatalf("pipelinerunmetrics.NewRecorder() error: %v", err)
			}

			recorder := tracetest.NewSpanRecorder()
			c := &Reconciler{
				KubeClientSet:     clients.Kube,
				PipelineClientSet: clients.Pipeline,
				Clock:             testClock,
				pipelineRunLister: informers.PipelineRun.Lister(),
				metrics:           metrics,
				tracerProvider:    tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder)),
			}

			if err := c.ReconcileKind(ctx, pr); err != nil {
				t.Fatalf("ReconcileKind() error: %v", err)
			}

			got, found := "", false
			for _, s := range recorder.Ended() {
				if s.Name() != "PipelineRun:ReconcileKind" {
					continue
				}
				for _, attr := range s.Attributes() {
					if string(attr.Key) == "reconcile.write_intent" {
						got, found = attr.Value.AsString(), true
					}
				}
			}
			if !found {
				t.Fatal("no reconcile.write_intent attribute on the PipelineRun:ReconcileKind span")
			}
			if got != tc.want {
				t.Errorf("reconcile.write_intent = %q, want %q", got, tc.want)
			}
		})
	}
}
