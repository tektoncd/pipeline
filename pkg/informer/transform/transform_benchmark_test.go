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
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// createRealisticPipelineRun creates a PipelineRun with realistic field sizes
// that would be seen in production environments.
func createRealisticPipelineRun(name string, completed bool) *v1.PipelineRun {
	// Create realistic managedFields (typically 500-2000 bytes)
	// FieldsV1.Raw must contain valid JSON
	fieldsV1Data1 := []byte(`{"f:metadata":{"f:annotations":{"f:kubectl.kubernetes.io/last-applied-configuration":{}},"f:labels":{"f:tekton.dev/pipeline":{}}},"f:spec":{"f:pipelineRef":{"f:name":{}},"f:params":{}}}`)
	fieldsV1Data2 := []byte(`{"f:status":{"f:conditions":{},"f:pipelineSpec":{},"f:provenance":{},"f:childReferences":{},"f:startTime":{},"f:completionTime":{}}}`)

	managedFields := []metav1.ManagedFieldsEntry{
		{
			Manager:    "kubectl-client-side-apply",
			Operation:  metav1.ManagedFieldsOperationApply,
			APIVersion: "tekton.dev/v1",
			Time:       &metav1.Time{},
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: fieldsV1Data1},
		},
		{
			Manager:    "tekton-pipelines-controller",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			APIVersion: "tekton.dev/v1",
			Time:       &metav1.Time{},
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: fieldsV1Data2},
		},
	}

	// Create realistic last-applied-configuration (typically 2-10KB)
	lastApplied := make(map[string]interface{})
	lastApplied["apiVersion"] = "tekton.dev/v1"
	lastApplied["kind"] = "PipelineRun"
	lastApplied["metadata"] = map[string]interface{}{
		"name":      name,
		"namespace": "default",
	}
	lastApplied["spec"] = map[string]interface{}{
		"pipelineRef": map[string]interface{}{
			"name": "my-pipeline",
		},
		"params": []map[string]interface{}{
			{"name": "param1", "value": "value1"},
			{"name": "param2", "value": "value2"},
			{"name": "param3", "value": "value3"},
		},
	}
	lastAppliedJSON, _ := json.Marshal(lastApplied) //nolint:errchkjson // Test helper, error handling not needed

	// Create realistic PipelineSpec (typically 5-50KB)
	pipelineSpec := &v1.PipelineSpec{
		Description: "A realistic pipeline for testing memory usage",
		Params: []v1.ParamSpec{
			{Name: "param1", Type: v1.ParamTypeString, Default: &v1.ParamValue{Type: v1.ParamTypeString, StringVal: "default1"}},
			{Name: "param2", Type: v1.ParamTypeString, Default: &v1.ParamValue{Type: v1.ParamTypeString, StringVal: "default2"}},
			{Name: "param3", Type: v1.ParamTypeArray, Default: &v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"a", "b", "c"}}},
		},
		Tasks: []v1.PipelineTask{
			{
				Name: "task-1",
				TaskRef: &v1.TaskRef{
					Name: "my-task-1",
				},
				Params: v1.Params{
					{Name: "p1", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(params.param1)"}},
				},
			},
			{
				Name: "task-2",
				TaskRef: &v1.TaskRef{
					Name: "my-task-2",
				},
				RunAfter: []string{"task-1"},
			},
			{
				Name: "task-3",
				TaskRef: &v1.TaskRef{
					Name: "my-task-3",
				},
				RunAfter: []string{"task-1"},
			},
		},
	}

	// Create realistic Provenance
	provenance := &v1.Provenance{
		RefSource: &v1.RefSource{
			URI:    "https://github.com/tektoncd/pipeline.git",
			Digest: map[string]string{"sha1": "abc123def456"},
		},
		FeatureFlags: &config.FeatureFlags{
			EnableAPIFields: "stable",
		},
	}

	conditionStatus := corev1.ConditionUnknown
	if completed {
		conditionStatus = corev1.ConditionTrue
	}

	return &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:          name,
			Namespace:     "default",
			ManagedFields: managedFields,
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: string(lastAppliedJSON),
				"tekton.dev/created-by":     "user@example.com",
			},
			Labels: map[string]string{
				"tekton.dev/pipeline": "my-pipeline",
			},
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: conditionStatus,
					},
				},
			},
			PipelineRunStatusFields: v1.PipelineRunStatusFields{
				PipelineSpec: pipelineSpec,
				Provenance:   provenance,
				SpanContext: map[string]string{
					"trace-id": "abc123",
					"span-id":  "def456",
				},
				ChildReferences: []v1.ChildStatusReference{
					{Name: "task-1-run", PipelineTaskName: "task-1"},
					{Name: "task-2-run", PipelineTaskName: "task-2"},
					{Name: "task-3-run", PipelineTaskName: "task-3"},
				},
			},
		},
	}
}

// measureObjectSize returns the approximate JSON size of an object in bytes.
func measureObjectSize(obj interface{}) int {
	data, _ := json.Marshal(obj) //nolint:errchkjson // Test helper, error handling not needed
	return len(data)
}

// BenchmarkTransformMemorySavings benchmarks the memory savings from the transform.
func BenchmarkTransformMemorySavings(b *testing.B) {
	// Create a completed PipelineRun (will have status fields stripped)
	original := createRealisticPipelineRun("benchmark-pr", true)
	originalSize := measureObjectSize(original)

	b.Run("Original", func(b *testing.B) {
		b.ReportAllocs()
		for i := range b.N {
			pr := createRealisticPipelineRun(fmt.Sprintf("pr-%d", i), true)
			_ = measureObjectSize(pr)
		}
	})

	b.Run("Transformed", func(b *testing.B) {
		b.ReportAllocs()
		for i := range b.N {
			pr := createRealisticPipelineRun(fmt.Sprintf("pr-%d", i), true)
			transformed, _ := TransformForCache(pr)
			_ = measureObjectSize(transformed)
		}
	})

	// Report the size difference
	transformed, _ := TransformForCache(original.DeepCopy())
	transformedSize := measureObjectSize(transformed)
	reduction := float64(originalSize-transformedSize) / float64(originalSize) * 100

	b.Logf("Original size: %d bytes", originalSize)
	b.Logf("Transformed size: %d bytes", transformedSize)
	b.Logf("Reduction: %.1f%%", reduction)
}

// BenchmarkCacheMemoryUsage simulates caching many PipelineRuns and measures memory.
func BenchmarkCacheMemoryUsage(b *testing.B) {
	const numObjects = 1000

	b.Run("WithoutTransform", func(b *testing.B) {
		b.ReportAllocs()
		runtime.GC()
		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		cache := make([]*v1.PipelineRun, numObjects)
		for i := range numObjects {
			cache[i] = createRealisticPipelineRun(fmt.Sprintf("pr-%d", i), true)
		}

		runtime.GC()
		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		memUsed := memAfter.HeapAlloc - memBefore.HeapAlloc
		b.Logf("Memory for %d PipelineRuns (no transform): %d bytes (%.2f KB each)",
			numObjects, memUsed, float64(memUsed)/float64(numObjects)/1024)

		// Keep cache alive
		runtime.KeepAlive(cache)
	})

	b.Run("WithTransform", func(b *testing.B) {
		b.ReportAllocs()
		runtime.GC()
		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		cache := make([]interface{}, numObjects)
		for i := range numObjects {
			pr := createRealisticPipelineRun(fmt.Sprintf("pr-%d", i), true)
			cache[i], _ = TransformForCache(pr)
		}

		runtime.GC()
		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		memUsed := memAfter.HeapAlloc - memBefore.HeapAlloc
		b.Logf("Memory for %d PipelineRuns (with transform): %d bytes (%.2f KB each)",
			numObjects, memUsed, float64(memUsed)/float64(numObjects)/1024)

		// Keep cache alive
		runtime.KeepAlive(cache)
	})
}

// TestMeasureTransformSavings is a test that prints the memory savings.
// Run with: go test -v -run TestMeasureTransformSavings
func TestMeasureTransformSavings(t *testing.T) {
	// Measure JSON size (serialized object size - most reliable metric)
	original := createRealisticPipelineRun("test", true)
	originalCopy := original.DeepCopy()
	transformed, _ := TransformForCache(originalCopy)

	originalSize := measureObjectSize(original)
	transformedSize := measureObjectSize(transformed)

	if originalSize == 0 {
		t.Fatal("Original size is 0, JSON marshaling failed")
	}

	jsonSavings := float64(originalSize-transformedSize) / float64(originalSize) * 100

	t.Logf("\n=== JSON Size Report (Serialized Object Size) ===")
	t.Logf("Original JSON size:    %d bytes", originalSize)
	t.Logf("Transformed JSON size: %d bytes", transformedSize)
	t.Logf("Size saved:            %d bytes (%.1f%%)", originalSize-transformedSize, jsonSavings)
	t.Logf("=================================================")

	// Verify fields were actually stripped
	transformedPR := transformed.(*v1.PipelineRun)
	t.Logf("\n=== Fields Stripped ===")
	t.Logf("ManagedFields:              %v (was %d entries)", len(transformedPR.ManagedFields) == 0, len(original.ManagedFields))
	t.Logf("last-applied-configuration: %v", transformedPR.Annotations[LastAppliedConfigAnnotation] == "")
	t.Logf("PipelineSpec:               %v", transformedPR.Status.PipelineSpec == nil)
	t.Logf("Provenance:                 %v", transformedPR.Status.Provenance == nil)
	t.Logf("SpanContext:                %v", transformedPR.Status.SpanContext == nil)
	t.Logf("=======================")

	// Measure individual field sizes
	t.Logf("\n=== Individual Field Sizes ===")
	managedFieldsSize := measureObjectSize(original.ManagedFields)
	lastAppliedSize := len(original.Annotations[LastAppliedConfigAnnotation])
	pipelineSpecSize := measureObjectSize(original.Status.PipelineSpec)
	provenanceSize := measureObjectSize(original.Status.Provenance)
	spanContextSize := measureObjectSize(original.Status.SpanContext)

	t.Logf("ManagedFields:              %d bytes", managedFieldsSize)
	t.Logf("last-applied-configuration: %d bytes", lastAppliedSize)
	t.Logf("PipelineSpec:               %d bytes", pipelineSpecSize)
	t.Logf("Provenance:                 %d bytes", provenanceSize)
	t.Logf("SpanContext:                %d bytes", spanContextSize)
	t.Logf("Total stripped:             %d bytes", managedFieldsSize+lastAppliedSize+pipelineSpecSize+provenanceSize+spanContextSize)
	t.Logf("===============================")

	// Assertions
	if jsonSavings < 20 {
		t.Errorf("Expected at least 20%% size reduction, got %.1f%%", jsonSavings)
	}
}

// TestMeasurePodTransformSavings measures the memory savings from stripping Pod fields.
func TestMeasurePodTransformSavings(t *testing.T) {
	// Create a realistic Pod (like one created by TaskRun controller)
	original := createRealisticTaskRunPod()

	// Measure original size BEFORE transforming (transform mutates in place)
	originalJSON, _ := json.Marshal(original) //nolint:errchkjson // Test helper, error handling not needed

	// Create another instance for transformation
	toTransform := createRealisticTaskRunPod()

	// Transform the Pod
	transformed, err := TransformPodForCache(toTransform)
	if err != nil {
		t.Fatalf("TransformPodForCache() error = %v", err)
	}

	// Measure transformed size
	transformedJSON, _ := json.Marshal(transformed) //nolint:errchkjson // Test helper, error handling not needed

	originalSize := len(originalJSON)
	transformedSize := len(transformedJSON)
	jsonSavings := float64(originalSize-transformedSize) / float64(originalSize) * 100

	t.Logf("\n=== Pod JSON Size Report (Serialized Object Size) ===")
	t.Logf("Original JSON size:    %d bytes", originalSize)
	t.Logf("Transformed JSON size: %d bytes", transformedSize)
	t.Logf("Size saved:            %d bytes (%.1f%%)", originalSize-transformedSize, jsonSavings)
	t.Logf("=====================================================")

	// Verify fields were actually stripped
	transformedPod := transformed.(*corev1.Pod)
	t.Logf("\n=== Fields Stripped ===")
	t.Logf("ManagedFields:              %v (was %d entries)", len(transformedPod.ManagedFields) == 0, len(original.ManagedFields))
	t.Logf("last-applied-configuration: %v", transformedPod.Annotations[LastAppliedConfigAnnotation] == "")
	t.Logf("OwnerReferences:            %v (was %d entries)", len(transformedPod.OwnerReferences) == 0, len(original.OwnerReferences))
	t.Logf("Labels:                     %v (was %d entries)", len(transformedPod.Labels) == 0, len(original.Labels))
	t.Logf("Spec.Volumes:               %v (was %d entries)", len(transformedPod.Spec.Volumes) == 0, len(original.Spec.Volumes))
	t.Logf("Spec.InitContainers:        %v (was %d entries)", len(transformedPod.Spec.InitContainers) == 0, len(original.Spec.InitContainers))
	t.Logf("Spec.ServiceAccountName:    %v", transformedPod.Spec.ServiceAccountName == "")
	t.Logf("=======================")

	// Verify container names are preserved
	if len(transformedPod.Spec.Containers) != len(original.Spec.Containers) {
		t.Errorf("Container count mismatch: got %d, want %d", len(transformedPod.Spec.Containers), len(original.Spec.Containers))
	}
	for i, c := range transformedPod.Spec.Containers {
		if c.Name != original.Spec.Containers[i].Name {
			t.Errorf("Container[%d].Name mismatch: got %q, want %q", i, c.Name, original.Spec.Containers[i].Name)
		}
	}

	// Verify status is fully preserved
	if transformedPod.Status.Phase != original.Status.Phase {
		t.Errorf("Status.Phase mismatch: got %v, want %v", transformedPod.Status.Phase, original.Status.Phase)
	}
	if len(transformedPod.Status.ContainerStatuses) != len(original.Status.ContainerStatuses) {
		t.Errorf("ContainerStatuses count mismatch: got %d, want %d", len(transformedPod.Status.ContainerStatuses), len(original.Status.ContainerStatuses))
	}

	// Measure individual field sizes
	t.Logf("\n=== Individual Field Sizes ===")
	managedFieldsSize := measureObjectSize(original.ManagedFields)
	lastAppliedSize := len(original.Annotations[LastAppliedConfigAnnotation])
	volumesSize := measureObjectSize(original.Spec.Volumes)
	initContainersSize := measureObjectSize(original.Spec.InitContainers)
	containersEnvSize := 0
	for _, c := range original.Spec.Containers {
		containersEnvSize += measureObjectSize(c.Env)
		containersEnvSize += measureObjectSize(c.VolumeMounts)
		containersEnvSize += measureObjectSize(c.Command)
		containersEnvSize += measureObjectSize(c.Args)
	}

	t.Logf("ManagedFields:              %d bytes", managedFieldsSize)
	t.Logf("last-applied-configuration: %d bytes", lastAppliedSize)
	t.Logf("Spec.Volumes:               %d bytes", volumesSize)
	t.Logf("Spec.InitContainers:        %d bytes", initContainersSize)
	t.Logf("Container fields (env/cmd): %d bytes", containersEnvSize)
	t.Logf("Total estimated stripped:   %d bytes", managedFieldsSize+lastAppliedSize+volumesSize+initContainersSize+containersEnvSize)
	t.Logf("===============================")

	// Assertions - Pod transforms should save significant space
	if jsonSavings < 40 {
		t.Errorf("Expected at least 40%% size reduction for Pod, got %.1f%%", jsonSavings)
	}
}

// createRealisticTaskRunPod creates a Pod similar to what TaskRun controller creates.
func createRealisticTaskRunPod() *corev1.Pod {
	// Create realistic managedFields
	fieldsV1Data1 := []byte(`{"f:metadata":{"f:annotations":{"f:tekton.dev/ready":{}},"f:labels":{"f:tekton.dev/task":{},"f:tekton.dev/taskRun":{}}},"f:spec":{"f:containers":{},"f:volumes":{}}}`)
	fieldsV1Data2 := []byte(`{"f:status":{"f:conditions":{},"f:containerStatuses":{},"f:phase":{}}}`)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-taskrun-pod-abc123",
			Namespace: "default",
			Labels: map[string]string{
				"tekton.dev/task":              "build-task",
				"tekton.dev/taskRun":           "build-taskrun-abc123",
				"tekton.dev/pipelineRun":       "pipeline-run-xyz789",
				"tekton.dev/pipeline":          "my-pipeline",
				"tekton.dev/pipelineTask":      "build",
				"app.kubernetes.io/managed-by": "tekton-pipelines",
			},
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"my-taskrun-pod-abc123","namespace":"default","labels":{"tekton.dev/task":"build-task"}},"spec":{"containers":[{"name":"step-build","image":"golang:1.21"}]}}`,
				"tekton.dev/ready":          "READY",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "tekton-pipelines-controller",
					Operation:  metav1.ManagedFieldsOperationUpdate,
					APIVersion: "v1",
					FieldsType: "FieldsV1",
					FieldsV1:   &metav1.FieldsV1{Raw: fieldsV1Data1},
				},
				{
					Manager:    "kubelet",
					Operation:  metav1.ManagedFieldsOperationUpdate,
					APIVersion: "v1",
					FieldsType: "FieldsV1",
					FieldsV1:   &metav1.FieldsV1{Raw: fieldsV1Data2},
				},
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "tekton.dev/v1",
					Kind:       "TaskRun",
					Name:       "build-taskrun-abc123",
					UID:        "abc123-def456-ghi789",
				},
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "pipeline-sa",
			RestartPolicy:      corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{Name: "tekton-internal-workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "tekton-internal-home", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "tekton-internal-results", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "tekton-internal-steps", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "tekton-internal-scripts", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "tekton-internal-downward", VolumeSource: corev1.VolumeSource{DownwardAPI: &corev1.DownwardAPIVolumeSource{}}},
			},
			InitContainers: []corev1.Container{
				{
					Name:    "prepare",
					Image:   "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/entrypoint:v0.50.0",
					Command: []string{"/ko-app/entrypoint", "init", "/ko-app/entrypoint", "/tekton/bin/entrypoint"},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "tekton-internal-bin", MountPath: "/tekton/bin"},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "step-clone",
					Image:   "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init:v0.50.0",
					Command: []string{"/tekton/bin/entrypoint"},
					Args:    []string{"-wait_file", "/tekton/downward/ready", "-post_file", "/tekton/run/0/out", "-termination_path", "/tekton/termination", "-step_metadata_dir", "/tekton/run/0/status", "-entrypoint", "/ko-app/git-init", "--"},
					Env: []corev1.EnvVar{
						{Name: "HOME", Value: "/tekton/home"},
						{Name: "PARAM_URL", Value: "https://github.com/tektoncd/pipeline"},
						{Name: "PARAM_REVISION", Value: "main"},
						{Name: "PARAM_DEPTH", Value: "1"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "tekton-internal-workspace", MountPath: "/workspace"},
						{Name: "tekton-internal-home", MountPath: "/tekton/home"},
						{Name: "tekton-internal-results", MountPath: "/tekton/results"},
					},
				},
				{
					Name:    "step-build",
					Image:   "golang:1.21",
					Command: []string{"/tekton/bin/entrypoint"},
					Args:    []string{"-wait_file", "/tekton/run/0/out", "-post_file", "/tekton/run/1/out", "-termination_path", "/tekton/termination", "-step_metadata_dir", "/tekton/run/1/status", "-entrypoint", "go", "--", "build", "-o", "/workspace/output", "./cmd/..."},
					Env: []corev1.EnvVar{
						{Name: "HOME", Value: "/tekton/home"},
						{Name: "GOOS", Value: "linux"},
						{Name: "GOARCH", Value: "amd64"},
						{Name: "CGO_ENABLED", Value: "0"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "tekton-internal-workspace", MountPath: "/workspace"},
						{Name: "tekton-internal-home", MountPath: "/tekton/home"},
					},
				},
				{
					Name:    "step-test",
					Image:   "golang:1.21",
					Command: []string{"/tekton/bin/entrypoint"},
					Args:    []string{"-wait_file", "/tekton/run/1/out", "-post_file", "/tekton/run/2/out", "-termination_path", "/tekton/termination", "-step_metadata_dir", "/tekton/run/2/status", "-entrypoint", "go", "--", "test", "-v", "./..."},
					Env: []corev1.EnvVar{
						{Name: "HOME", Value: "/tekton/home"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "tekton-internal-workspace", MountPath: "/workspace"},
						{Name: "tekton-internal-home", MountPath: "/tekton/home"},
						{Name: "tekton-internal-results", MountPath: "/tekton/results"},
					},
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
				{Type: corev1.PodReady, Status: corev1.ConditionFalse, Reason: "PodCompleted"},
				{Type: corev1.ContainersReady, Status: corev1.ConditionFalse, Reason: "PodCompleted"},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "prepare",
					Ready: true,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"},
					},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "step-clone",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed", Message: `{"key":"commit","value":"abc123"}`},
					},
					ImageID: "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init@sha256:abc123",
				},
				{
					Name:  "step-build",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"},
					},
					ImageID: "docker.io/library/golang@sha256:def456",
				},
				{
					Name:  "step-test",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed", Message: `{"key":"coverage","value":"85%"}`},
					},
					ImageID: "docker.io/library/golang@sha256:def456",
				},
			},
		},
	}
}
