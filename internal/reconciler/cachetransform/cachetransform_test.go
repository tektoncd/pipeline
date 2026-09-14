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

package cachetransform

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func managedFields() []metav1.ManagedFieldsEntry {
	return []metav1.ManagedFieldsEntry{{
		Manager:    "kubectl",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		APIVersion: "tekton.dev/v1",
	}}
}

func annotations() map[string]string {
	return map[string]string{
		LastAppliedConfigAnnotation: `{"large":"json"}`,
		"tekton.dev/keep":           "yes",
	}
}

// assertStrippedMetadata verifies the universally-safe metadata fields were
// stripped while other annotations were preserved.
func assertStrippedMetadata(t *testing.T, meta metav1.ObjectMeta) {
	t.Helper()
	if meta.ManagedFields != nil {
		t.Errorf("expected ManagedFields to be stripped, got %v", meta.ManagedFields)
	}
	if _, exists := meta.Annotations[LastAppliedConfigAnnotation]; exists {
		t.Errorf("expected %s annotation to be stripped", LastAppliedConfigAnnotation)
	}
	if got := meta.Annotations["tekton.dev/keep"]; got != "yes" {
		t.Errorf("expected unrelated annotation to be preserved, got %q", got)
	}
}

func TestForTektonResource_StripsMetadata(t *testing.T) {
	doneStatus := duckv1.Status{Conditions: duckv1.Conditions{{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	}}}

	tests := []struct {
		name string
		obj  interface{}
		meta func(interface{}) metav1.ObjectMeta
	}{{
		name: "completed PipelineRun keeps pipelineSpec",
		obj: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{ManagedFields: managedFields(), Annotations: annotations()},
			Status: v1.PipelineRunStatus{
				Status:                  doneStatus,
				PipelineRunStatusFields: v1.PipelineRunStatusFields{PipelineSpec: &v1.PipelineSpec{}},
			},
		},
		meta: func(o interface{}) metav1.ObjectMeta { return o.(*v1.PipelineRun).ObjectMeta },
	}, {
		name: "completed TaskRun keeps taskSpec/steps/sidecars",
		obj: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{ManagedFields: managedFields(), Annotations: annotations()},
			Status: v1.TaskRunStatus{
				Status: doneStatus,
				TaskRunStatusFields: v1.TaskRunStatusFields{
					TaskSpec: &v1.TaskSpec{},
					Steps:    []v1.StepState{{Name: "step1"}},
					Sidecars: []v1.SidecarState{{Name: "sidecar1"}},
				},
			},
		},
		meta: func(o interface{}) metav1.ObjectMeta { return o.(*v1.TaskRun).ObjectMeta },
	}, {
		name: "completed CustomRun",
		obj: &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{ManagedFields: managedFields(), Annotations: annotations()},
			Status:     v1beta1.CustomRunStatus{Status: doneStatus},
		},
		meta: func(o interface{}) metav1.ObjectMeta { return o.(*v1beta1.CustomRun).ObjectMeta },
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ForTektonResource(tc.obj)
			if err != nil {
				t.Fatalf("ForTektonResource returned error: %v", err)
			}
			assertStrippedMetadata(t, tc.meta(out))
		})
	}
}

func TestForTektonResource_PreservesLoadBearingStatusFields(t *testing.T) {
	done := duckv1.Status{Conditions: duckv1.Conditions{{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue}}}

	t.Run("PipelineRun pipelineSpec", func(t *testing.T) {
		pr := &v1.PipelineRun{Status: v1.PipelineRunStatus{
			Status:                  done,
			PipelineRunStatusFields: v1.PipelineRunStatusFields{PipelineSpec: &v1.PipelineSpec{}},
		}}
		out, _ := ForTektonResource(pr)
		if out.(*v1.PipelineRun).Status.PipelineSpec == nil {
			t.Error("expected completed PipelineRun.Status.PipelineSpec to be preserved")
		}
	})

	t.Run("TaskRun taskSpec/steps/sidecars", func(t *testing.T) {
		tr := &v1.TaskRun{Status: v1.TaskRunStatus{
			Status: done,
			TaskRunStatusFields: v1.TaskRunStatusFields{
				TaskSpec: &v1.TaskSpec{},
				Steps:    []v1.StepState{{Name: "step1"}},
				Sidecars: []v1.SidecarState{{Name: "sidecar1"}},
			},
		}}
		out, _ := ForTektonResource(tr)
		st := out.(*v1.TaskRun).Status
		if st.TaskSpec == nil {
			t.Error("expected completed TaskRun.Status.TaskSpec to be preserved")
		}
		if len(st.Steps) != 1 {
			t.Error("expected completed TaskRun.Status.Steps to be preserved")
		}
		if len(st.Sidecars) != 1 {
			t.Error("expected completed TaskRun.Status.Sidecars to be preserved")
		}
	})
}

func TestForTektonResource_PassesThroughUnknownObjects(t *testing.T) {
	in := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}
	out, err := ForTektonResource(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("expected unknown object to pass through unchanged (-want +got):\n%s", diff)
	}
}

func TestForTektonResource_HandlesTombstones(t *testing.T) {
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "ns/pr",
		Obj: &v1.PipelineRun{ObjectMeta: metav1.ObjectMeta{ManagedFields: managedFields(), Annotations: annotations()}},
	}
	out, err := ForTektonResource(tombstone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wrapped, ok := out.(cache.DeletedFinalStateUnknown)
	if !ok {
		t.Fatalf("expected DeletedFinalStateUnknown, got %T", out)
	}
	if wrapped.Key != "ns/pr" {
		t.Errorf("expected tombstone key preserved, got %q", wrapped.Key)
	}
	assertStrippedMetadata(t, wrapped.Obj.(*v1.PipelineRun).ObjectMeta)
}

func TestForPod_StripsMetadataAndPreservesSpecStatus(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pod",
			Namespace:       "ns",
			ManagedFields:   managedFields(),
			Annotations:     annotations(),
			Labels:          map[string]string{"app": "tekton"},
			OwnerReferences: []metav1.OwnerReference{{Name: "owner"}},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "step-a", Image: "img"}},
			Volumes:    []corev1.Volume{{Name: "ws"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	out, err := ForPod(pod)
	if err != nil {
		t.Fatalf("ForPod returned error: %v", err)
	}
	got := out.(*corev1.Pod)

	assertStrippedMetadata(t, got.ObjectMeta)
	// Required metadata preserved.
	if got.Labels["app"] != "tekton" {
		t.Error("expected Labels to be preserved")
	}
	if len(got.OwnerReferences) != 1 {
		t.Error("expected OwnerReferences to be preserved")
	}
	// Spec and status preserved (only metadata is stripped).
	if diff := cmp.Diff(pod.Spec, got.Spec); diff != "" {
		t.Errorf("expected Pod.Spec preserved (-want +got):\n%s", diff)
	}
	if got.Status.Phase != corev1.PodSucceeded {
		t.Error("expected Pod.Status to be preserved")
	}
}

func TestForPod_PassesThroughNonPods(t *testing.T) {
	in := &v1.TaskRun{ObjectMeta: metav1.ObjectMeta{Name: "tr"}}
	out, err := ForPod(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("expected non-pod to pass through unchanged (-want +got):\n%s", diff)
	}
}

func TestForPod_HandlesTombstones(t *testing.T) {
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "ns/pod",
		Obj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{ManagedFields: managedFields(), Annotations: annotations()}},
	}
	out, err := ForPod(tombstone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wrapped, ok := out.(cache.DeletedFinalStateUnknown)
	if !ok {
		t.Fatalf("expected DeletedFinalStateUnknown, got %T", out)
	}
	assertStrippedMetadata(t, wrapped.Obj.(*corev1.Pod).ObjectMeta)
}
