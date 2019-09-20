// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package task

import (
	"errors"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	k8stest "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
)

func TestTaskDescribe_Empty(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}

	task := Command(p)
	_, err := test.ExecuteCommand(task, "desc", "bar")
	if err == nil {
		t.Errorf("Error expected here")
	}
	expected := "tasks.tekton.dev \"bar\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestTaskDescribe_OnlyName(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Tasks: []*v1alpha1.Task{
			tb.Task("task-1", "ns"),
			tb.Task("task", "ns-2"),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline}
	task := Command(p)
	out, err := test.ExecuteCommand(task, "desc", "task-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:        task-1
Namespace:   ns

Input Resources
No input resources

Output Resources
No output resources

Params
No params

Steps
No steps

Taskruns
No taskruns
`
	test.AssertOutput(t, expected, out)
}

func TestTaskDescribe_OnlyNameDiffNameSpace(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Tasks: []*v1alpha1.Task{
			tb.Task("task-1", "ns"),
			tb.Task("task", "ns-2"),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline}
	p.SetNamespace("ns")
	task := Command(p)
	out, err := test.ExecuteCommand(task, "desc", "task", "-n", "ns-2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:        task
Namespace:   ns-2

Input Resources
No input resources

Output Resources
No output resources

Params
No params

Steps
No steps

Taskruns
No taskruns
`
	test.AssertOutput(t, expected, out)
}

func TestTaskDescribe_OnlyNameParams(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Tasks: []*v1alpha1.Task{
			tb.Task("task-1", "ns",
				tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsParamSpec("myarg", v1alpha1.ParamTypeString),
						tb.InputsParamSpec("myprint", v1alpha1.ParamTypeString),
					),
				),
			),
			tb.Task("task", "ns-2"),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline}
	task := Command(p)
	out, err := test.ExecuteCommand(task, "desc", "task-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:        task-1
Namespace:   ns

Input Resources
No input resources

Output Resources
No output resources

Params
NAME      TYPE
myarg     string
myprint   string

Steps
No steps

Taskruns
No taskruns
`
	test.AssertOutput(t, expected, out)
}

func TestTaskDescribe_Full(t *testing.T) {
	clock := clockwork.NewFakeClock()
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Tasks: []*v1alpha1.Task{
			tb.Task("task-1", "ns",
				tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("my-repo", v1alpha1.PipelineResourceTypeGit),
						tb.InputsResource("my-image", v1alpha1.PipelineResourceTypeImage),
						tb.InputsParamSpec("myarg", v1alpha1.ParamTypeString),
						tb.InputsParamSpec("print", v1alpha1.ParamTypeString),
					),
					tb.TaskOutputs(
						tb.OutputsResource("code-image", v1alpha1.PipelineResourceTypeImage),
					),
					tb.Step("hello", "busybox"),
					tb.Step("exit", "busybox"),
				),
			),
		},
		TaskRuns: []*v1alpha1.TaskRun{
			tb.TaskRun("tr-1", "ns",
				tb.TaskRunLabel("tekton.dev/task", "task-1"),
				tb.TaskRunSpec(tb.TaskRunTaskRef("task-1")),
				tb.TaskRunStatus(
					tb.StatusCondition(apis.Condition{
						Status: corev1.ConditionFalse,
						Reason: resources.ReasonFailed,
					}),
					tb.TaskRunStartTime(clock.Now()),
					cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				),
			),
			tb.TaskRun("tr-2", "ns",
				tb.TaskRunLabel("tekton.dev/task", "task-1"),
				tb.TaskRunSpec(tb.TaskRunTaskRef("task-1")),
				tb.TaskRunStatus(
					tb.StatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					tb.TaskRunStartTime(clock.Now().Add(10*time.Minute)),
					cb.TaskRunCompletionTime(clock.Now().Add(17*time.Minute)),
				),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}
	task := Command(p)
	clock.Advance(20 * time.Minute)
	out, err := test.ExecuteCommand(task, "desc", "task-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:        task-1
Namespace:   ns

Input Resources
NAME       TYPE
my-repo    git
my-image   image

Output Resources
NAME         TYPE
code-image   image

Params
NAME    TYPE
myarg   string
print   string

Steps
NAME
hello
exit

Taskruns
NAME   STARTED          DURATION    STATUS
tr-1   20 minutes ago   5 minutes   Failed
tr-2   10 minutes ago   7 minutes   Succeeded

`
	test.AssertOutput(t, expected, out)
}

func TestTaskDescribe_PipelineRunError(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Tasks: []*v1alpha1.Task{
			tb.Task("task-1", "ns"),
			tb.Task("task", "ns-2"),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	cs.Pipeline.PrependReactor("list", "taskruns", func(action k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fake list taskrun error")
	})

	task := Command(p)
	out, err := test.ExecuteCommand(task, "desc", "task-1")
	if err == nil {
		t.Errorf("Expected error got nil")
	}
	expected := "failed to get taskruns for task task-1 \nError: fake list taskrun error\n"
	test.AssertOutput(t, expected, out)
	test.AssertOutput(t, "fake list taskrun error", err.Error())
}
