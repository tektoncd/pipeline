//go:build e2e
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

	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const sleepDuration = 15 * time.Second

// TestDAGPipelineRun creates a graph of arbitrary Tasks, then looks at the corresponding
// TaskRun start times to ensure they were run in the order intended, which is:
//
//	                            |
//	                     pipeline-task-1
//	                    /               \
//	pipeline-task-2-parallel-1    pipeline-task-2-parallel-2
//	                    \                /
//	                     pipeline-task-3
//	                            |
//	                     pipeline-task-4
func TestDAGPipelineRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	echoTask := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: text
    type: string
    description: 'The text that should be echoed'
  results:
    - name: result
  steps:
  - image: busybox
    script: 'echo $(params["text"])'
  - image: busybox
    # Sleep for N seconds so that we can check that tasks that
    # should be run in parallel have overlap.
    script: |
      sleep %d
      echo $(params.text) | tee $(results.result.path)
`, helpers.ObjectNameForTest(t), namespace, int(sleepDuration.Seconds())))
	if _, err := c.V1TaskClient.Create(ctx, echoTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create echo Task: %s", err)
	}

	// Intentionally declaring Tasks in a mixed up order to ensure the order
	// of execution isn't at all dependent on the order they are declared in
	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: pipeline-task-3
    params:
    - name: text
      value: wow
    - name: result-p2
      value: $(tasks.pipeline-task-2-parallel-2.results.result)
    - name: result-p1
      value: $(tasks.pipeline-task-2-parallel-1.results.result)
    taskRef:
      name: %s
  - name: pipeline-task-2-parallel-2
    params:
    - name: text
      value: such parallel
    - name: result1
      value: $(tasks.pipeline-task-1.results.result)
    taskRef:
      name: %s
  - name: pipeline-task-4
    params:
    - name: text
      value: very cloud native
    taskRef:
      name: %s
    runAfter:
      - pipeline-task-3
  - name: pipeline-task-2-parallel-1
    params:
    - name: text
      value: much graph
    taskRef:
      name: %s
    runAfter:
      - pipeline-task-1
  - name: pipeline-task-1
    params:
    - name: text
      value: how to ci/cd?
    taskRef:
      name: %s
`, helpers.ObjectNameForTest(t), namespace, echoTask.Name, echoTask.Name, echoTask.Name, echoTask.Name, echoTask.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag-pipeline: %s", err)
	}
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag-pipeline-run PipelineRun: %s", err)
	}
	t.Logf("Waiting for DAG pipeline to complete")
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunSucceed(pipelineRun.Name), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %s", err)
	}

	verifyExpectedOrder(ctx, t, c.V1TaskRunClient, pipelineRun.Name)
}

func verifyExpectedOrder(ctx context.Context, t *testing.T, c clientset.TaskRunInterface, prName string) {
	t.Helper()
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
		prName + "-pipeline-task-1",
		// Could be task-2-parallel-1 or task-2-parallel-2
		prName + "-pipeline-task-2-parallel",
		prName + "-pipeline-task-2-parallel",
		prName + "-pipeline-task-3",
		prName + "-pipeline-task-4",
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
