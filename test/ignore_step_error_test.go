// +build e2e

/*
Copyright 2021 The Tekton Authors

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
	"testing"

	"github.com/tektoncd/pipeline/test/parse"

	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestMissingResultWhenStepErrorIsIgnored(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	pipelineRun := parse.MustParsePipelineRun(t, `
metadata:
  name: pipelinerun-with-failing-step
spec:
  pipelineSpec:
    tasks:
    - name: task1
      taskSpec:
        results:
        - name: result1
        - name: result2
        steps:
        - name: failing-step
          onError: continue
          image: busybox
          script: 'echo -n 123 | tee $(results.result1.path); exit 1; echo -n 456 | tee $(results.result2.path)'
    - name: task2
      runAfter: [ task1 ]
      params:
      - name: param1
        value: $(tasks.task1.results.result1)
      - name: param2
        value: $(tasks.task1.results.result2)
      taskSpec:
        params:
        - name: param1
        - name: param2
        steps:
        - name: foo
          image: busybox
          script: 'exit 0'`)

	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to fail", namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(pipelinerun.ReasonInvalidTaskResultReference, pipelineRun.Name), "InvalidTaskResultReference"); err != nil {
		t.Errorf("Error waiting for PipelineRun to fail: %s", err)
	}

	taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	if len(taskrunList.Items) != 1 {
		t.Fatalf("The pipelineRun should have exactly 1 taskRun for the first task \"task1\"")
	}

	taskrunItem := taskrunList.Items[0]
	if taskrunItem.Labels["tekton.dev/pipelineTask"] != "task1" {
		t.Fatalf("TaskRun was not found for the task \"task1\"")
	}

	if len(taskrunItem.Status.TaskRunResults) != 1 {
		t.Fatalf("task1 should have produced a result before failing the step")
	}

	for _, r := range taskrunItem.Status.TaskRunResults {
		if r.Name == "result1" && r.Value != "123" {
			t.Fatalf("task1 should have initialized a result \"result1\" to \"123\"")
		}
	}

}
