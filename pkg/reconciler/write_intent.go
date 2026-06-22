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

// writeIntentAttributeKey is the span attribute recording what a reconcile pass
// intends to write to etcd. The generated reconciler issues the status update
// after ReconcileKind returns, so a rejected update is not reflected here.
const writeIntentAttributeKey = "reconcile.write_intent"

const (
	writeIntentWrite      = "write"
	writeIntentStatusOnly = "status-only"
	writeIntentNoOp       = "no-op"
)

// classifyWriteIntent maps what changed during a reconcile to a write intent.
// A labels or annotations change is treated as a full object write and takes
// precedence over a status-only change.
func classifyWriteIntent(metadataChanged, statusChanged bool) string {
	switch {
	case metadataChanged:
		return writeIntentWrite
	case statusChanged:
		return writeIntentStatusOnly
	default:
		return writeIntentNoOp
	}
}

// RecordWriteIntent tags span with what a reconcile pass changed on the object,
// comparing the status and labels/annotations captured on entry against their
// current values. It no-ops when the span is not recording, so a caller pays
// nothing for the comparison when tracing is disabled or sampled out. Status
// uses the same equality the generated reconciler uses to decide whether to
// write the status subresource.
func RecordWriteIntent(span trace.Span, oldStatus, newStatus any, oldLabels, newLabels, oldAnnotations, newAnnotations map[string]string) {
	if !span.IsRecording() {
		return
	}
	statusChanged := !equality.Semantic.DeepEqual(oldStatus, newStatus)
	metadataChanged := !maps.Equal(oldLabels, newLabels) || !maps.Equal(oldAnnotations, newAnnotations)
	span.SetAttributes(attribute.String(writeIntentAttributeKey, classifyWriteIntent(metadataChanged, statusChanged)))
}
