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
	"testing"

	"github.com/tektoncd/pipeline/test/parse"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// TestTaskRunPipelineRunStatus is an integration test that will
// verify a very simple "hello world" TaskRun and PipelineRun failure
// execution lead to the correct TaskRun status.
func TestTaskRunPipelineRunStatus(t *testing.T) {
	taskRunPipelineRunStatus(t, false)
}

// TestTaskRunPipelineRunStatusWithSpire is an integration test with spire enabled that will
// verify a very simple "hello world" TaskRun and PipelineRun failure
// execution lead to the correct TaskRun status.
func TestTaskRunPipelineRunStatusWithSpire(t *testing.T) {
	taskRunPipelineRunStatus(t, true)
}

func taskRunPipelineRunStatus(t *testing.T, spireEnabled bool) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var c *clients
	var namespace string

	if spireEnabled {
		c, namespace = setup(ctx, t, requireAnyGate(spireFeatureGates))
	} else {
		c, namespace = setup(ctx, t)
	}

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  steps:
  - name: foo
    image: busybox
    command: ['ls', '-la']`, helpers.ObjectNameForTest(t)))
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  taskRef:
    name: %s
  serviceAccountName: inexistent`, helpers.ObjectNameForTest(t), task.Name))
	if _, err := c.TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRun.Name, TaskRunFailed(taskRun.Name), "BuildValidationFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	if spireEnabled {
		tr, err := c.TaskRunClient.Get(ctx, taskRun.Name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Error retrieving taskrun: %s", err)
		}
		spireShouldFailTaskRunResultsVerify(tr, t)
		spireShouldPassSpireAnnotation(tr, t)
	}

	pipeline := parse.MustParsePipeline(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  tasks:
  - name: foo
    taskRef:
      name: %s`, helpers.ObjectNameForTest(t), task.Name))
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	pipelineRun := parse.MustParsePipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    name: %s
  serviceAccountName: inexistent`, helpers.ObjectNameForTest(t), pipeline.Name))
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to fail", namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "BuildValidationFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	if spireEnabled {
		taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
		if err != nil {
			t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
		}
		for _, taskrunItem := range taskrunList.Items {
			spireShouldFailTaskRunResultsVerify(&taskrunItem, t)
			spireShouldPassSpireAnnotation(&taskrunItem, t)
		}
	}

}
