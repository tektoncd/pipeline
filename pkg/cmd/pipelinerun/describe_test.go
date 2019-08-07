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

package pipelinerun

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/cli/pkg/test"
	tu "github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

func TestPipelineRunDescribe_not_found(t *testing.T) {
	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}

	pipelinerun := Command(p)
	_, err := test.ExecuteCommand(pipelinerun, "desc", "bar", "-n", "ns")
	if err == nil {
		t.Errorf("Expected error, did not get any")
	}
	expected := "Failed to find pipelinerun \"bar\""
	tu.AssertOutput(t, expected, err.Error())
}

func TestPipelineRunDescribe_only_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			),
		),
	}

	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus(map[string]*v1alpha1.PipelineRunTaskRunStatus{
						"tr-1": {
							PipelineTaskName: "t-1",
							Status:           &trs[0].Status,
						},
					}),
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					tb.PipelineRunStartTime(clock.Now()),
					cb.PipelineRunCompletionTime(clock.Now().Add(5*time.Minute)),
				),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:           pipeline-run
Namespace:      ns
Pipeline Ref:   pipeline

Status
STARTED          DURATION    STATUS
10 minutes ago   5 minutes   Succeeded

Resources
No resources

Params
No params

Taskruns
NAME   TASK NAME   STARTED         DURATION    STATUS
tr-1   t-1         8 minutes ago   3 minutes   Succeeded
`

	tu.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_failed(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.Condition(apis.Condition{
					Status:  corev1.ConditionFalse,
					Reason:  resources.ReasonFailed,
					Message: "Testing tr failed",
				}),
			),
		),
	}

	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline",
					tb.PipelineRunServiceAccount("test-sa"),
				),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus(map[string]*v1alpha1.PipelineRunTaskRunStatus{
						"tr-1": {
							PipelineTaskName: "t-1",
							Status:           &trs[0].Status,
						},
					}),
					tb.PipelineRunStatusCondition(apis.Condition{
						Status:  corev1.ConditionFalse,
						Reason:  "Resource not found",
						Message: "Resource test-resource not found in the pipelinerun",
					}),
					tb.PipelineRunStartTime(clock.Now()),
					cb.PipelineRunCompletionTime(clock.Now().Add(5*time.Minute)),
				),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:              pipeline-run
Namespace:         ns
Pipeline Ref:      pipeline
Service Account:   test-sa

Status
STARTED          DURATION    STATUS
10 minutes ago   5 minutes   Failed(Resource not found)

Message
Resource test-resource not found in the pipelinerun

Resources
No resources

Params
No params

Taskruns
NAME   TASK NAME   STARTED         DURATION    STATUS
tr-1   t-1         8 minutes ago   3 minutes   Failed
`

	tu.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_with_resources_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			),
		),
	}

	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline",
					tb.PipelineRunServiceAccount("test-sa"),
					tb.PipelineRunParam("test-param", "param-value"),
					tb.PipelineRunResourceBinding("test-resource",
						tb.PipelineResourceBindingRef("test-resource-ref"),
					),
				),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus(map[string]*v1alpha1.PipelineRunTaskRunStatus{
						"tr-1": {
							PipelineTaskName: "t-1",
							Status:           &trs[0].Status,
						},
					}),
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					tb.PipelineRunStartTime(clock.Now()),
					cb.PipelineRunCompletionTime(clock.Now().Add(5*time.Minute)),
				),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:              pipeline-run
Namespace:         ns
Pipeline Ref:      pipeline
Service Account:   test-sa

Status
STARTED          DURATION    STATUS
10 minutes ago   5 minutes   Succeeded

Resources
NAME            RESOURCE REF
test-resource   test-resource-ref

Params
NAME         VALUE
test-param   param-value

Taskruns
NAME   TASK NAME   STARTED         DURATION    STATUS
tr-1   t-1         8 minutes ago   3 minutes   Succeeded
`

	tu.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_without_start_time(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:           pipeline-run
Namespace:      ns
Pipeline Ref:   pipeline

Status
STARTED   DURATION   STATUS
---       ---        ---

Resources
No resources

Params
No params

Taskruns
No taskruns
`

	tu.AssertOutput(t, expected, actual)
}
