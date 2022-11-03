//go:build e2e
// +build e2e

/*
Copyright 2019 The Tekton Authors

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

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/test/parse"
	jsonpatch "gomodules.xyz/jsonpatch/v2"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/pod"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestTaskRunFailure(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var c *clients
	var namespace string

	c, namespace = setup(ctx, t)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := helpers.ObjectNameForTest(t)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'exit 1']
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'sleep 30s']
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1beta1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, task.Name))
	if _, err := c.V1beta1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed", v1beta1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1beta1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	if spireEnabled, _ := hasAnyGate(ctx, spireFeatureGates, t, c, namespace); spireEnabled {
		spireShouldFailTaskRunResultsVerify(taskrun, t)
	}

	expectedStepState := []v1beta1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name:          "unnamed-1",
		ContainerName: "step-unnamed-1",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name:          "unnamed-2",
		ContainerName: "step-unnamed-2",
	}}
	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}

	releaseAnnotation, ok := taskrun.Annotations[pod.ReleaseAnnotation]
	// This should always contain a commit truncated to 7 characters, possibly with "-dirty" suffix (based on knative.dev/pkg/changeset)
	commitIDRegexp := regexp.MustCompile(`^[a-f0-9]{7}(-dirty)?$`)
	if !ok || !commitIDRegexp.MatchString(releaseAnnotation) {
		t.Fatalf("expected Taskrun to be annotated with %s=devel or with nightly release tag, got %s=%s", pod.ReleaseAnnotation, pod.ReleaseAnnotation, releaseAnnotation)
	}
}

func TestTaskRunStatus(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var c *clients
	var namespace string

	c, namespace = setup(ctx, t)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := helpers.ObjectNameForTest(t)

	fqImageName := getTestImage(busyboxImage)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: %s
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
`, helpers.ObjectNameForTest(t), namespace, fqImageName))
	if _, err := c.V1beta1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, task.Name))
	if _, err := c.V1beta1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1beta1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1beta1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	if spireEnabled, _ := hasAnyGate(ctx, spireFeatureGates, t, c, namespace); spireEnabled {
		spireShouldPassTaskRunResultsVerify(taskrun, t)
	}

	expectedStepState := []v1beta1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}}

	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
	// Note(chmouel): Sometime we have docker-pullable:// or docker.io/library as prefix, so let only compare the suffix
	if !strings.HasSuffix(taskrun.Status.Steps[0].ImageID, fqImageName) {
		t.Fatalf("`ImageID: %s` does not end with `%s`", taskrun.Status.Steps[0].ImageID, fqImageName)
	}

	if d := cmp.Diff(taskrun.Status.TaskSpec, &task.Spec); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}

// TestTaskRunModification is an exclusive test for SPIRE integration into taskrun.
// The test starts a taskrun which has a sleep. While the taskrun is "sleep"ing,
// the text modifies the taskrun results.
// This change is caught by the taskrun reconciler when it tries to verify the results.
func TestTaskRunModification(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var c *clients
	var namespace string

	c, namespace = setup(ctx, t, requireAnyGate(spireFeatureGates))

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := "non-falsifiable-provenance"

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: non-falsifiable
  namespace: %s
spec:
  steps:
  - image: ubuntu
    script: |
      #!/usr/bin/env bash
      sleep 20
      printf "hello" > "$(results.foo.path)"
       printf "world" > "$(results.bar.path)"
    results:
    - name: foo
    - name: bar
`, namespace))
	if _, err := c.V1beta1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: non-falsifiable
`, taskRunName, namespace))
	if _, err := c.V1beta1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to be in running state", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, Running(taskRunName), "TaskRunRunning", v1beta1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to start running: %s", err)
	}

	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "replace",
		Path:      "/status/taskSpec/steps/0/image",
		Value:     "not-ubuntu",
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		t.Fatalf("failed to marshal patch bytes in order to stop")
	}
	t.Logf("Patching TaskRun %s in namespace %s mid run for spire to catch the un-authorized changed", taskRunName, namespace)
	if _, err := c.V1beta1TaskRunClient.Patch(ctx, taskRunName, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, "status"); err != nil {
		t.Fatalf("Failed to patch taskrun `%s`: %s", taskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to succeed", taskRunName, namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed", v1beta1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1beta1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	if spireEnabled, _ := hasAnyGate(ctx, spireFeatureGates, t, c, namespace); spireEnabled {
		spireShouldFailTaskRunResultsVerify(taskrun, t)
	}

	expectedStepState := []v1beta1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}}

	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}
