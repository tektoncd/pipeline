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

package reconciler

import (
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClassifyWriteOutcome(t *testing.T) {
	tests := []struct {
		name            string
		metadataChanged bool
		statusChanged   bool
		want            string
	}{{
		name: "nothing changed is a no-op",
		want: "no-op",
	}, {
		name:          "only the status changed",
		statusChanged: true,
		want:          "status-only",
	}, {
		name:            "a labels/annotations change is a full object write",
		metadataChanged: true,
		want:            "write",
	}, {
		name:            "a metadata change outranks a status change",
		metadataChanged: true,
		statusChanged:   true,
		want:            "write",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyWriteOutcome(tc.metadataChanged, tc.statusChanged)
			if got != tc.want {
				t.Errorf("ClassifyWriteOutcome(%t, %t) = %q, want %q", tc.metadataChanged, tc.statusChanged, got, tc.want)
			}
		})
	}
}

// TestRecordWriteOutcome runs the same comparison and span tagging the
// reconcilers do, on real PipelineRunStatus values, and reads the resulting
// attribute back off a recorded span.
func TestRecordWriteOutcome(t *testing.T) {
	started := v1.PipelineRunStatus{}
	started.StartTime = &metav1.Time{}
	// initTracing persists span context into the status, which is a real write.
	spanCtxOnly := v1.PipelineRunStatus{}
	spanCtxOnly.SpanContext = map[string]string{"traceparent": "abc"}
	labels := map[string]string{"tekton.dev/pipeline": "build"}

	tests := []struct {
		name                 string
		oldStatus, newStatus v1.PipelineRunStatus
		oldLabels, newLabels map[string]string
		want                 string
	}{{
		name: "untouched run is a no-op",
		want: "no-op", oldLabels: labels, newLabels: labels,
	}, {
		name:      "status moved but labels did not",
		newStatus: started, want: "status-only", oldLabels: labels, newLabels: labels,
	}, {
		name:      "span context persisted into status is still a write",
		newStatus: spanCtxOnly, want: "status-only", oldLabels: labels, newLabels: labels,
	}, {
		name: "a label changed", want: "write",
		oldLabels: labels, newLabels: map[string]string{"tekton.dev/pipeline": "deploy"},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := tracetest.NewSpanRecorder()
			provider := tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder))
			_, span := provider.Tracer("test").Start(t.Context(), "reconcile")

			RecordWriteOutcome(span, tc.oldStatus, tc.newStatus, tc.oldLabels, tc.newLabels, nil, nil)
			span.End()

			ended := recorder.Ended()
			if len(ended) != 1 {
				t.Fatalf("recorded %d spans, want 1", len(ended))
			}
			value, found := "", false
			for _, attr := range ended[0].Attributes() {
				if string(attr.Key) == "reconcile.write_outcome" {
					value, found = attr.Value.AsString(), true
				}
			}
			if !found {
				t.Fatalf("span is missing the %q attribute, got %v", WriteOutcomeAttributeKey, ended[0].Attributes())
			}
			if value != tc.want {
				t.Errorf("write_outcome = %q, want %q", value, tc.want)
			}
		})
	}
}
