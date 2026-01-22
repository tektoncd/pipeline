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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestTransformPodForCache_StripsManagedFields(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:   "kubectl",
					Operation: metav1.ManagedFieldsOperationApply,
				},
			},
		},
	}

	result, err := TransformPodForCache(pod)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	transformed, ok := result.(*corev1.Pod)
	if !ok {
		t.Fatalf("TransformPodForCache() returned %T, want *corev1.Pod", result)
	}

	if len(transformed.ManagedFields) != 0 {
		t.Errorf("TransformPodForCache() ManagedFields = %v, want empty", transformed.ManagedFields)
	}
}

func TestTransformPodForCache_StripsLastAppliedConfiguration(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: `{"large": "json"}`,
				"tekton.dev/ready":          "READY",
				"other-annotation":          "should-keep",
			},
		},
	}

	result, err := TransformPodForCache(pod)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	transformed := result.(*corev1.Pod)

	if _, exists := transformed.Annotations[LastAppliedConfigAnnotation]; exists {
		t.Error("TransformPodForCache() should strip last-applied-configuration annotation")
	}

	// Should preserve tekton.dev/ready annotation (used by entrypoint)
	if transformed.Annotations["tekton.dev/ready"] != "READY" {
		t.Error("TransformPodForCache() should preserve tekton.dev/ready annotation")
	}

	if transformed.Annotations["other-annotation"] != "should-keep" {
		t.Error("TransformPodForCache() should preserve other annotations")
	}
}

func TestTransformPodForCache_PreservesRequiredMetadata(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			UID:       "test-uid",
			Labels: map[string]string{
				"tekton.dev/taskRun": "my-taskrun",
				"tekton.dev/task":    "my-task",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "tekton.dev/v1",
					Kind:       "TaskRun",
					Name:       "my-taskrun",
					UID:        "taskrun-uid",
				},
			},
		},
	}

	result, err := TransformPodForCache(pod)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	transformed := result.(*corev1.Pod)

	// Name and Namespace MUST be preserved
	if transformed.Name != "test-pod" {
		t.Errorf("TransformPodForCache() Name = %q, want %q", transformed.Name, "test-pod")
	}
	if transformed.Namespace != "test-ns" {
		t.Errorf("TransformPodForCache() Namespace = %q, want %q", transformed.Namespace, "test-ns")
	}

	// Labels MUST be preserved - used by TaskRun controller for pod listing/recovery
	// When TaskRun doesn't have Status.PodName (e.g., after crash), it lists Pods by label
	if len(transformed.Labels) != 2 {
		t.Errorf("TransformPodForCache() should preserve Labels, got %d", len(transformed.Labels))
	}
	if transformed.Labels["tekton.dev/taskRun"] != "my-taskrun" {
		t.Error("TransformPodForCache() should preserve tekton.dev/taskRun label")
	}

	// OwnerReferences MUST be preserved - used by controller.FilterController for event filtering
	// Maps Pod events back to their parent TaskRun for reconciliation
	if len(transformed.OwnerReferences) != 1 {
		t.Errorf("TransformPodForCache() should preserve OwnerReferences, got %d", len(transformed.OwnerReferences))
	}
	if transformed.OwnerReferences[0].Name != "my-taskrun" {
		t.Error("TransformPodForCache() should preserve OwnerReference name")
	}
}

func TestTransformPodForCache_PreservesStatusFields(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase:   corev1.PodRunning,
			Reason:  "test-reason",
			Message: "test-message",
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodScheduled,
					Status: corev1.ConditionTrue,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "step-build",
					Ready: true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "prepare",
					Ready: true,
				},
			},
		},
	}

	result, err := TransformPodForCache(pod)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	transformed := result.(*corev1.Pod)

	// All status fields should be preserved
	if transformed.Status.Phase != corev1.PodRunning {
		t.Errorf("TransformPodForCache() Status.Phase = %v, want %v", transformed.Status.Phase, corev1.PodRunning)
	}
	if transformed.Status.Reason != "test-reason" {
		t.Errorf("TransformPodForCache() Status.Reason = %q, want %q", transformed.Status.Reason, "test-reason")
	}
	if transformed.Status.Message != "test-message" {
		t.Errorf("TransformPodForCache() Status.Message = %q, want %q", transformed.Status.Message, "test-message")
	}
	if len(transformed.Status.Conditions) != 1 {
		t.Errorf("TransformPodForCache() Status.Conditions length = %d, want 1", len(transformed.Status.Conditions))
	}
	if len(transformed.Status.ContainerStatuses) != 1 {
		t.Errorf("TransformPodForCache() Status.ContainerStatuses length = %d, want 1", len(transformed.Status.ContainerStatuses))
	}
	if len(transformed.Status.InitContainerStatuses) != 1 {
		t.Errorf("TransformPodForCache() Status.InitContainerStatuses length = %d, want 1", len(transformed.Status.InitContainerStatuses))
	}
}

func TestTransformPodForCache_PreservesContainerNames(t *testing.T) {
	// The TaskRun controller uses Spec.Containers[].Name for sorting container statuses
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "step-build",
					Image:   "golang:1.21",
					Command: []string{"go", "build"},
					Env: []corev1.EnvVar{
						{Name: "GOOS", Value: "linux"},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workspace", MountPath: "/workspace"},
					},
				},
				{
					Name:  "step-test",
					Image: "golang:1.21",
				},
			},
		},
	}

	result, err := TransformPodForCache(pod)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	transformed := result.(*corev1.Pod)

	// Container names MUST be preserved
	if len(transformed.Spec.Containers) != 2 {
		t.Fatalf("TransformPodForCache() Spec.Containers length = %d, want 2", len(transformed.Spec.Containers))
	}
	if transformed.Spec.Containers[0].Name != "step-build" {
		t.Errorf("TransformPodForCache() Spec.Containers[0].Name = %q, want %q", transformed.Spec.Containers[0].Name, "step-build")
	}
	if transformed.Spec.Containers[1].Name != "step-test" {
		t.Errorf("TransformPodForCache() Spec.Containers[1].Name = %q, want %q", transformed.Spec.Containers[1].Name, "step-test")
	}

	// Other container fields should be stripped
	if transformed.Spec.Containers[0].Image != "" {
		t.Errorf("TransformPodForCache() should strip Container.Image, got %q", transformed.Spec.Containers[0].Image)
	}
	if len(transformed.Spec.Containers[0].Command) != 0 {
		t.Errorf("TransformPodForCache() should strip Container.Command, got %v", transformed.Spec.Containers[0].Command)
	}
	if len(transformed.Spec.Containers[0].Env) != 0 {
		t.Errorf("TransformPodForCache() should strip Container.Env, got %v", transformed.Spec.Containers[0].Env)
	}
	if len(transformed.Spec.Containers[0].VolumeMounts) != 0 {
		t.Errorf("TransformPodForCache() should strip Container.VolumeMounts, got %v", transformed.Spec.Containers[0].VolumeMounts)
	}
	if transformed.Spec.Containers[0].Resources.Requests != nil {
		t.Errorf("TransformPodForCache() should strip Container.Resources, got %v", transformed.Spec.Containers[0].Resources)
	}
}

func TestTransformPodForCache_StripsSpecFields(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "builder",
			NodeSelector: map[string]string{
				"kubernetes.io/arch": "amd64",
			},
			Tolerations: []corev1.Toleration{
				{Key: "dedicated", Operator: corev1.TolerationOpExists},
			},
			Volumes: []corev1.Volume{
				{Name: "workspace"},
			},
			InitContainers: []corev1.Container{
				{Name: "init", Image: "busybox"},
			},
			RestartPolicy:   corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{},
			Affinity:        &corev1.Affinity{},
		},
	}

	result, err := TransformPodForCache(pod)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	transformed := result.(*corev1.Pod)

	// All these Spec fields should be stripped
	if transformed.Spec.ServiceAccountName != "" {
		t.Errorf("TransformPodForCache() should strip Spec.ServiceAccountName")
	}
	if len(transformed.Spec.NodeSelector) != 0 {
		t.Errorf("TransformPodForCache() should strip Spec.NodeSelector")
	}
	if len(transformed.Spec.Tolerations) != 0 {
		t.Errorf("TransformPodForCache() should strip Spec.Tolerations")
	}
	if len(transformed.Spec.Volumes) != 0 {
		t.Errorf("TransformPodForCache() should strip Spec.Volumes")
	}
	if len(transformed.Spec.InitContainers) != 0 {
		t.Errorf("TransformPodForCache() should strip Spec.InitContainers")
	}
	if transformed.Spec.SecurityContext != nil {
		t.Errorf("TransformPodForCache() should strip Spec.SecurityContext")
	}
	if transformed.Spec.Affinity != nil {
		t.Errorf("TransformPodForCache() should strip Spec.Affinity")
	}
}

func TestTransformPodForCache_HandlesTombstones(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl"},
			},
		},
	}

	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/test-pod",
		Obj: pod,
	}

	result, err := TransformPodForCache(tombstone)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	transformed, ok := result.(cache.DeletedFinalStateUnknown)
	if !ok {
		t.Fatalf("TransformPodForCache() returned %T, want cache.DeletedFinalStateUnknown", result)
	}

	transformedPod, ok := transformed.Obj.(*corev1.Pod)
	if !ok {
		t.Fatalf("TransformPodForCache() tombstone.Obj = %T, want *corev1.Pod", transformed.Obj)
	}

	if len(transformedPod.ManagedFields) != 0 {
		t.Error("TransformPodForCache() should strip ManagedFields from tombstone objects")
	}
}

func TestTransformPodForCache_PassesThroughNonPodObjects(t *testing.T) {
	// Other object types should pass through unchanged
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-service",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl"},
			},
		},
	}

	result, err := TransformPodForCache(service)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	// Should return the original object unchanged
	if result != service {
		t.Error("TransformPodForCache() should pass through non-Pod objects unchanged")
	}
}

// Benchmark test to measure memory savings
func BenchmarkTransformPodForCache(b *testing.B) {
	pod := createRealisticPod()

	b.ResetTimer()
	for range b.N {
		_, _ = TransformPodForCache(pod)
	}
}

func createRealisticPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-pod",
			Namespace: "default",
			Labels: map[string]string{
				"tekton.dev/task":        "build",
				"tekton.dev/taskRun":     "build-run-abc123",
				"tekton.dev/pipelineRun": "pipeline-run-xyz",
			},
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: `{"very": "large", "configuration": "data that can be several KB"}`,
				"tekton.dev/ready":          "READY",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
				{Manager: "tekton-pipelines-controller", Operation: metav1.ManagedFieldsOperationUpdate},
			},
			OwnerReferences: []metav1.OwnerReference{
				{Name: "build-run-abc123", Kind: "TaskRun"},
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "pipeline",
			RestartPolicy:      corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{Name: "workspace"},
				{Name: "tekton-internal-scripts"},
				{Name: "tekton-internal-home"},
				{Name: "tekton-internal-results"},
			},
			InitContainers: []corev1.Container{
				{
					Name:    "prepare",
					Image:   "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/entrypoint",
					Command: []string{"/ko-app/entrypoint", "init"},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "step-clone",
					Image:   "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init",
					Command: []string{"/ko-app/git-init"},
					Env: []corev1.EnvVar{
						{Name: "HOME", Value: "/tekton/home"},
						{Name: "PARAM_URL", Value: "https://github.com/example/repo"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workspace", MountPath: "/workspace"},
					},
				},
				{
					Name:    "step-build",
					Image:   "golang:1.21",
					Command: []string{"go", "build", "-o", "/workspace/output", "./..."},
				},
				{
					Name:    "step-test",
					Image:   "golang:1.21",
					Command: []string{"go", "test", "./..."},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:   corev1.PodSucceeded,
			Reason:  "Completed",
			Message: "All containers completed successfully",
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
				{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
				{Type: corev1.PodReady, Status: corev1.ConditionFalse},
				{Type: corev1.ContainersReady, Status: corev1.ConditionFalse},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "step-clone",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Reason:   "Completed",
						},
					},
				},
				{
					Name:  "step-build",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Reason:   "Completed",
						},
					},
				},
				{
					Name:  "step-test",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Reason:   "Completed",
						},
					},
				},
			},
		},
	}
}
