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

package transform

import (
	"context"
	"testing"
	"time"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	fakeversioned "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	externalversions "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TestTransformIntegration_PipelineRun verifies that the transform function
// is applied when objects are synced through the informer from the API server.
func TestTransformIntegration_PipelineRun(t *testing.T) {
	// Create a PipelineRun with fields that should be stripped
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:   "kubectl",
					Operation: metav1.ManagedFieldsOperationApply,
				},
			},
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: `{"large": "config"}`,
				"keep-this":                 "value",
			},
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue, // Completed
					},
				},
			},
			PipelineRunStatusFields: v1.PipelineRunStatusFields{
				PipelineSpec: &v1.PipelineSpec{
					Description: "should be stripped",
				},
				Provenance: &v1.Provenance{
					RefSource: &v1.RefSource{
						URI: "should be stripped",
					},
				},
				SpanContext: map[string]string{
					"trace-id": "should be stripped",
				},
			},
		},
	}

	// Create fake clientset with the PipelineRun
	fakeClient := fakeversioned.NewSimpleClientset(pr)

	// Create informer factory WITH our transform
	factory := externalversions.NewSharedInformerFactoryWithOptions(
		fakeClient,
		0, // No resync
		externalversions.WithTransform(TransformForCache),
	)

	// Get the PipelineRun informer
	prInformer := factory.Tekton().V1().PipelineRuns().Informer()

	// Start the informer
	stopCh := make(chan struct{})
	defer close(stopCh)
	go prInformer.Run(stopCh)

	// Wait for cache sync
	if !waitForCacheSync(prInformer.HasSynced, stopCh, 5*time.Second) {
		t.Fatal("Timed out waiting for cache sync")
	}

	// Get the object from cache
	obj, exists, err := prInformer.GetIndexer().GetByKey("default/test-pr")
	if err != nil {
		t.Fatalf("Error getting from cache: %v", err)
	}
	if !exists {
		t.Fatal("PipelineRun not found in cache")
	}

	cached := obj.(*v1.PipelineRun)

	// Verify Phase 1: Metadata fields stripped
	if len(cached.ManagedFields) != 0 {
		t.Errorf("ManagedFields should be stripped, got: %v", cached.ManagedFields)
	}

	if _, exists := cached.Annotations[LastAppliedConfigAnnotation]; exists {
		t.Error("last-applied-configuration annotation should be stripped")
	}

	if cached.Annotations["keep-this"] != "value" {
		t.Error("Other annotations should be preserved")
	}

	// Verify Phase 2: Status fields stripped (completed PipelineRun)
	if cached.Status.PipelineSpec != nil {
		t.Error("PipelineSpec should be stripped from completed PipelineRun")
	}

	if cached.Status.Provenance != nil {
		t.Error("Provenance should be stripped from completed PipelineRun")
	}

	if cached.Status.SpanContext != nil {
		t.Error("SpanContext should be stripped from completed PipelineRun")
	}
}

// TestTransformIntegration_PipelineRun_Running verifies that status fields
// are preserved for running PipelineRuns.
func TestTransformIntegration_PipelineRun_Running(t *testing.T) {
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "running-pr",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl"},
			},
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown, // Still running
					},
				},
			},
			PipelineRunStatusFields: v1.PipelineRunStatusFields{
				PipelineSpec: &v1.PipelineSpec{
					Description: "should be preserved",
				},
				Provenance: &v1.Provenance{
					RefSource: &v1.RefSource{
						URI: "should be preserved",
					},
				},
			},
		},
	}

	fakeClient := fakeversioned.NewSimpleClientset(pr)
	factory := externalversions.NewSharedInformerFactoryWithOptions(
		fakeClient,
		0,
		externalversions.WithTransform(TransformForCache),
	)

	prInformer := factory.Tekton().V1().PipelineRuns().Informer()

	stopCh := make(chan struct{})
	defer close(stopCh)
	go prInformer.Run(stopCh)

	if !waitForCacheSync(prInformer.HasSynced, stopCh, 5*time.Second) {
		t.Fatal("Timed out waiting for cache sync")
	}

	obj, exists, err := prInformer.GetIndexer().GetByKey("default/running-pr")
	if err != nil || !exists {
		t.Fatalf("Error getting from cache: %v, exists: %v", err, exists)
	}

	cached := obj.(*v1.PipelineRun)

	// ManagedFields should still be stripped
	if len(cached.ManagedFields) != 0 {
		t.Error("ManagedFields should be stripped even for running PipelineRuns")
	}

	// Status fields should be preserved for running PipelineRuns
	if cached.Status.PipelineSpec == nil {
		t.Error("PipelineSpec should be preserved for running PipelineRun")
	}

	if cached.Status.Provenance == nil {
		t.Error("Provenance should be preserved for running PipelineRun")
	}
}

// TestTransformIntegration_TaskRun verifies the transform works for TaskRuns.
func TestTransformIntegration_TaskRun(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl"},
			},
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: `{"large": "config"}`,
			},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue, // Completed
					},
				},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				TaskSpec: &v1.TaskSpec{
					Description: "should be stripped",
				},
				Provenance: &v1.Provenance{
					RefSource: &v1.RefSource{
						URI: "should be stripped",
					},
				},
				SpanContext: map[string]string{
					"trace-id": "should be stripped",
				},
			},
		},
	}

	fakeClient := fakeversioned.NewSimpleClientset(tr)
	factory := externalversions.NewSharedInformerFactoryWithOptions(
		fakeClient,
		0,
		externalversions.WithTransform(TransformForCache),
	)

	trInformer := factory.Tekton().V1().TaskRuns().Informer()

	stopCh := make(chan struct{})
	defer close(stopCh)
	go trInformer.Run(stopCh)

	if !waitForCacheSync(trInformer.HasSynced, stopCh, 5*time.Second) {
		t.Fatal("Timed out waiting for cache sync")
	}

	obj, exists, err := trInformer.GetIndexer().GetByKey("default/test-tr")
	if err != nil || !exists {
		t.Fatalf("Error getting from cache: %v, exists: %v", err, exists)
	}

	cached := obj.(*v1.TaskRun)

	// Verify Phase 1
	if len(cached.ManagedFields) != 0 {
		t.Error("ManagedFields should be stripped")
	}

	if _, exists := cached.Annotations[LastAppliedConfigAnnotation]; exists {
		t.Error("last-applied-configuration should be stripped")
	}

	// Verify Phase 2
	if cached.Status.TaskSpec != nil {
		t.Error("TaskSpec should be stripped from completed TaskRun")
	}

	if cached.Status.Provenance != nil {
		t.Error("Provenance should be stripped from completed TaskRun")
	}

	if cached.Status.SpanContext != nil {
		t.Error("SpanContext should be stripped from completed TaskRun")
	}
}

// TestTransformIntegration_FactoryWithTransform verifies the factory properly
// configures the transform on informers.
func TestTransformIntegration_FactoryWithTransform(t *testing.T) {
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "factory-test-pr",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "test"},
			},
		},
	}

	fakeClient := fakeversioned.NewSimpleClientset(pr)

	// Create factory WITHOUT transform
	factoryNoTransform := externalversions.NewSharedInformerFactory(fakeClient, 0)

	// Create factory WITH transform
	factoryWithTransform := externalversions.NewSharedInformerFactoryWithOptions(
		fakeClient,
		0,
		externalversions.WithTransform(TransformForCache),
	)

	// Test without transform
	stopCh1 := make(chan struct{})
	informer1 := factoryNoTransform.Tekton().V1().PipelineRuns().Informer()
	go informer1.Run(stopCh1)
	waitForCacheSync(informer1.HasSynced, stopCh1, 5*time.Second)

	obj1, exists, _ := informer1.GetIndexer().GetByKey("default/factory-test-pr")
	close(stopCh1)

	if !exists {
		t.Fatal("PipelineRun not found in cache without transform")
	}

	if obj1 != nil {
		cached1 := obj1.(*v1.PipelineRun)
		if len(cached1.ManagedFields) == 0 {
			t.Error("Factory WITHOUT transform should NOT strip ManagedFields")
		}
	}

	// Test with transform
	stopCh2 := make(chan struct{})
	informer2 := factoryWithTransform.Tekton().V1().PipelineRuns().Informer()
	go informer2.Run(stopCh2)
	waitForCacheSync(informer2.HasSynced, stopCh2, 5*time.Second)

	obj2, exists, _ := informer2.GetIndexer().GetByKey("default/factory-test-pr")
	close(stopCh2)

	if !exists {
		t.Fatal("PipelineRun not found in cache with transform")
	}

	cached2 := obj2.(*v1.PipelineRun)
	if len(cached2.ManagedFields) != 0 {
		t.Error("Factory WITH transform should strip ManagedFields")
	}
}

// waitForCacheSync waits for the cache to sync with a timeout.
func waitForCacheSync(hasSynced func() bool, stopCh <-chan struct{}, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 10*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		return hasSynced(), nil
	}) == nil
}
