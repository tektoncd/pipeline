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

// Package transform provides cache transform functions for reducing memory
// usage in the Tekton Pipeline controller informer caches.
//
// Transform functions are applied to objects before they are stored in the
// informer cache, allowing us to strip large, unnecessary fields from objects
// that have completed their lifecycle while preserving the data needed for
// reconciliation.
//
// This implements the pattern described in:
// - https://github.com/projectcontour/contour/pull/5099
// - https://github.com/kubevela/kubevela/pull/5683
//
// DEVELOPER WARNING:
// If you add new reconciliation logic that reads a field from cached objects
// (via listers), you MUST verify that field is not stripped by these transforms.
// Fields stripped from cached objects will be nil/empty even though they exist
// in etcd. If you need a stripped field, either:
//  1. Use the Kubernetes API client directly to fetch the full object from etcd
//  2. Update the transform functions to preserve the field (consider memory impact)
//
// See docs/developers/controller-logic.md for more details.
package transform

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// LastAppliedConfigAnnotation is the annotation used by kubectl to store
	// the last applied configuration. This can be very large and is not needed
	// in the cache.
	LastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
)

// TransformForCache is a cache.TransformFunc that strips unnecessary fields
// from cached objects to reduce memory usage.
//
// This function handles:
//   - Stripping managedFields from all PipelineRuns and TaskRuns (Phase 1)
//   - Stripping last-applied-configuration annotation from all PipelineRuns and TaskRuns
//   - Stripping heavy status fields (PipelineSpec/TaskSpec, Provenance, SpanContext) from
//     completed PipelineRuns and TaskRuns only (Phase 2)
//
// Objects that are not PipelineRuns or TaskRuns are passed through unchanged.
// Tombstone objects (DeletedFinalStateUnknown) are unwrapped, transformed,
// and rewrapped.
func TransformForCache(obj interface{}) (interface{}, error) {
	// Handle tombstones (DeletedFinalStateUnknown) which wrap deleted objects
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		transformedObj, err := TransformForCache(tombstone.Obj)
		if err != nil {
			return obj, nil //nolint:nilerr // Intentional: return original object on error for graceful degradation
		}
		return cache.DeletedFinalStateUnknown{
			Key: tombstone.Key,
			Obj: transformedObj,
		}, nil
	}

	// Transform PipelineRuns
	if pr, ok := obj.(*v1.PipelineRun); ok {
		return transformPipelineRun(pr), nil
	}

	// Transform TaskRuns
	if tr, ok := obj.(*v1.TaskRun); ok {
		return transformTaskRun(tr), nil
	}

	// Transform CustomRuns
	if cr, ok := obj.(*v1beta1.CustomRun); ok {
		return transformCustomRun(cr), nil
	}

	// Pass through other objects unchanged
	return obj, nil
}

// transformPipelineRun strips unnecessary fields from a PipelineRun.
func transformPipelineRun(pr *v1.PipelineRun) *v1.PipelineRun {
	// Phase 1: Strip metadata fields (all PipelineRuns)
	// These fields are large and not needed for reconciliation
	pr.ManagedFields = nil

	// Strip last-applied-configuration annotation
	if pr.Annotations != nil {
		delete(pr.Annotations, LastAppliedConfigAnnotation)
	}

	// Phase 2: Strip heavy status fields from completed PipelineRuns only
	// Running PipelineRuns need these fields for proper reconciliation
	if pr.IsDone() {
		pr.Status.PipelineSpec = nil
		pr.Status.Provenance = nil
		pr.Status.SpanContext = nil
	}

	return pr
}

// transformTaskRun strips unnecessary fields from a TaskRun.
func transformTaskRun(tr *v1.TaskRun) *v1.TaskRun {
	// Phase 1: Strip metadata fields (all TaskRuns)
	// These fields are large and not needed for reconciliation
	tr.ManagedFields = nil

	// Strip last-applied-configuration annotation
	if tr.Annotations != nil {
		delete(tr.Annotations, LastAppliedConfigAnnotation)
	}

	// Phase 2 & 3: Strip heavy status fields from completed TaskRuns only
	// Running TaskRuns need these fields for proper reconciliation
	if tr.IsDone() {
		// Phase 2: Strip build/provenance metadata
		tr.Status.TaskSpec = nil
		tr.Status.Provenance = nil
		tr.Status.SpanContext = nil

		// Phase 3: Strip detailed execution state (Steps, Sidecars)
		// The PipelineRun controller only needs:
		// - Status.Conditions (for completion state)
		// - Status.Results (for result propagation)
		// - Status.Artifacts (for artifact aggregation)
		// The TaskRun controller doesn't access completed TaskRuns for these fields.
		//
		// NOTE: RetriesStatus, PodName, and CompletionTime are NOT stripped because:
		// - RetriesStatus: Required for retry tracking in retryTaskRun(), which appends
		//   to this slice. If stripped, previous retry history would be lost.
		// - PodName: Used for debugging and referenced by retry status entries
		// - CompletionTime: Part of the observable API and used for metrics/debugging
		tr.Status.Steps = nil
		tr.Status.Sidecars = nil
	}

	return tr
}

// transformCustomRun strips unnecessary fields from a CustomRun.
func transformCustomRun(cr *v1beta1.CustomRun) *v1beta1.CustomRun {
	// Phase 1: Strip metadata fields (all CustomRuns)
	cr.ManagedFields = nil

	// Strip last-applied-configuration annotation
	if cr.Annotations != nil {
		delete(cr.Annotations, LastAppliedConfigAnnotation)
	}

	// Phase 2: Strip heavy status fields from completed CustomRuns only
	// The PipelineRun controller only needs:
	// - Status.Conditions (for completion state)
	// - Status.Results (for result propagation)
	if cr.IsDone() {
		cr.Status.RetriesStatus = nil
		cr.Status.CompletionTime = nil
		// Note: ExtraFields is preserved as it may contain custom task-specific data
		// that could be needed for debugging or external integrations
	}

	return cr
}

// TransformPodForCache is a cache.TransformFunc that strips unnecessary fields
// from Pod objects to reduce memory usage in the TaskRun controller.
//
// The TaskRun controller needs:
// - ObjectMeta: Name, Namespace, Labels, OwnerReferences, Annotations
//   - Labels: Used for pod listing/recovery when Status.PodName is not set
//   - OwnerReferences: Used by controller.FilterController for event filtering
//
// - Status: Phase, ContainerStatuses, InitContainerStatuses, Conditions, Reason, Message
// - Spec.Containers[].Name (for sorting container statuses)
//
// Fields that can be safely stripped: ManagedFields, Finalizers, most of Spec.
func TransformPodForCache(obj interface{}) (interface{}, error) {
	// Handle tombstones (DeletedFinalStateUnknown) which wrap deleted objects
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		transformedObj, err := TransformPodForCache(tombstone.Obj)
		if err != nil {
			return obj, nil //nolint:nilerr // Intentional: return original object on error for graceful degradation
		}
		return cache.DeletedFinalStateUnknown{
			Key: tombstone.Key,
			Obj: transformedObj,
		}, nil
	}

	// Transform Pods
	if pod, ok := obj.(*corev1.Pod); ok {
		return transformPod(pod), nil
	}

	// Pass through other objects unchanged
	return obj, nil
}

// transformPod strips unnecessary fields from a Pod.
func transformPod(pod *corev1.Pod) *corev1.Pod {
	// Strip metadata fields that are not needed
	pod.ManagedFields = nil
	pod.Finalizers = nil
	// NOTE: Labels and OwnerReferences MUST be preserved:
	// - Labels: Used by TaskRun controller for pod listing/recovery
	// - OwnerReferences: Used by controller.FilterController for event filtering

	// Strip last-applied-configuration annotation but preserve others (like tekton.dev/ready)
	if pod.Annotations != nil {
		delete(pod.Annotations, LastAppliedConfigAnnotation)
	}

	// Strip Spec fields not needed by TaskRun controller
	// Keep only container names for status sorting
	strippedContainers := make([]corev1.Container, len(pod.Spec.Containers))
	for i, c := range pod.Spec.Containers {
		strippedContainers[i] = corev1.Container{
			Name: c.Name,
		}
	}

	// Replace entire Spec with minimal version
	pod.Spec = corev1.PodSpec{
		Containers: strippedContainers,
	}

	// Status is fully preserved - all fields are used by TaskRun controller

	return pod
}
