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
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestTransformForCache_StripsManagedFields(t *testing.T) {
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
		},
	}

	result, err := TransformForCache(pr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed, ok := result.(*v1.PipelineRun)
	if !ok {
		t.Fatalf("TransformForCache() returned %T, want *v1.PipelineRun", result)
	}

	if len(transformed.ManagedFields) != 0 {
		t.Errorf("TransformForCache() ManagedFields = %v, want empty", transformed.ManagedFields)
	}
}

func TestTransformForCache_StripsLastAppliedConfiguration(t *testing.T) {
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "default",
			Annotations: map[string]string{
				"kubectl.kubernetes.io/last-applied-configuration": `{"large": "json"}`,
				"other-annotation": "should-keep",
			},
		},
	}

	result, err := TransformForCache(pr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.PipelineRun)

	if _, exists := transformed.Annotations["kubectl.kubernetes.io/last-applied-configuration"]; exists {
		t.Error("TransformForCache() should strip last-applied-configuration annotation")
	}

	if transformed.Annotations["other-annotation"] != "should-keep" {
		t.Error("TransformForCache() should preserve other annotations")
	}
}

func TestTransformForCache_StripsStatusFieldsFromCompletedPipelineRuns(t *testing.T) {
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "default",
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue, // This makes IsDone() return true
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

	result, err := TransformForCache(pr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.PipelineRun)

	if transformed.Status.PipelineSpec != nil {
		t.Error("TransformForCache() should strip PipelineSpec from completed PipelineRun")
	}

	if transformed.Status.Provenance != nil {
		t.Error("TransformForCache() should strip Provenance from completed PipelineRun")
	}

	if transformed.Status.SpanContext != nil {
		t.Error("TransformForCache() should strip SpanContext from completed PipelineRun")
	}
}

func TestTransformForCache_PreservesStatusFieldsForRunningPipelineRuns(t *testing.T) {
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "default",
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

	result, err := TransformForCache(pr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.PipelineRun)

	if transformed.Status.PipelineSpec == nil {
		t.Error("TransformForCache() should preserve PipelineSpec for running PipelineRun")
	}

	if transformed.Status.Provenance == nil {
		t.Error("TransformForCache() should preserve Provenance for running PipelineRun")
	}
}

func TestTransformForCache_HandlesTombstones(t *testing.T) {
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl"},
			},
		},
	}

	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/test-pr",
		Obj: pr,
	}

	result, err := TransformForCache(tombstone)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed, ok := result.(cache.DeletedFinalStateUnknown)
	if !ok {
		t.Fatalf("TransformForCache() returned %T, want cache.DeletedFinalStateUnknown", result)
	}

	transformedPR, ok := transformed.Obj.(*v1.PipelineRun)
	if !ok {
		t.Fatalf("TransformForCache() tombstone.Obj = %T, want *v1.PipelineRun", transformed.Obj)
	}

	if len(transformedPR.ManagedFields) != 0 {
		t.Error("TransformForCache() should strip ManagedFields from tombstone objects")
	}
}

func TestTransformForCache_PassesThroughNonTektonObjects(t *testing.T) {
	// Other object types should pass through unchanged
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl"},
			},
		},
	}

	result, err := TransformForCache(pod)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	// Should return the original object unchanged
	if result != pod {
		t.Error("TransformForCache() should pass through non-Tekton objects unchanged")
	}
}

// TaskRun tests

func TestTransformForCache_TaskRun_StripsManagedFields(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:   "kubectl",
					Operation: metav1.ManagedFieldsOperationApply,
				},
			},
		},
	}

	result, err := TransformForCache(tr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed, ok := result.(*v1.TaskRun)
	if !ok {
		t.Fatalf("TransformForCache() returned %T, want *v1.TaskRun", result)
	}

	if len(transformed.ManagedFields) != 0 {
		t.Errorf("TransformForCache() ManagedFields = %v, want empty", transformed.ManagedFields)
	}
}

func TestTransformForCache_TaskRun_StripsLastAppliedConfiguration(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: `{"large": "json"}`,
				"other-annotation":          "should-keep",
			},
		},
	}

	result, err := TransformForCache(tr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.TaskRun)

	if _, exists := transformed.Annotations[LastAppliedConfigAnnotation]; exists {
		t.Error("TransformForCache() should strip last-applied-configuration annotation from TaskRun")
	}

	if transformed.Annotations["other-annotation"] != "should-keep" {
		t.Error("TransformForCache() should preserve other annotations on TaskRun")
	}
}

func TestTransformForCache_TaskRun_StripsStatusFieldsFromCompletedTaskRuns(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue, // This makes IsDone() return true
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

	result, err := TransformForCache(tr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.TaskRun)

	if transformed.Status.TaskSpec != nil {
		t.Error("TransformForCache() should strip TaskSpec from completed TaskRun")
	}

	if transformed.Status.Provenance != nil {
		t.Error("TransformForCache() should strip Provenance from completed TaskRun")
	}

	if transformed.Status.SpanContext != nil {
		t.Error("TransformForCache() should strip SpanContext from completed TaskRun")
	}
}

func TestTransformForCache_TaskRun_PreservesStatusFieldsForRunningTaskRuns(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown, // Still running
					},
				},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				TaskSpec: &v1.TaskSpec{
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

	result, err := TransformForCache(tr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.TaskRun)

	if transformed.Status.TaskSpec == nil {
		t.Error("TransformForCache() should preserve TaskSpec for running TaskRun")
	}

	if transformed.Status.Provenance == nil {
		t.Error("TransformForCache() should preserve Provenance for running TaskRun")
	}
}

// Additional TaskRun transform tests for Phase 3: Strip more fields from completed TaskRuns
// These fields are not needed by the PipelineRun controller for completed child TaskRuns.

func TestTransformForCache_TaskRun_StripsStepsAndSidecarsFromCompletedTaskRuns(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
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
				Steps: []v1.StepState{
					{Name: "step-1", ContainerState: corev1.ContainerState{}},
					{Name: "step-2", ContainerState: corev1.ContainerState{}},
				},
				Sidecars: []v1.SidecarState{
					{Name: "sidecar-1", ContainerState: corev1.ContainerState{}},
				},
				PodName: "test-pod-abc123",
			},
		},
	}

	result, err := TransformForCache(tr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.TaskRun)

	// Steps and Sidecars should be stripped from completed TaskRuns
	if len(transformed.Status.Steps) != 0 {
		t.Errorf("TransformForCache() should strip Steps from completed TaskRun, got %d", len(transformed.Status.Steps))
	}

	if len(transformed.Status.Sidecars) != 0 {
		t.Errorf("TransformForCache() should strip Sidecars from completed TaskRun, got %d", len(transformed.Status.Sidecars))
	}

	// PodName should be preserved (needed for retry tracking and debugging)
	if transformed.Status.PodName != "test-pod-abc123" {
		t.Errorf("TransformForCache() should preserve PodName for completed TaskRun, got %q", transformed.Status.PodName)
	}
}

func TestTransformForCache_TaskRun_PreservesStepsAndSidecarsForRunningTaskRuns(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown, // Still running
					},
				},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{
					{Name: "step-1", ContainerState: corev1.ContainerState{}},
				},
				Sidecars: []v1.SidecarState{
					{Name: "sidecar-1", ContainerState: corev1.ContainerState{}},
				},
				PodName: "test-pod-abc123",
			},
		},
	}

	result, err := TransformForCache(tr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.TaskRun)

	// Steps and Sidecars should be preserved for running TaskRuns
	if len(transformed.Status.Steps) != 1 {
		t.Errorf("TransformForCache() should preserve Steps for running TaskRun, got %d", len(transformed.Status.Steps))
	}

	if len(transformed.Status.Sidecars) != 1 {
		t.Errorf("TransformForCache() should preserve Sidecars for running TaskRun, got %d", len(transformed.Status.Sidecars))
	}

	// PodName should be preserved for running TaskRuns
	if transformed.Status.PodName == "" {
		t.Error("TransformForCache() should preserve PodName for running TaskRun")
	}
}

func TestTransformForCache_TaskRun_PreservesResultsAndArtifacts(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
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
				Results: []v1.TaskRunResult{
					{Name: "digest", Type: v1.ResultsTypeString, Value: v1.ParamValue{StringVal: "sha256:abc123"}},
				},
				Artifacts: &v1.Artifacts{
					Outputs: []v1.Artifact{
						{Name: "image", Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha256": "abc123"}}}},
					},
				},
			},
		},
	}

	result, err := TransformForCache(tr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1.TaskRun)

	// Results MUST be preserved (used by PipelineRun for result propagation)
	if len(transformed.Status.Results) != 1 {
		t.Errorf("TransformForCache() should preserve Results, got %d", len(transformed.Status.Results))
	}

	// Artifacts MUST be preserved (used by PipelineRun for artifact aggregation)
	if transformed.Status.Artifacts == nil || len(transformed.Status.Artifacts.Outputs) != 1 {
		t.Error("TransformForCache() should preserve Artifacts")
	}
}

// CustomRun transform tests

func TestTransformForCache_CustomRun_StripsManagedFields(t *testing.T) {
	cr := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cr",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:   "kubectl",
					Operation: metav1.ManagedFieldsOperationApply,
				},
			},
		},
	}

	result, err := TransformForCache(cr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed, ok := result.(*v1beta1.CustomRun)
	if !ok {
		t.Fatalf("TransformForCache() returned %T, want *v1beta1.CustomRun", result)
	}

	if len(transformed.ManagedFields) != 0 {
		t.Errorf("TransformForCache() ManagedFields = %v, want empty", transformed.ManagedFields)
	}
}

func TestTransformForCache_CustomRun_StripsLastAppliedConfiguration(t *testing.T) {
	cr := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cr",
			Namespace: "default",
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: `{"large": "json"}`,
				"other-annotation":          "should-keep",
			},
		},
	}

	result, err := TransformForCache(cr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1beta1.CustomRun)

	if _, exists := transformed.Annotations[LastAppliedConfigAnnotation]; exists {
		t.Error("TransformForCache() should strip last-applied-configuration annotation from CustomRun")
	}

	if transformed.Annotations["other-annotation"] != "should-keep" {
		t.Error("TransformForCache() should preserve other annotations on CustomRun")
	}
}

func TestTransformForCache_CustomRun_StripsRetriesStatusFieldsFromCompletedCustomRuns(t *testing.T) {
	cr := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cr",
			Namespace: "default",
		},
		Status: v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue, // Completed
					},
				},
			},
			CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				RetriesStatus: []v1beta1.CustomRunStatus{
					{Status: duckv1.Status{Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}}}},
				},
			},
		},
	}

	result, err := TransformForCache(cr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1beta1.CustomRun)

	// RetriesStatus should be stripped from completed CustomRuns
	if len(transformed.Status.RetriesStatus) != 0 {
		t.Errorf("TransformForCache() should strip RetriesStatus from completed CustomRun, got %d", len(transformed.Status.RetriesStatus))
	}
}

func TestTransformForCache_CustomRun_PreservesResultsForCompletedCustomRuns(t *testing.T) {
	cr := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cr",
			Namespace: "default",
		},
		Status: v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue, // Completed
					},
				},
			},
			CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				Results: []v1beta1.CustomRunResult{
					{Name: "output", Value: "result-value"},
				},
			},
		},
	}

	result, err := TransformForCache(cr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1beta1.CustomRun)

	// Results MUST be preserved (used by PipelineRun for result propagation)
	if len(transformed.Status.Results) != 1 {
		t.Errorf("TransformForCache() should preserve Results, got %d", len(transformed.Status.Results))
	}
}

func TestTransformForCache_CustomRun_PreservesConditionsForCompletedCustomRuns(t *testing.T) {
	cr := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cr",
			Namespace: "default",
		},
		Status: v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
						Reason: "Succeeded",
					},
				},
			},
		},
	}

	result, err := TransformForCache(cr)
	if err != nil {
		t.Fatalf("TransformForCache() error = %v", err)
	}

	transformed := result.(*v1beta1.CustomRun)

	// Conditions MUST be preserved (used for status determination)
	if len(transformed.Status.Conditions) != 1 {
		t.Errorf("TransformForCache() should preserve Conditions, got %d", len(transformed.Status.Conditions))
	}
}
