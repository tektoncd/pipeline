// +build e2e

/*
Copyright 2018 The Knative Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTaskRunFailure(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	taskRunName := "failing-taskrun"

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := tb.Task("failing-task", namespace, tb.TaskSpec(
		tb.Step("hello", "busybox",
			tb.Command("/bin/sh"), tb.Args("-c", "echo hello"),
		),
		tb.Step("exit", "busybox",
			tb.Command("/bin/sh"), tb.Args("-c", "exit 1"),
		),
		tb.Step("world", "busybox",
			tb.Command("/bin/sh"), tb.Args("-c", "sleep 30s"),
		),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := tb.TaskRun(taskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef("failing-task"),
	))
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	expectedStepState := []v1alpha1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name: "exit",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name: "hello",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name: "world",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name: "nop",
	}}
	ignoreFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}
