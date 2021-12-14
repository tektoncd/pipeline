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
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/test/parse"

	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

const sleepDuration = 15 * time.Second

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
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	echoTask := parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: echo-task
  namespace: %s
spec:
  resources:
    inputs:
    - name: repo
      type: git
    outputs:
    - name: repo
      type: git
  params:
  - name: text
    type: string
    description: 'The text that should be echoed'
  steps:
  - image: busybox
    script: 'echo $(params["text"])'
  - image: busybox
    # Sleep for N seconds so that we can check that tasks that
    # should be run in parallel have overlap.
    script: 'sleep %d'
  - image: busybox
    script: 'ln -s $(resources.inputs.repo.path) $(resources.outputs.repo.path)'
`, namespace, int(sleepDuration.Seconds())))
	if _, err := c.TaskClient.Create(ctx, echoTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create echo Task: %s", err)
	}

	// Create the repo PipelineResource (doesn't really matter which repo we use)
	repoResource := parse.MustParsePipelineResource(t, `
metadata:
  name: repo
spec:
  type: git
  params:
  - name: Url
    value: https://github.com/githubtraining/example-basic
`)
	if _, err := c.PipelineResourceClient.Create(ctx, repoResource, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create simple repo PipelineResource: %s", err)
	}

	// Intentionally declaring Tasks in a mixed up order to ensure the order
	// of execution isn't at all dependent on the order they are declared in
	pipeline := parse.MustParsePipeline(t, fmt.Sprintf(`
metadata:
  name: dag-pipeline
  namespace: %s
spec:
  resources:
  - name: repo
    type: git
  tasks:
  - name: pipeline-task-3
    params:
    - name: text
      value: wow
    resources:
      inputs:
      - from:
        - pipeline-task-2-parallel-1
        - pipeline-task-2-parallel-2
        name: repo
        resource: repo
      outputs:
      - name: repo
        resource: repo
    taskRef:
      name: echo-task
  - name: pipeline-task-2-parallel-2
    params:
    - name: text
      value: such parallel
    resources:
      inputs:
      - from:
        - pipeline-task-1
        name: repo
        resource: repo
      outputs:
      - name: repo
        resource: repo
    taskRef:
      name: echo-task
  - name: pipeline-task-4
    params:
    - name: text
      value: very cloud native
    resources:
      inputs:
      - name: repo
        resource: repo
      outputs:
      - name: repo
        resource: repo
    runAfter:
    - pipeline-task-3
    taskRef:
      name: echo-task
  - name: pipeline-task-2-parallel-1
    params:
    - name: text
      value: much graph
    resources:
      inputs:
      - from:
        - pipeline-task-1
        name: repo
        resource: repo
      outputs:
      - name: repo
        resource: repo
    taskRef:
      name: echo-task
  - name: pipeline-task-1
    params:
    - name: text
      value: how to ci/cd?
    resources:
      inputs:
      - name: repo
        resource: repo
      outputs:
      - name: repo
        resource: repo
    taskRef:
      name: echo-task
`, namespace))
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag-pipeline: %s", err)
	}
	pipelineRun := parse.MustParsePipelineRun(t, fmt.Sprintf(`
metadata:
  name: dag-pipeline-run
  namespace: %s
spec:
  pipelineRef:
    name: dag-pipeline
  resources:
  - name: repo
    resourceRef:
      name: repo
`, namespace))
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag-pipeline-run PipelineRun: %s", err)
	}
	t.Logf("Waiting for DAG pipeline to complete")
	if err := WaitForPipelineRunState(ctx, c, "dag-pipeline-run", timeout, PipelineRunSucceed("dag-pipeline-run"), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %s", err)
	}

	verifyExpectedOrder(ctx, t, c.TaskRunClient)
}

func verifyExpectedOrder(ctx context.Context, t *testing.T, c clientset.TaskRunInterface) {
	t.Logf("Verifying order of execution")

	taskRunsResp, err := c.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't get TaskRuns (so that we could check when they executed): %v", err)
	}
	taskRuns := taskRunsResp.Items
	if len(taskRuns) != 5 {
		t.Fatalf("Expected 5 TaskRuns to have executed but got start times for %d TaskRuns", len(taskRuns))
	}

	sort.Slice(taskRuns, func(i, j int) bool {
		it := taskRuns[i].Status.StartTime.Time
		jt := taskRuns[j].Status.StartTime.Time
		return it.Before(jt)
	})

	wantPrefixes := []string{
		"dag-pipeline-run-pipeline-task-1",
		// Could be task-2-parallel-1 or task-2-parallel-2
		"dag-pipeline-run-pipeline-task-2-parallel",
		"dag-pipeline-run-pipeline-task-2-parallel",
		"dag-pipeline-run-pipeline-task-3",
		"dag-pipeline-run-pipeline-task-4",
	}
	for i, wp := range wantPrefixes {
		if !strings.HasPrefix(taskRuns[i].Name, wp) {
			t.Errorf("Expected task %q to execute first, but %q was first", wp, taskRuns[0].Name)
		}
	}

	// Check that the two tasks that can run in parallel did
	s1 := taskRuns[1].Status.StartTime.Time
	s2 := taskRuns[2].Status.StartTime.Time
	absDiff := time.Duration(math.Abs(float64(s2.Sub(s1))))
	if absDiff > sleepDuration {
		t.Errorf("Expected parallel tasks to execute more or less at the same time, but they were %v apart", absDiff)
	}
}
