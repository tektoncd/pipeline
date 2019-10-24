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

package taskrun

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskRunDescribe_invalid_namespace(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
		},
	})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	taskrun := Command(p)
	_, err := test.ExecuteCommand(taskrun, "desc", "bar", "-n", "invalid")
	if err == nil {
		t.Errorf("Expected error but did not get one")
	}
	expected := "namespaces \"invalid\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestTaskRunDescribe_not_found(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	taskrun := Command(p)
	_, err := test.ExecuteCommand(taskrun, "desc", "bar", "-n", "ns")
	if err == nil {
		t.Errorf("Expected error but did not get one")
	}
	expected := "failed to find taskrun \"bar\""
	test.AssertOutput(t, expected, err.Error())
}

func TestTaskRunDescribe_empty_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunLabel("tekton.dev/task", "t1"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("t1")),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:        tr-1
Namespace:   ns
Task Ref:    t1

Status
STARTED    DURATION    STATUS
---        ---         Succeeded

Input Resources
No resources

Output Resources
No resources

Params
No params

Steps
No steps
`

	test.AssertOutput(t, expected, actual)
}

func TestTaskRunDescribe_only_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clockwork.NewFakeClock().Now().Add(20*time.Second)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName("step1"),
					tb.StateTerminated(0),
				),
				tb.StepState(
					cb.StepName("step2"),
					tb.StateTerminated(0),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("t1"),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input", "param")),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input2", "param2")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("git", tb.TaskResourceBindingRef("git"))),
				tb.TaskRunInputs(tb.TaskRunInputsResource("image-input", tb.TaskResourceBindingRef("image"))),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output", tb.TaskResourceBindingRef("image"))),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output2", tb.TaskResourceBindingRef("image"))),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:        tr-1
Namespace:   ns
Task Ref:    t1

Status
STARTED         DURATION    STATUS
9 minutes ago   ---         Succeeded

Input Resources
NAME          RESOURCE REF
git           git
image-input   image

Output Resources
NAME            RESOURCE REF
image-output    image
image-output2   image

Params
NAME     VALUE
input    param
input2   param2

Steps
NAME
step1
step2
`

	test.AssertOutput(t, expected, actual)
}

func TestTaskRunDescribe_failed(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Status:  corev1.ConditionFalse,
					Reason:  resources.ReasonFailed,
					Message: "Testing tr failed",
				}),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("t1"),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:        tr-1
Namespace:   ns
Task Ref:    t1

Status
STARTED         DURATION    STATUS
8 minutes ago   3 minutes   Failed

Message
Testing tr failed

Input Resources
No resources

Output Resources
No resources

Params
No params

Steps
No steps
`

	test.AssertOutput(t, expected, actual)
}
