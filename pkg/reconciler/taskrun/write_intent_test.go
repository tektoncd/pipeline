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

package taskrun

import (
	"fmt"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	podconvert "github.com/tektoncd/pipeline/pkg/pod"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/changeset"
)

// TestReconcileKindRecordsWriteIntent drives the real ReconcileKind against a
// recording tracer and reads the attribute back off the span it produced.
//
// A pending TaskRun is used because that path returns before anything needs the
// metrics recorder or a pod, and because one already marked pending changes no
// condition of its own. What is left is what the reconcile itself writes: the
// release annotation, and the span context initTracing persists. The latter is
// a real write that must not be reported as a no-op, which is why the baseline
// is captured before initTracing.
func TestReconcileKindRecordsWriteIntent(t *testing.T) {
	release := map[string]string{podconvert.ReleaseAnnotation: changeset.Get()}

	tests := []struct {
		name        string
		annotations map[string]string
		spanContext map[string]string
		want        string
	}{{
		// The reconcile stamps the controller version on the object, which
		// updateLabelsAndAnnotations persists with a full object Update, and
		// initTracing persists span context into the status, which the
		// framework writes separately. Two writes to one key.
		name: "the reconcile writes both the release annotation and the status",
		want: "metadata-and-status",
	}, {
		name:        "only the span context initTracing persists changed",
		annotations: release,
		want:        "status-only",
	}, {
		name:        "span context already stored, so nothing changed",
		annotations: release,
		spanContext: map[string]string{"traceparent": "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01"},
		want:        "no-op",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "test-taskrun-pending", Namespace: "foo", Annotations: tc.annotations},
				Spec:       v1.TaskRunSpec{Status: v1.TaskRunSpecStatusPending},
			}
			tr.Status.MarkResourceOngoing(v1.TaskRunReasonPending, fmt.Sprintf("TaskRun %q is pending", tr.Name))
			tr.Status.SpanContext = tc.spanContext

			ctx, _ := ttesting.SetupFakeContext(t)
			clients, informers := test.SeedTestData(t, ctx, test.Data{TaskRuns: []*v1.TaskRun{tr}})

			recorder := tracetest.NewSpanRecorder()
			c := &Reconciler{
				KubeClientSet:     clients.Kube,
				PipelineClientSet: clients.Pipeline,
				Clock:             testClock,
				taskRunLister:     informers.TaskRun.Lister(),
				podLister:         informers.Pod.Lister(),
				tracerProvider:    tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder)),
			}

			if err := c.ReconcileKind(ctx, tr); err != nil {
				t.Fatalf("ReconcileKind() error: %v", err)
			}

			got, found := "", false
			for _, s := range recorder.Ended() {
				if s.Name() != "TaskRun:ReconcileKind" {
					continue
				}
				for _, attr := range s.Attributes() {
					if string(attr.Key) == "reconcile.write_intent" {
						got, found = attr.Value.AsString(), true
					}
				}
			}
			if !found {
				t.Fatal("no reconcile.write_intent attribute on the TaskRun:ReconcileKind span")
			}
			if got != tc.want {
				t.Errorf("reconcile.write_intent = %q, want %q", got, tc.want)
			}
		})
	}
}

// The metadata half is taken from the branch that actually updates the object,
// not from comparing its labels and annotations before and after. Those differ:
// updateLabelsAndAnnotations compares against the informer lister's copy, so metadata
// another actor changed during the reconcile makes it write even though nothing
// local moved. Inferring from the local object reports no-op for that.
func TestReconcileKindRecordsMetadataWriteAnotherActorCaused(t *testing.T) {
	release := changeset.Get()
	// What the API already holds: this reconcile never touches the extra key.
	stored := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-taskrun-pending", Namespace: "foo",
			Annotations: map[string]string{
				podconvert.ReleaseAnnotation: release,
				"example.dev/added-by":       "another-controller",
			},
		},
		Spec: v1.TaskRunSpec{Status: v1.TaskRunSpecStatusPending},
	}
	stored.Status.MarkResourceOngoing(v1.TaskRunReasonPending, fmt.Sprintf("TaskRun %q is pending", stored.Name))
	stored.Status.SpanContext = map[string]string{"traceparent": "00-0f57e147e992b304d977436289d10628-73d5909e31793992-01"}

	ctx, _ := ttesting.SetupFakeContext(t)
	clients, informers := test.SeedTestData(t, ctx, test.Data{TaskRuns: []*v1.TaskRun{stored}})

	// The object this pass is reconciling, from before the other actor wrote.
	// Nothing in the reconcile changes its metadata or its status.
	reconciling := stored.DeepCopy()
	delete(reconciling.Annotations, "example.dev/added-by")

	recorder := tracetest.NewSpanRecorder()
	c := &Reconciler{
		KubeClientSet:     clients.Kube,
		PipelineClientSet: clients.Pipeline,
		Clock:             testClock,
		taskRunLister:     informers.TaskRun.Lister(),
		podLister:         informers.Pod.Lister(),
		tracerProvider:    tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder)),
	}
	if err := c.ReconcileKind(ctx, reconciling); err != nil {
		t.Fatalf("ReconcileKind() error: %v", err)
	}

	got := ""
	for _, s := range recorder.Ended() {
		if s.Name() != "TaskRun:ReconcileKind" {
			continue
		}
		for _, attr := range s.Attributes() {
			if string(attr.Key) == "reconcile.write_intent" {
				got = attr.Value.AsString()
			}
		}
	}
	if got != "metadata-only" {
		t.Errorf("reconcile.write_intent = %q, want %q: the reconcile issued a metadata update even though its own copy did not change", got, "metadata-only")
	}
}
