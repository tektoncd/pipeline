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

func TestPipelineRunDescribe_invalid_namespace(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipelinerun := Command(p)
	_, err := test.ExecuteCommand(pipelinerun, "desc", "bar", "-n", "invalid")
	if err == nil {
		t.Errorf("Expected error for invalid namespace")
	}
	expected := "namespaces \"invalid\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestPipelineRunDescribe_not_found(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipelinerun := Command(p)
	_, err := test.ExecuteCommand(pipelinerun, "desc", "bar", "-n", "ns")
	if err == nil {
		t.Errorf("Expected error, did not get any")
	}
	expected := "failed to find pipelinerun \"bar\""
	test.AssertOutput(t, expected, err.Error())
}

func TestPipelineRunDescribe_only_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus("tr-1", &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: "t-1",
						Status:           &trs[0].Status,
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
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

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

	test.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_multiple_taskrun_ordering(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			),
		),
		tb.TaskRun("tr-2", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(5*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(9*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus("tr-1", &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: "t-1",
						Status:           &trs[0].Status,
					}),
					tb.PipelineRunTaskRunsStatus("tr-2", &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: "t-2",
						Status:           &trs[1].Status,
					}),
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					tb.PipelineRunStartTime(clock.Now()),
					cb.PipelineRunCompletionTime(clock.Now().Add(15*time.Minute)),
				),
			),
		},
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}
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
STARTED          DURATION     STATUS
10 minutes ago   15 minutes   Succeeded

Resources
No resources

Params
No params

Taskruns
NAME   TASK NAME   STARTED         DURATION    STATUS
tr-2   t-2         5 minutes ago   4 minutes   Succeeded
tr-1   t-1         8 minutes ago   3 minutes   Succeeded
`
	test.AssertOutput(t, expected, actual)

}

func TestPipelineRunDescribe_failed(t *testing.T) {
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
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline",
					tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus("tr-1", &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: "t-1",
						Status:           &trs[0].Status,
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
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:                           pipeline-run
Namespace:                      ns
Pipeline Ref:                   pipeline
Service Account (deprecated):   test-sa

Status
STARTED          DURATION    STATUS
10 minutes ago   5 minutes   Failed(Resource not found)

Message
Resource test-resource not found in the pipelinerun (Testing tr failed)

Resources
No resources

Params
No params

Taskruns
NAME   TASK NAME   STARTED         DURATION    STATUS
tr-1   t-1         8 minutes ago   3 minutes   Failed
`

	test.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_failed_withoutTRCondition(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline",
					tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus("tr-1", &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: "t-1",
						Status:           &trs[0].Status,
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
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:                           pipeline-run
Namespace:                      ns
Pipeline Ref:                   pipeline
Service Account (deprecated):   test-sa

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
tr-1   t-1         8 minutes ago   3 minutes   ---
`

	test.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_failed_withoutPRCondition(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline",
					tb.PipelineRunDeprecatedServiceAccountName("test-sa", ""),
				),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus("tr-1", &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: "t-1",
						Status:           &trs[0].Status,
					}),
					tb.PipelineRunStartTime(clock.Now()),
					cb.PipelineRunCompletionTime(clock.Now().Add(5*time.Minute)),
				),
			),
		},
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

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
10 minutes ago   5 minutes   ---

Resources
No resources

Params
No params

Taskruns
NAME   TASK NAME   STARTED         DURATION    STATUS
tr-1   t-1         8 minutes ago   3 minutes   ---
`

	test.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_with_resources_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline",
					tb.PipelineRunDeprecatedServiceAccountName("test-sa", "test-sa-deprecated"),
					tb.PipelineRunParam("test-param", "param-value"),
					tb.PipelineRunResourceBinding("test-resource",
						tb.PipelineResourceBindingRef("test-resource-ref"),
					),
				),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus("tr-1", &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: "t-1",
						Status:           &trs[0].Status,
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
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:                           pipeline-run
Namespace:                      ns
Pipeline Ref:                   pipeline
Service Account (deprecated):   test-sa-deprecated
Service Account:                test-sa

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

	test.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_without_start_time(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(),
			),
		},
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

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

	test.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_without_pipelineref(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunStatus(),
			),
		},
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:        pipeline-run
Namespace:   ns

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

	test.AssertOutput(t, expected, actual)
}

func TestPipelineRunDescribe_no_resourceref(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("pipeline-run", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline",
					tb.PipelineRunDeprecatedServiceAccountName("test-sa", "test-sa-deprecated"),
					tb.PipelineRunParam("test-param", "param-value"),
					tb.PipelineRunResourceBinding("test-resource"),
				),
				tb.PipelineRunStatus(
					tb.PipelineRunTaskRunsStatus("tr-1", &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: "t-1",
						Status:           &trs[0].Status,
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
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	pipelinerun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(pipelinerun, "desc", "pipeline-run", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := `Name:                           pipeline-run
Namespace:                      ns
Pipeline Ref:                   pipeline
Service Account (deprecated):   test-sa-deprecated
Service Account:                test-sa

Status
STARTED          DURATION    STATUS
10 minutes ago   5 minutes   Succeeded

Resources
NAME            RESOURCE REF
test-resource   

Params
NAME         VALUE
test-param   param-value

Taskruns
NAME   TASK NAME   STARTED         DURATION    STATUS
tr-1   t-1         8 minutes ago   3 minutes   Succeeded
`

	test.AssertOutput(t, expected, actual)
}
