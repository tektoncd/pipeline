// +build e2e,windows_e2e

/*
Copyright 2021 The Tekton Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

var (
	ignoreTerminatedFields = cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields       = cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID")
)

func TestWindows(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := "windows-taskrun"
	t.Logf("Creating TaskRun in namespace %s", namespace)

	taskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: taskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Container: corev1.Container{
						Image:   "mcr.microsoft.com/windows/nanoserver:1809",
						Command: []string{"cmd.exe"},
						Args:    []string{"/c", "echo hello"},
					},
				}},
			},
			PodTemplate: &v1beta1.PodTemplate{
				NodeSelector: map[string]string{"kubernetes.io/os": "windows"},
			},
		},
	}
	if _, err := c.TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceeded"); err != nil {
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
	if d := cmp.Diff(expectedStepState, taskrun.Status.Steps, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

func TestWindowsFailure(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := "failing-windows-taskrun"
	t.Logf("Creating TaskRun in namespace %s", namespace)

	taskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: taskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Container: corev1.Container{
						Image:   "mcr.microsoft.com/windows/nanoserver:1809",
						Command: []string{"cmd.exe"},
						Args:    []string{"/c", "echo hello"},
					}}, {
					Container: corev1.Container{
						Image:   "mcr.microsoft.com/windows/nanoserver:1809",
						Command: []string{"cmd.exe"},
						Args:    []string{"/c", "exit 1"},
					}}, {
					Container: corev1.Container{
						Image:   "mcr.microsoft.com/windows/nanoserver:1809",
						Command: []string{"cmd.exe"},
						Args:    []string{"/c", "ping localhost -n 5"},
					}},
				},
			},
			PodTemplate: &v1beta1.PodTemplate{
				NodeSelector: map[string]string{"kubernetes.io/os": "windows"},
			},
		},
	}
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
	if d := cmp.Diff(expectedStepState, taskrun.Status.Steps, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}
