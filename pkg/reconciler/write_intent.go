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

// writeIntentAttributeKey is the span attribute recording which update paths a
// reconcile pass selected. It is an upper bound on the revisions that pass can
// account for, not a count of the ones etcd committed: a request can be
// rejected, and an update carrying nothing the stored object does not already
// have is a no-op below the API.
const writeIntentAttributeKey = "reconcile.write_intent"

const (
	writeIntentMetadata   = "metadata-only"
	writeIntentStatusOnly = "status-only"
	writeIntentBoth       = "metadata-and-status"
	writeIntentNoOp       = "no-op"
)

// classifyWriteIntent maps the update paths a reconcile selected to an intent.
//
// The metadata path and the status path are separate requests against the same
// key: the reconciler updates the object itself while ReconcileKind runs, and
// the generated reconciler updates the status subresource after it returns. A
// pass that takes both can therefore account for two revisions, and reporting
// it as one would undercount them.
func classifyWriteIntent(metadataUpdateAttempted, statusDirty bool) string {
	switch {
	case metadataUpdateAttempted && statusDirty:
		return writeIntentBoth
	case metadataUpdateAttempted:
		return writeIntentMetadata
	case statusDirty:
		return writeIntentStatusOnly
	default:
		return writeIntentNoOp
	}
}

// metadataUpdateKey carries the flag a reconcile sets when it takes the branch
// that updates an object's labels or annotations.
type metadataUpdateKey struct{}

// TrackMetadataUpdate returns a context carrying a flag for this reconcile to
// set when it takes the branch that updates the object's labels or annotations,
// and the flag itself.
//
// The intent is classified from that flag rather than from comparing the
// object's metadata before and after, because the two are not the same
// decision. The update compares against the informer lister's copy, so metadata
// another actor changed during the reconcile makes it take the branch when
// nothing local moved, and metadata that copy already agrees with makes it skip
// the branch when something local did.
func TrackMetadataUpdate(ctx context.Context) (context.Context, *atomic.Bool) {
	attempted := &atomic.Bool{}
	return context.WithValue(ctx, metadataUpdateKey{}, attempted), attempted
}

// MarkMetadataUpdate records that this reconcile is about to update the
// object's labels or annotations. It is set before the request, so it survives
// a conflict or any other failure. It does nothing when the context is not
// tracking, so callers outside a tracked reconcile need no special case.
func MarkMetadataUpdate(ctx context.Context) {
	if attempted, ok := ctx.Value(metadataUpdateKey{}).(*atomic.Bool); ok {
		attempted.Store(true)
	}
}

// RecordWriteIntent tags span with the update paths a reconcile pass selected:
// metadataUpdateAttempted is whether it took the branch that updates the
// object, and the status is compared with the same equality the generated
// reconciler uses to decide whether to write the status subresource. It no-ops
// when the span is not recording.
//
// The status parameters share one type so that passing one of each does not
// compile. Left untyped they would both satisfy any, and the comparison would
// report a difference on every single reconcile.
func RecordWriteIntent[S any](span trace.Span, oldStatus, newStatus *S, metadataUpdateAttempted bool) {
	if !span.IsRecording() {
		return
	}
	statusDirty := !equality.Semantic.DeepEqual(oldStatus, newStatus)
	span.SetAttributes(attribute.String(writeIntentAttributeKey, classifyWriteIntent(metadataUpdateAttempted, statusDirty)))
}
