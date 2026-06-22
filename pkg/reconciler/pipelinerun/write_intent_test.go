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

// The metadata half is taken from the branch that actually updates the object,
// not from comparing its labels and annotations before and after. Those differ:
// updateLabelsAndAnnotations compares against the informer lister's copy, so
// metadata another actor changed during the reconcile makes it take the branch
// even though nothing local moved. Inferring from the local object reports
// no-op for that.
//
// Driving updateLabelsAndAnnotations rather than the whole reconcile keeps this
// to the branch under test: what it proves is that taking the branch is what
// sets the flag, and that a request really went out when it did.
func TestUpdateLabelsAndAnnotationsMarksTheMetadataUpdate(t *testing.T) {
	// What the API already holds: this reconcile never touches the extra key.
	stored := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pipelinerun-metadata", Namespace: "foo",
			Annotations: map[string]string{"example.dev/added-by": "another-controller"},
		},
	}

	ctx, _ := ttesting.SetupFakeContext(t)
	clients, informers := test.SeedTestData(t, ctx, test.Data{PipelineRuns: []*v1.PipelineRun{stored}})

	// The object this pass is reconciling, from before the other actor wrote.
	reconciling := stored.DeepCopy()
	delete(reconciling.Annotations, "example.dev/added-by")

	c := &Reconciler{
		PipelineClientSet: clients.Pipeline,
		pipelineRunLister: informers.PipelineRun.Lister(),
		tracerProvider:    tracesdk.NewTracerProvider(),
	}

	ctx, attempted := tknreconciler.TrackMetadataUpdate(ctx)
	if _, err := c.updateLabelsAndAnnotations(ctx, reconciling); err != nil {
		t.Fatalf("updateLabelsAndAnnotations() error: %v", err)
	}

	if !attempted.Load() {
		t.Error("the metadata update was not marked, so a reconcile taking this branch would be classified as if it had not")
	}

	// Read the flag against what the client saw, so a marker left in the wrong
	// place cannot report an update that never went out.
	updated := false
	for _, action := range clients.Pipeline.Actions() {
		if action.Matches("update", "pipelineruns") && action.GetSubresource() == "" {
			updated = true
		}
	}
	if !updated {
		t.Error("no PipelineRun update reached the client, so the flag above is not describing a request")
	}
}

// A request that fails is still a request the reconcile made, and the attribute
// is an upper bound on the update paths a pass selected rather than a count of
// what committed. The flag is set before the call for that reason, so a
// conflict or any other rejection leaves it set.
func TestUpdateLabelsAndAnnotationsMarksAnUpdateThatFails(t *testing.T) {
	stored := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pipelinerun-metadata-fails", Namespace: "foo",
			Annotations: map[string]string{"example.dev/added-by": "another-controller"},
		},
	}

	ctx, _ := ttesting.SetupFakeContext(t)
	clients, informers := test.SeedTestData(t, ctx, test.Data{PipelineRuns: []*v1.PipelineRun{stored}})

	rejected := errors.New("the apiserver rejected it")
	clients.Pipeline.PrependReactor("update", "pipelineruns", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, rejected
	})

	reconciling := stored.DeepCopy()
	delete(reconciling.Annotations, "example.dev/added-by")

	c := &Reconciler{
		PipelineClientSet: clients.Pipeline,
		pipelineRunLister: informers.PipelineRun.Lister(),
		tracerProvider:    tracesdk.NewTracerProvider(),
	}

	ctx, attempted := tknreconciler.TrackMetadataUpdate(ctx)
	if _, err := c.updateLabelsAndAnnotations(ctx, reconciling); !errors.Is(err, rejected) {
		t.Fatalf("updateLabelsAndAnnotations() error = %v, want %v", err, rejected)
	}
	if !attempted.Load() {
		t.Error("the failed update was not marked, so a pass that issued a request would be reported as if it had not")
	}
}
