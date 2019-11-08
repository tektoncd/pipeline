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

package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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

func TestPipelineDescribe_invalid_namespace(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	_, err := test.ExecuteCommand(pipeline, "desc", "bar", "-n", "invalid")
	if err == nil {
		t.Errorf("Error expected here for invalid namespace")
	}
	expected := "namespaces \"invalid\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestPipelineDescribe_invalid_pipeline(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}
	cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	_, err := test.ExecuteCommand(pipeline, "desc", "bar")
	if err == nil {
		t.Errorf("Error expected here")
	}
	expected := "pipelines.tekton.dev \"bar\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestPipelinesDescribe_empty(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-5*time.Minute)),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{},
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}
	pipeline := Command(p)

	got, err := test.ExecuteCommand(pipeline, "desc", "-n", "ns", "pipeline")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"Name:   pipeline",
		"\nResources",
		"No resources\n",
		"Params",
		"No params\n",
		"Tasks",
		"No tasks\n",
		"Pipelineruns",
		"No pipelineruns\n",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}

func TestPipelinesDescribe_with_run(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-5*time.Minute)),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun("pipeline-run-1", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run starts now
					tb.PipelineRunStartTime(clock.Now()),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
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
	pipeline := Command(p)

	// -5 : pipeline created
	//  0 : pipeline run - 1 started
	// 10 : pipeline run - 1 finished
	// 15 : <<< now run pipeline ls << - advance clock to this point

	clock.Advance(15 * time.Minute)
	got, err := test.ExecuteCommand(pipeline, "desc", "-n", "ns", "pipeline")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"Name:   pipeline",
		"\nResources",
		"No resources\n",
		"Params",
		"No params\n",
		"Tasks",
		"No tasks\n",
		"Pipelineruns",
		"NAME             STARTED          DURATION     STATUS",
		"pipeline-run-1   15 minutes ago   10 minutes   Succeeded\n",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}

func TestPipelinesDescribe_with_task_run(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-5*time.Minute)),
				tb.PipelineSpec(
					tb.PipelineTask("task", "taskref",
						tb.RunAfter("one", "two")),
				),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun("pipeline-run-1", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run starts now
					tb.PipelineRunStartTime(clock.Now()),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
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
	pipeline := Command(p)

	// -5 : pipeline created
	//  0 : pipeline run - 1 started
	// 10 : pipeline run - 1 finished
	// 15 : <<< now run pipeline ls << - advance clock to this point

	clock.Advance(15 * time.Minute)
	got, err := test.ExecuteCommand(pipeline, "desc", "-n", "ns", "pipeline")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"Name:   pipeline",
		"\nResources",
		"No resources\n",
		"Params",
		"No params\n",
		"Tasks",
		"NAME   TASKREF   RUNAFTER",
		"task   taskref   [one two]\n",
		"Pipelineruns",
		"NAME             STARTED          DURATION     STATUS",
		"pipeline-run-1   15 minutes ago   10 minutes   Succeeded\n",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}

func TestPipelinesDescribe_with_resource_param_task_run(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-5*time.Minute)),
				tb.PipelineSpec(
					tb.PipelineTask("task", "taskref",
						tb.RunAfter("one", "two"),
					),
					tb.PipelineDeclaredResource("name", v1alpha1.PipelineResourceTypeGit),
					tb.PipelineParamSpec("pipeline-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent")),
				),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun("pipeline-run-1", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run starts now
					tb.PipelineRunStartTime(clock.Now()),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
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
	pipeline := Command(p)

	// -5 : pipeline created
	//  0 : pipeline run - 1 started
	// 10 : pipeline run - 1 finished
	// 15 : <<< now run pipeline ls << - advance clock to this point

	clock.Advance(15 * time.Minute)
	got, err := test.ExecuteCommand(pipeline, "desc", "-n", "ns", "pipeline")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"Name:   pipeline",
		"\nResources",
		"NAME   TYPE",
		"name   git\n",
		"Params",
		"NAME             TYPE     DEFAULT VALUE",
		"pipeline-param   string   somethingdifferent\n",
		"Tasks",
		"NAME   TASKREF   RUNAFTER",
		"task   taskref   [one two]\n",
		"Pipelineruns",
		"NAME             STARTED          DURATION     STATUS",
		"pipeline-run-1   15 minutes ago   10 minutes   Succeeded\n",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}

func TestPipelinesDescribe_with_multiple_resource_param_task_run(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-5*time.Minute)),
				tb.PipelineSpec(
					tb.PipelineTask("task", "taskref",
						tb.RunAfter("one", "two"),
					),
					tb.PipelineDeclaredResource("name", v1alpha1.PipelineResourceTypeGit),
					tb.PipelineDeclaredResource("code", v1alpha1.PipelineResourceTypeGit),
					tb.PipelineDeclaredResource("code-image", v1alpha1.PipelineResourceTypeImage),
					tb.PipelineDeclaredResource("artifact-image", v1alpha1.PipelineResourceTypeImage),
					tb.PipelineDeclaredResource("repo", v1alpha1.PipelineResourceTypeGit),
					tb.PipelineParamSpec("pipeline-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent")),
					tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeArray, tb.ParamSpecDefault("booms", "booms", "booms")),
					tb.PipelineParamSpec("pipeline-param2", v1alpha1.ParamTypeString),
					tb.PipelineParamSpec("rev-param2", v1alpha1.ParamTypeArray),
				),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun("pipeline-run-1", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run starts now
					tb.PipelineRunStartTime(clock.Now()),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
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
	pipeline := Command(p)

	// -5 : pipeline created
	//  0 : pipeline run - 1 started
	// 10 : pipeline run - 1 finished
	// 15 : <<< now run pipeline ls << - advance clock to this point

	clock.Advance(15 * time.Minute)
	got, err := test.ExecuteCommand(pipeline, "desc", "-n", "ns", "pipeline")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"Name:   pipeline",
		"\nResources",
		"NAME             TYPE",
		"code             git",
		"name             git",
		"repo             git",
		"artifact-image   image",
		"code-image       image\n",
		"Params",
		"NAME              TYPE     DEFAULT VALUE",
		"pipeline-param    string   somethingdifferent",
		"rev-param         array    [booms booms booms]",
		"pipeline-param2   string   ",
		"rev-param2        array    \n",
		"Tasks",
		"NAME   TASKREF   RUNAFTER",
		"task   taskref   [one two]\n",
		"Pipelineruns",
		"NAME             STARTED          DURATION     STATUS",
		"pipeline-run-1   15 minutes ago   10 minutes   Succeeded\n",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}
