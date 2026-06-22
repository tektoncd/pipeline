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

// Package cachetransform provides cache transform functions for reducing memory
// usage in the Tekton Pipeline controller informer caches.
//
// Transform functions are applied to objects before they are stored in the
// informer cache, allowing us to strip large, unnecessary fields while
// preserving the data needed for reconciliation. They only affect the
// controller's in-memory informer cache; the full objects always remain in
// etcd.
//
// The transforms are wired into the TaskRun and PipelineRun controllers via
// Setup (see setup.go), which calls SharedIndexInformer.SetTransform.
//
// Scope: these transforms currently strip only universally-safe metadata
// fields (managedFields and the kubectl last-applied-configuration annotation).
// Stripping status fields from completed objects was intentionally deferred:
// several of those fields (TaskRun status.taskSpec/steps/sidecars and
// PipelineRun status.pipelineSpec) are still read from cache after completion
// (by the parent PipelineRun, by retryTaskRun, by stopSidecars, and by upcoming
// Pipelines-in-Pipelines result propagation).
//
// DEVELOPER WARNING:
// If you add new reconciliation logic that reads a field from cached objects
// (via listers), you MUST verify that field is not stripped by these transforms.
// Fields stripped from cached objects will be nil/empty even though they exist
// in etcd. If you need a stripped field, either:
//  1. Use the Kubernetes API client directly to fetch the full object from etcd, or
//  2. Update the transform functions to preserve the field (consider memory impact).
//
// See docs/developers/controller-logic.md for more details.
package cachetransform

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// LastAppliedConfigAnnotation is the annotation used by kubectl to store
	// the last applied configuration. This can be very large and is not needed
	// in the cache.
	LastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
)

// ForTektonResource is a cache.TransformFunc that strips unnecessary metadata
// fields from cached PipelineRuns, TaskRuns and CustomRuns to reduce memory
// usage.
//
// Objects that are not PipelineRuns, TaskRuns or CustomRuns are passed through
// unchanged. Tombstone objects (DeletedFinalStateUnknown) are unwrapped,
// transformed, and rewrapped.
func ForTektonResource(obj interface{}) (interface{}, error) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		transformed, err := ForTektonResource(tombstone.Obj)
		if err != nil {
			return obj, nil //nolint:nilerr // Intentional: return original object on error for graceful degradation
		}
		return cache.DeletedFinalStateUnknown{Key: tombstone.Key, Obj: transformed}, nil
	}

	switch o := obj.(type) {
	case *v1.PipelineRun:
		stripMetadata(&o.ObjectMeta)
		return o, nil
	case *v1.TaskRun:
		stripMetadata(&o.ObjectMeta)
		return o, nil
	case *v1beta1.CustomRun:
		stripMetadata(&o.ObjectMeta)
		return o, nil
	default:
		return obj, nil
	}
}

// ForPod is a cache.TransformFunc that strips unnecessary metadata fields from
// cached Pods to reduce memory usage in the TaskRun controller.
//
// Only universally-safe metadata is stripped; the Pod spec and status are left
// intact. Tombstone objects (DeletedFinalStateUnknown) are unwrapped,
// transformed, and rewrapped.
func ForPod(obj interface{}) (interface{}, error) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		transformed, err := ForPod(tombstone.Obj)
		if err != nil {
			return obj, nil //nolint:nilerr // Intentional: return original object on error for graceful degradation
		}
		return cache.DeletedFinalStateUnknown{Key: tombstone.Key, Obj: transformed}, nil
	}

	if pod, ok := obj.(*corev1.Pod); ok {
		stripMetadata(&pod.ObjectMeta)
		return pod, nil
	}

	return obj, nil
}

// stripMetadata removes universally-safe, large metadata fields that are never
// read from the cache by any reconciler:
//   - managedFields (can be 25-30% of object size)
//   - the kubectl last-applied-configuration annotation
//
// delete on a nil map is a no-op, so no nil check is needed for Annotations.
func stripMetadata(meta *metav1.ObjectMeta) {
	meta.ManagedFields = nil
	delete(meta.Annotations, LastAppliedConfigAnnotation)
}
