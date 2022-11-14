//go:build e2e
// +build e2e

/*
Copyright 2023 The Tekton Authors

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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestTaskResultsFromFailedTasks(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	pipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
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
          image: busybox
          script: 'echo -n 123 | tee $(results.result1.path); exit 1; echo -n 456 | tee $(results.result2.path)'
    finally:
    - name: finaltask1
      params:
      - name: param1
        value: $(tasks.task1.results.result1)
      taskSpec:
        params:
        - name: param1
        steps:
        - image: busybox
          script: 'exit 0'
    - name: finaltask2
      params:
      - name: param1
        value: $(tasks.task1.results.result2)
      taskSpec:
        params:
        - name: param1
        steps:
        - image: busybox
          script: exit 0`, helpers.ObjectNameForTest(t)))

	if _, err := c.V1beta1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to fail", namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1beta1.PipelineRunReasonFailed.String(), pipelineRun.Name), "InvalidTaskResultReference", v1beta1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to fail: %s", err)
	}

	taskrunList, err := c.V1beta1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	if len(taskrunList.Items) != 2 {
		t.Fatalf("The pipelineRun \"%s\" should have exactly 2 taskRuns, one for the task \"task1\""+
			"and one more for the final task \"finaltask1\" instead it has \"%d\" taskRuns", pipelineRun.Name, len(taskrunList.Items))
	}

	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Labels["tekton.dev/pipelineTask"]; n {
		case "task1":
			if !isFailed(t, "", taskrunItem.Status.Conditions) {
				t.Fatalf("task1 should have been a failure")
			}
			if len(taskrunItem.Status.TaskRunResults) != 1 {
				t.Fatalf("task1 should have produced a result even with the failing step")
			}
			for _, r := range taskrunItem.Status.TaskRunResults {
				if r.Name == "result1" && r.Value.StringVal != "123" {
					t.Fatalf("task1 should have initialized a result \"result1\" to \"123\"")
				}
			}
		case "finaltask1":
			if !isSuccessful(t, "", taskrunItem.Status.Conditions) {
				t.Fatalf("finaltask1 should have been successful")
			}
		default:
			t.Fatalf("TaskRuns were not found for both final and dag tasks")
		}
	}
}
