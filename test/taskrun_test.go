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
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/test/parse"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/pod"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestTaskRunFailure(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := "failing-taskrun"

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: failing-task
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
`, namespace))
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: failing-task
`, taskRunName, namespace))
	if _, err := c.TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
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
	// This should always contain a commit truncated to ~7 characters (based on knative.dev/pkg/changeset)
	commitIDRegexp := regexp.MustCompile(`^[a-f0-9]{7}$`)
	if !ok || !commitIDRegexp.MatchString(releaseAnnotation) {
		t.Fatalf("expected Taskrun to be annotated with %s=devel or with nightly release tag, got %s=%s", pod.ReleaseAnnotation, pod.ReleaseAnnotation, releaseAnnotation)
	}
}

func TestTaskRunStatus(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := "status-taskrun"

	fqImageName := getTestImage(busyboxImage)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: status-task
  namespace: %s
spec:
  steps:
  - image: %s
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
`, namespace, fqImageName))
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: status-task
`, taskRunName, namespace))
	if _, err := c.TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
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

// TestTaskRunStatusSpecificationsAltered tests that a running TaskRun doesn't fail with a validation error
// when its status is altered/corrupted.
func TestTaskRunStatusSpecificationsAltered(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	trName := "taskrun-with-status-corrupted-while-running"
	tName := "valid-task"
	t.Logf("Creating Task and TaskRun %s in namespace %s", trName, namespace)

	task := parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  params:
  - name: foo
  steps:
  - name: echo
    image: ubuntu
    script: |
      #!/usr/bin/env bash
      # Sleep for 10s
      sleep 10
      echo $(params.foo)
`, tName))
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}

	taskrun := parse.MustParseTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  params:
  - name: foo
    value: bar
  taskRef:
    name: %s
`, trName, tName))

	_, err := c.TaskRunClient.Create(ctx, taskrun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create taskRun `%s`: %s", trName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
	if err := WaitForTaskRunState(ctx, c, trName, Running(trName), "TaskRunRunning"); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", trName, err)
	}

	// Get the updated TaskRun
	reconciledRun, err := c.TaskRunClient.Get(ctx, trName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting updated TaskRun: %v", err)
	}

	// interjecting taskSpec in the status and set it to something invalid to verify that the validation does not catch it
	// in real world, the status is considered as a source of truth and not expected to have altered in anyways
	// this is to test no validation done on the specifications after from the status
	invalidSpecs := &v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{
			Script: "invalidSpec without any container image",
		}},
	}
	reconciledRun.Status.TaskSpec = invalidSpecs

	if _, err := c.TaskRunClient.UpdateStatus(ctx, reconciledRun, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to update taskRun `%s`: %s", trName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess"); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", trName, err)
	}

	r, err := c.TaskRunClient.Get(ctx, trName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting updated TaskRun: %v", err)
	}

	if d := cmp.Diff(r.Status.TaskSpec, invalidSpecs); d != "" {
		t.Fatalf("the specifications in the status does not match, -got, +want: %v", d)
	}
}
