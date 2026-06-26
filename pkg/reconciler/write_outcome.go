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
	"maps"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/api/equality"
)

// WriteOutcomeAttributeKey is the span attribute that records whether a
// reconcile pass changed anything that gets written to etcd. It lets operators
// see how many reconciliations cost a write versus being short-circuited.
const WriteOutcomeAttributeKey = "reconcile.write_outcome"

const (
	writeOutcomeWrite      = "write"
	writeOutcomeStatusOnly = "status-only"
	writeOutcomeNoOp       = "no-op"
)

// ClassifyWriteOutcome maps what changed during a reconcile to a write outcome.
// A labels/annotations change means the object itself is updated, so it counts
// as a full write and takes precedence over a status-only change.
func ClassifyWriteOutcome(metadataChanged, statusChanged bool) string {
	switch {
	case metadataChanged:
		return writeOutcomeWrite
	case statusChanged:
		return writeOutcomeStatusOnly
	default:
		return writeOutcomeNoOp
	}
}

// RecordWriteOutcome tags span with what a reconcile pass changed on the object,
// comparing the status and labels/annotations captured on entry against their
// current values. Status uses the same equality the generated reconciler uses.
func RecordWriteOutcome(span trace.Span, oldStatus, newStatus any, oldLabels, newLabels, oldAnnotations, newAnnotations map[string]string) {
	statusChanged := !equality.Semantic.DeepEqual(oldStatus, newStatus)
	metadataChanged := !maps.Equal(oldLabels, newLabels) || !maps.Equal(oldAnnotations, newAnnotations)
	span.SetAttributes(attribute.String(WriteOutcomeAttributeKey, ClassifyWriteOutcome(metadataChanged, statusChanged)))
}
