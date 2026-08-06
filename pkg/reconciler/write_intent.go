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
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/api/equality"
)

// writeIntentAttributeKey is the span attribute recording what a reconcile pass
// intends to write to etcd. The generated reconciler issues the status update
// after ReconcileKind returns, so a rejected update is not reflected here.
const writeIntentAttributeKey = "reconcile.write_intent"

const (
	writeIntentMetadata   = "metadata-only"
	writeIntentStatusOnly = "status-only"
	writeIntentBoth       = "metadata-and-status"
	writeIntentNoOp       = "no-op"
)

// classifyWriteIntent maps what changed during a reconcile to a write intent.
//
// A labels or annotations change and a status change are separate writes to the
// same key: the reconciler updates the object itself while ReconcileKind runs,
// and the generated reconciler updates the status subresource after it returns.
// A pass that does both therefore intends two writes, and reporting it as one
// would undercount the revisions it accounts for.
func classifyWriteIntent(metadataChanged, statusChanged bool) string {
	switch {
	case metadataChanged && statusChanged:
		return writeIntentBoth
	case metadataChanged:
		return writeIntentMetadata
	case statusChanged:
		return writeIntentStatusOnly
	default:
		return writeIntentNoOp
	}
}

// metadataWriteKey carries the flag a reconcile sets when it updates an
// object's labels or annotations.
type metadataWriteKey struct{}

// TrackMetadataWrite returns a context carrying a flag for this reconcile to
// set when it updates the object's labels or annotations, and the flag itself.
//
// The write intent is classified from that flag rather than from comparing the
// object's metadata before and after. The two are not the same decision: the
// update compares the object against a freshly read one, so metadata another
// actor changed during the reconcile makes it write when nothing local moved,
// and metadata it already agrees with makes it skip when something local did.
func TrackMetadataWrite(ctx context.Context) (context.Context, *atomic.Bool) {
	wrote := &atomic.Bool{}
	return context.WithValue(ctx, metadataWriteKey{}, wrote), wrote
}

// MetadataWritten records that this reconcile is updating the object's labels
// or annotations. It does nothing when the context is not tracking, so callers
// outside a tracked reconcile need no special case.
func MetadataWritten(ctx context.Context) {
	if wrote, ok := ctx.Value(metadataWriteKey{}).(*atomic.Bool); ok {
		wrote.Store(true)
	}
}

// RecordWriteIntent tags span with what a reconcile pass intends to write:
// metadataWritten is whether it took the branch that updates the object, and
// the status is compared with the same equality the generated reconciler uses
// to decide whether to write the status subresource. It no-ops when the span is
// not recording.
//
// The status parameters share one type so that passing a pointer for one and a
// value for the other does not compile. Left untyped they would both satisfy
// any, and the comparison would report a difference on every single reconcile.
func RecordWriteIntent[S any](span trace.Span, oldStatus, newStatus S, metadataWritten bool) {
	if !span.IsRecording() {
		return
	}
	statusChanged := !equality.Semantic.DeepEqual(oldStatus, newStatus)
	span.SetAttributes(attribute.String(writeIntentAttributeKey, classifyWriteIntent(metadataWritten, statusChanged)))
}
