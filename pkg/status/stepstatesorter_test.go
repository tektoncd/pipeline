/*
 Copyright 2019 Knative Authors LLC
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

package status

import (
	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestSortTaskRunStepOrder(t *testing.T) {
	task := tb.Task("failing-task", "default", tb.TaskSpec(
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

	taskRunStatusSteps := []v1alpha1.StepState{{
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
		Name: "nop",
	}}

	SortTaskRunStepOrder(taskRunStatusSteps, task.Spec.Steps)
	actualStepOrder := []string{}
	for _, state :=  range(taskRunStatusSteps) {
		actualStepOrder = append(actualStepOrder, state.Name)
	}

	expectedStepOrder := []string{"hello", "exit", "world", "nop"}

	if d := cmp.Diff(actualStepOrder, expectedStepOrder); d != "" {
		t.Errorf("The status steps in TaksRun doesn't match the spec steps in Task, diff: %s", d)
	}
}
