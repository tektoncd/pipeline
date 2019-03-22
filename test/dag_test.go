// +build e2e

/*
Copyright 2018 Knative Authors LLC
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
	"sort"
	"strings"
	"testing"
	"time"

	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDAGPipelineRun creates a graph of arbitrary Tasks, then looks at the corresponding
// TaskRun start times to ensure they were run in the order intended, which is:
//                               |
//                        pipeline-task-1
//                       /               \
//   pipeline-task-2-parallel-1    pipeline-task-2-parallel-2
//                       \                /
//                        pipeline-task-3
//                               |
//                        pipeline-task-4
func TestDAGPipelineRun(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	// Create the Task that echoes text
	echoTask := tb.Task("echo-task", namespace, tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("repo", v1alpha1.PipelineResourceTypeGit),
			tb.InputsParam("text", tb.ParamDescription("The text that should be echoed")),
		),
		tb.TaskOutputs(tb.OutputsResource("repo", v1alpha1.PipelineResourceTypeGit)),
		tb.Step("echo-text", "busybox", tb.Command("echo"), tb.Args("${inputs.params.text}")),
	))
	if _, err := c.TaskClient.Create(echoTask); err != nil {
		t.Fatalf("Failed to create echo Task: %s", err)
	}

	// Create the repo PipelineResource (doesn't really matter which repo we use)
	repoResource := tb.PipelineResource("repo", namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/githubtraining/example-basic"),
	))
	if _, err := c.PipelineResourceClient.Create(repoResource); err != nil {
		t.Fatalf("Failed to create simple repo PipelineResource: %s", err)
	}

	// Intentionally declaring Tasks in a mixed up order to ensure the order
	// of execution isn't at all dependent on the order they are declared in
	pipeline := tb.Pipeline("dag-pipeline", namespace, tb.PipelineSpec(
		tb.PipelineDeclaredResource("repo", "repo"),
		tb.PipelineTask("pipeline-task-3", "echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-2-parallel-1", "pipeline-task-2-parallel-2")),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "wow"),
		),
		tb.PipelineTask("pipeline-task-2-parallel-2", "echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-1")), tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "such parallel"),
		),
		tb.PipelineTask("pipeline-task-4", "echo-task",
			tb.RunAfter("pipeline-task-3"),
			tb.PipelineTaskInputResource("repo", "repo"),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "very cloud native"),
		),
		tb.PipelineTask("pipeline-task-2-parallel-1", "echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-1")),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "much graph"),
		),
		tb.PipelineTask("pipeline-task-1", "echo-task",
			tb.PipelineTaskInputResource("repo", "repo"),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "how to ci/cd?"),
		),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create dag-pipeline: %s", err)
	}
	pipelineRun := tb.PipelineRun("dag-pipeline-run", namespace, tb.PipelineRunSpec("dag-pipeline",
		tb.PipelineRunResourceBinding("repo", tb.PipelineResourceBindingRef("repo")),
	))
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create dag-pipeline-run PipelineRun: %s", err)
	}
	t.Logf("Waiting for DAG pipeline to complete")
	if err := WaitForPipelineRunState(c, "dag-pipeline-run", pipelineRunTimeout, PipelineRunSucceed("dag-pipeline-run"), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %s", err)
	}

	t.Logf("Verifying order of execution")
	times := getTaskStartTimes(t, c.TaskRunClient)
	vefifyExpectedOrder(t, times)
}

type runTime struct {
	name string
	t    time.Time
}

type runTimes []runTime

func (f runTimes) Len() int {
	return len(f)
}

func (f runTimes) Less(i, j int) bool {
	return f[i].t.Before(f[j].t)
}

func (f runTimes) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func getTaskStartTimes(t *testing.T, c clientset.TaskRunInterface) runTimes {
	taskRuns, err := c.List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't get TaskRuns (so that we could check when they executed): %v", err)
	}
	times := runTimes{}
	for _, t := range taskRuns.Items {
		times = append(times, runTime{
			name: t.Name,
			t:    t.Status.StartTime.Time,
		})
	}
	return times
}

func vefifyExpectedOrder(t *testing.T, times runTimes) {
	if len(times) != 5 {
		t.Fatalf("Expected 5 Taskruns to have executed but only got start times for %d Taskruns", len(times))
	}

	sort.Sort(times)

	if !strings.HasPrefix(times[0].name, "dag-pipeline-run-pipeline-task-1") {
		t.Errorf("Expected first task to execute first, but %q was first", times[0].name)
	}
	if !strings.HasPrefix(times[1].name, "dag-pipeline-run-pipeline-task-2") {
		t.Errorf("Expected parallel tasks to run second & third, but %q was second", times[1].name)
	}
	if !strings.HasPrefix(times[2].name, "dag-pipeline-run-pipeline-task-2") {
		t.Errorf("Expected parallel tasks to run second & third, but %q was third", times[2].name)
	}
	if !strings.HasPrefix(times[3].name, "dag-pipeline-run-pipeline-task-3") {
		t.Errorf("Expected third task to execute third, but %q was third", times[3].name)
	}
	if !strings.HasPrefix(times[4].name, "dag-pipeline-run-pipeline-task-4") {
		t.Errorf("Expected fourth task to execute fourth, but %q was fourth", times[4].name)
	}

	// Check that the two tasks that can run in parallel did
	parallelDiff := times[2].t.Sub(times[1].t)
	if parallelDiff > (time.Second * 5) {
		t.Errorf("Expected parallel tasks to execute more or less at the same time, but they were %v apart", parallelDiff)
	}
}
