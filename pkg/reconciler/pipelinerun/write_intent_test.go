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
	"errors"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/pipelinerunmetrics"
	tknreconciler "github.com/tektoncd/pipeline/pkg/reconciler"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// A completed PipelineRun returns early, so the only possible status change is
// the span context initTracing persists.
func TestReconcileKindRecordsWriteIntent(t *testing.T) {
	tests := []struct {
		name        string
		spanContext map[string]string
		want        string
	}{
		{name: "span context is persisted", want: "status-only"},
		{name: "span context already exists", spanContext: map[string]string{"traceparent": "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01"}, want: "no-op"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pipelinerun-done", Namespace: "foo"},
				Status: v1.PipelineRunStatus{Status: duckv1.Status{Conditions: duckv1.Conditions{{
					Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue, Reason: v1.PipelineRunReasonSuccessful.String(),
				}}}},
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
				KubeClientSet: clients.Kube, PipelineClientSet: clients.Pipeline, Clock: testClock,
				pipelineRunLister: informers.PipelineRun.Lister(), metrics: metrics,
				tracerProvider: tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder)),
			}

			if err := c.ReconcileKind(ctx, pr); err != nil {
				t.Fatalf("ReconcileKind() error: %v", err)
			}
			if got := recordedWriteIntent(recorder, "PipelineRun:ReconcileKind"); got != tc.want {
				t.Errorf("reconcile.write_intent = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSyncMetadataMarksUpdateAttempt(t *testing.T) {
	tests := []struct {
		name   string
		change bool
		reject bool
	}{
		{name: "no update"},
		{name: "successful update", change: true},
		{name: "rejected update", change: true, reject: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stored := &v1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "test-pipelinerun-metadata", Namespace: "foo"}}
			ctx, _ := ttesting.SetupFakeContext(t)
			clients, informers := test.SeedTestData(t, ctx, test.Data{PipelineRuns: []*v1.PipelineRun{stored}})
			rejected := errors.New("the apiserver rejected it")
			if tc.reject {
				clients.Pipeline.PrependReactor("update", "pipelineruns", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, rejected
				})
			}
			reconciling := stored.DeepCopy()
			if tc.change {
				reconciling.Annotations = map[string]string{"example.dev/added-by": "reconcile"}
			}
			c := &Reconciler{
				PipelineClientSet: clients.Pipeline,
				pipelineRunLister: informers.PipelineRun.Lister(),
				tracerProvider:    tracesdk.NewTracerProvider(),
			}

			ctx, attempted := tknreconciler.TrackMetadataUpdate(ctx)
			err := c.syncMetadata(ctx, reconciling)
			if tc.reject && !errors.Is(err, rejected) {
				t.Fatalf("syncMetadata() error = %v, want %v", err, rejected)
			}
			if !tc.reject && err != nil {
				t.Fatalf("syncMetadata() error: %v", err)
			}
			if attempted.Load() != tc.change {
				t.Errorf("metadata update attempted = %t, want %t", attempted.Load(), tc.change)
			}
			updated := false
			for _, action := range clients.Pipeline.Actions() {
				updated = updated || action.Matches("update", "pipelineruns")
			}
			if updated != tc.change {
				t.Errorf("PipelineRun update reached client = %t, want %t", updated, tc.change)
			}
		})
	}
}

func recordedWriteIntent(recorder *tracetest.SpanRecorder, spanName string) string {
	for _, span := range recorder.Ended() {
		if span.Name() != spanName {
			continue
		}
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "reconcile.write_intent" {
				return attr.Value.AsString()
			}
		}
	}
	return ""
}
