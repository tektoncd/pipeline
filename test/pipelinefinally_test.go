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
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var requireAlphaFeatureFlags = requireAnyGate(map[string]string{
	"enable-api-fields": "alpha",
})

func TestPipelineLevelFinally_OneDAGTaskFailed_InvalidTaskResult_Failure(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	task := getFailTask(t, namespace)
	task.Spec.Results = append(task.Spec.Results, v1.TaskResult{
		Name: "result",
	})
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	delayedTask := getDelaySuccessTask(t, namespace)
	if _, err := c.V1TaskClient.Create(ctx, delayedTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	successTask := getSuccessTask(t, namespace)
	successTask.Spec.Results = append(successTask.Spec.Results, v1.TaskResult{
		Name: "result",
	})
	if _, err := c.V1TaskClient.Create(ctx, successTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	finalTaskWithStatus := getTaskVerifyingStatus(t, namespace)
	if _, err := c.V1TaskClient.Create(ctx, finalTaskWithStatus, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task checking executing status: %s", err)
	}

	taskProducingResult := getSuccessTaskProducingResults(t, namespace)
	taskProducingResult.Spec.Results = append(taskProducingResult.Spec.Results, v1.TaskResult{
		Name: "result",
	})
	if _, err := c.V1TaskClient.Create(ctx, taskProducingResult, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task producing task results: %s", err)
	}

	taskConsumingResultInParam := getSuccessTaskConsumingResults(t, namespace, "dagtask-result")
	if _, err := c.V1TaskClient.Create(ctx, taskConsumingResultInParam, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task consuming task results in param: %s", err)
	}

	taskConsumingResultInWhenExpression := getSuccessTask(t, namespace)
	if _, err := c.V1TaskClient.Create(ctx, taskConsumingResultInWhenExpression, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task consuming task results in when expressions: %s", err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  finally:
  - name: finaltask1
    taskRef:
      name: %s
  - name: finaltask2
    params:
    - name: dagtask1-status
      value: $(tasks.dagtask1.status)
    - name: dagtask2-status
      value: $(tasks.dagtask2.status)
    - name: dagtask3-status
      value: $(tasks.dagtask3.status)
    - name: dagtasks-aggregate-status
      value: $(tasks.status)
    taskRef:
      name: %s
  - name: finaltaskconsumingdagtask1
    params:
    - name: dagtask-result
      value: $(tasks.dagtask1.results.result)
    taskRef:
      name: %s
  - name: finaltaskconsumingdagtask4
    params:
    - name: dagtask-result
      value: $(tasks.dagtask4.results.result)
    taskRef:
      name: %s
  - name: finaltaskconsumingdagtask5
    params:
    - name: dagtask-result
      value: $(tasks.dagtask5.results.result)
    taskRef:
      name: %s
  - name: guardedfinaltaskconsumingdagtask4
    taskRef:
      name: %s
    when:
    - input: $(tasks.dagtask4.results.result)
      operator: in
      values:
      - aResult
  - name: guardedfinaltaskusingdagtask5result1
    taskRef:
      name: %s
    when:
    - input: $(tasks.dagtask5.results.result)
      operator: in
      values:
      - Hello
  - name: guardedfinaltaskusingdagtask5result2
    taskRef:
      name: %s
    when:
    - input: $(tasks.dagtask5.results.result)
      operator: notin
      values:
      - Hello
  - name: guardedfinaltaskusingdagtask5status1
    taskRef:
      name: %s
    when:
    - input: $(tasks.dagtask5.status)
      operator: in
      values:
      - Succeeded
    - input: $(tasks.status)
      operator: in
      values:
      - Failed
  - name: guardedfinaltaskusingdagtask5status2
    taskRef:
      name: %s
    when:
    - input: $(tasks.dagtask5.status)
      operator: in
      values:
      - Failed
    - input: $(tasks.status)
      operator: notin
      values:
      - Failed
  tasks:
  - name: dagtask1
    taskRef:
      name: %s
  - name: dagtask2
    taskRef:
      name: %s
  - name: dagtask3
    taskRef:
      name: %s
    when:
    - input: banana
      operator: in
      values:
      - apple
  - name: dagtask4
    taskRef:
      name: %s
    when:
    - input: foo
      operator: notin
      values:
      - foo
  - name: dagtask5
    taskRef:
      name: %s
`, helpers.ObjectNameForTest(t), namespace,
		// Finally
		successTask.Name, finalTaskWithStatus.Name, taskConsumingResultInParam.Name,
		taskConsumingResultInParam.Name, taskConsumingResultInParam.Name, taskConsumingResultInWhenExpression.Name,
		taskConsumingResultInWhenExpression.Name, taskConsumingResultInWhenExpression.Name, taskConsumingResultInWhenExpression.Name,
		taskConsumingResultInWhenExpression.Name,
		// Tasks
		task.Name, delayedTask.Name, successTask.Name, successTask.Name, taskProducingResult.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := getPipelineRun(t, namespace, pipeline.Name)
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Waiting for PipelineRun %s to fail: %v", pipelineRun.Name, err)
	}

	// Get the status of the PipelineRun.
	pr, err := c.V1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// expecting taskRuns for dagtask1, dagtask2, dagtask5, finaltask1, finaltask2,
	// finaltaskconsumingdagtask5, guardedfinaltaskusingdagtask5result1, guardedfinaltaskusingdagtask5status1
	expectedTaskRunsCount := 8
	if len(taskrunList.Items) != expectedTaskRunsCount {
		var s []string
		for _, n := range taskrunList.Items {
			s = append(s, n.Labels["tekton.dev/pipelineTask"])
		}
		t.Fatalf("Error retrieving TaskRuns for PipelineRun %s. Expected %d taskRuns and found %d taskRuns for: %s",
			pipelineRun.Name, expectedTaskRunsCount, len(taskrunList.Items), strings.Join(s, ", "))
	}

	var dagTask1EndTime, dagTask2EndTime, finalTaskStartTime *metav1.Time
	// verify dag task failed, parallel dag task succeeded, and final task succeeded
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Labels["tekton.dev/pipelineTask"]; {
		case n == "dagtask1":
			if !isFailed(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("dag task %s should have failed", n)
			}
			dagTask1EndTime = taskrunItem.Status.CompletionTime
		case n == "dagtask2":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			dagTask2EndTime = taskrunItem.Status.CompletionTime
		case n == "dagtask4":
			t.Fatalf("task %s should have skipped due to when expression", n)
		case n == "dagtask5":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
		case n == "finaltask1":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			finalTaskStartTime = taskrunItem.Status.StartTime
		case n == "finaltask2":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			for _, p := range taskrunItem.Spec.Params {
				switch param := p.Name; param {
				case "dagtask1-status":
					if p.Value.StringVal != v1.TaskRunReasonFailed.String() {
						t.Errorf("Task param \"%s\" is set to \"%s\", expected it to resolve to \"%s\"", param, p.Value.StringVal, v1.TaskRunReasonFailed.String())
					}
				case "dagtask2-status":
					if p.Value.StringVal != v1.TaskRunReasonSuccessful.String() {
						t.Errorf("Task param \"%s\" is set to \"%s\", expected it to resolve to \"%s\"", param, p.Value.StringVal, v1.TaskRunReasonSuccessful.String())
					}

				case "dagtask3-status":
					if p.Value.StringVal != resources.PipelineTaskStateNone {
						t.Errorf("Task param \"%s\" is set to \"%s\", expected it to resolve to \"%s\"", param, p.Value.StringVal, resources.PipelineTaskStateNone)
					}
				case "dagtasks-aggregate-status":
					if p.Value.StringVal != v1.PipelineRunReasonFailed.String() {
						t.Errorf("Task param \"%s\" is set to \"%s\", expected it to resolve to \"%s\"", param, p.Value.StringVal, v1.PipelineRunReasonFailed.String())
					}
				}
			}
		case n == "finaltaskconsumingdagtask5":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			for _, p := range taskrunItem.Spec.Params {
				if p.Name == "dagtask-result" && p.Value.StringVal != "Hello" {
					t.Errorf("Error resolving task result reference in a finally task %s", n)
				}
			}
		case n == "guardedfinaltaskusingdagtask5result1":
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("final task %s should have succeeded", n)
			}
		case n == "guardedfinaltaskusingdagtask5status1":
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("final task %s should have succeeded", n)
			}
		case n == "guardedfinaltaskusingdagtask5result2":
			t.Fatalf("final task %s should have skipped due to when expression evaluating to false", n)
		case n == "finaltaskconsumingdagtask1" || n == "finaltaskconsumingdagtask4" || n == "guardedfinaltaskconsumingdagtask4":
			t.Fatalf("final task %s should have skipped due to missing task result reference", n)
		default:
			t.Fatalf("Found unexpected taskRun %s", n)
		}
	}
	// final task should start executing after dagtask1 fails and dagtask2 is done
	if finalTaskStartTime.Before(dagTask1EndTime) || finalTaskStartTime.Before(dagTask2EndTime) {
		t.Fatalf("Final Tasks should start getting executed after all DAG tasks finishes")
	}

	// two final tasks referring to results must be skipped
	// finaltaskconsumingdagtask1 has a reference to a task result from failed task
	// finaltaskconsumingdagtask4 has a reference to a task result from skipped task with when expression
	expectedSkippedTasks := []v1.SkippedTask{{
		Name:   "dagtask3",
		Reason: v1.StoppingSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "banana",
			Operator: "in",
			Values:   []string{"apple"},
		}},
	}, {
		Name:   "dagtask4",
		Reason: v1.StoppingSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "foo",
			Operator: "notin",
			Values:   []string{"foo"},
		}},
	}, {
		Name:   "finaltaskconsumingdagtask1",
		Reason: v1.MissingResultsSkip,
	}, {
		Name:   "finaltaskconsumingdagtask4",
		Reason: v1.MissingResultsSkip,
	}, {
		Name:   "guardedfinaltaskconsumingdagtask4",
		Reason: v1.MissingResultsSkip,
	}, {
		Name:   "guardedfinaltaskusingdagtask5result2",
		Reason: v1.WhenExpressionsSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "Hello",
			Operator: "notin",
			Values:   []string{"Hello"},
		}},
	}, {
		Name:   "guardedfinaltaskusingdagtask5status2",
		Reason: v1.WhenExpressionsSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "Succeeded",
			Operator: "in",
			Values:   []string{"Failed"},
		}, {
			Input:    "Failed",
			Operator: "notin",
			Values:   []string{"Failed"},
		}},
	}}

	actualSkippedTasks := pr.Status.SkippedTasks
	// Sort tasks based on their names to get similar order as in expected list
	if d := cmp.Diff(actualSkippedTasks, expectedSkippedTasks, cmpopts.SortSlices(func(i, j v1.SkippedTask) bool {
		return i.Name < j.Name
	})); d != "" {
		t.Fatalf("Expected four skipped tasks, dag task with condition failure dagtask3, dag task with when expression,"+
			"two final tasks with missing result reference finaltaskconsumingdagtask1 and finaltaskconsumingdagtask4 in SkippedTasks."+
			" Diff: %s", diff.PrintWantGot(d))
	}
}

func TestPipelineLevelFinally_OneFinalTaskFailed_Failure(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	task := getSuccessTask(t, namespace)
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	finalTask := getFailTask(t, namespace)
	if _, err := c.V1TaskClient.Create(ctx, finalTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  finally:
  - name: finaltask1
    taskRef:
      name: %s
  tasks:
  - name: dagtask1
    taskRef:
      name: %s
`, helpers.ObjectNameForTest(t), namespace, finalTask.Name, task.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := getPipelineRun(t, namespace, pipeline.Name)
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "PipelineRunFailed", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// verify dag task succeeded and final task failed
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Labels["tekton.dev/pipelineTask"]; {
		case n == "dagtask1":
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("dag task %s should have succeeded", n)
			}
		case n == "finaltask1":
			if !isFailed(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("final task %s should have failed", n)
			}
		default:
			t.Fatalf("TaskRuns were not found for both final and dag tasks")
		}
	}
}

func TestPipelineLevelFinally_OneFinalTask_CancelledRunFinally(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAlphaFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	task1 := getDelaySuccessTaskProducingResults(t, namespace)
	task1.Spec.Results = append(task1.Spec.Results, v1.TaskResult{
		Name: "result",
	})
	if _, err := c.V1TaskClient.Create(ctx, task1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	task2 := getSuccessTaskConsumingResults(t, namespace, "dagtask1-result")
	if _, err := c.V1TaskClient.Create(ctx, task2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	finalTask1 := getSuccessTask(t, namespace)
	if _, err := c.V1TaskClient.Create(ctx, finalTask1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	finalTask2 := getSuccessTaskConsumingResults(t, namespace, "dagtask1-result")
	if _, err := c.V1TaskClient.Create(ctx, finalTask2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  finally:
  - name: finaltask1
    taskRef:
      name: %s
  - name: finaltask2
    params:
    - name: dagtask1-result
      value: $(tasks.dagtask1.results.result)
    taskRef:
      name: %s
  tasks:
  - name: dagtask1
    taskRef:
      name: %s
  - name: dagtask2
    params:
    - name: dagtask1-result
      value: $(tasks.dagtask1.results.result)
    taskRef:
      name: %s
`, helpers.ObjectNameForTest(t), namespace, finalTask1.Name, finalTask2.Name, task1.Name, task2.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := getPipelineRun(t, namespace, pipeline.Name)
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, Running(pipelineRun.Name), "PipelineRunRunning", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to start: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1.PipelineRunSpecStatusCancelledRunFinally,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		t.Fatalf("failed to marshal patch bytes in order to stop")
	}
	if _, err := c.V1PipelineRunClient.Patch(ctx, pipelineRun.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
		t.Fatalf("Failed to patch PipelineRun `%s` with graceful stop: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1.PipelineRunReasonCancelled.String(), pipelineRun.Name), "PipelineRunCancelled", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// verify dag task succeeded and final task failed
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Labels["tekton.dev/pipelineTask"]; n {
		case "dagtask1":
			if !isCancelled(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("dag task %s should have been cancelled", n)
			}
		case "dagtask2":
			t.Fatalf("second dag task %s should be skipped as it depends on the result from cancelled 'dagtask1'", n)
		case "finaltask1":
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("first final task %s should have succeeded", n)
			}
		case "finaltask2":
			t.Fatalf("second final task %s should be skipped as it depends on the result from cancelled 'dagtask1'", n)
		default:
			t.Fatalf("TaskRuns were not found for both final and dag tasks")
		}
	}
}

func TestPipelineLevelFinally_OneFinalTask_StoppedRunFinally(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAlphaFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	task1 := getDelaySuccessTaskProducingResults(t, namespace)
	task1.Spec.Results = append(task1.Spec.Results, v1.TaskResult{
		Name: "result",
	})
	if _, err := c.V1TaskClient.Create(ctx, task1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	task2 := getSuccessTaskConsumingResults(t, namespace, "dagtask1-result")
	if _, err := c.V1TaskClient.Create(ctx, task2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	finalTask1 := getSuccessTask(t, namespace)
	if _, err := c.V1TaskClient.Create(ctx, finalTask1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	finalTask2 := getSuccessTaskConsumingResults(t, namespace, "dagtask1-result")
	if _, err := c.V1TaskClient.Create(ctx, finalTask2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  finally:
  - name: finaltask1
    taskRef:
      name: %s
  - name: finaltask2
    params:
    - name: dagtask1-result
      value: $(tasks.dagtask1.results.result)
    taskRef:
      name: %s
  tasks:
  - name: dagtask1
    taskRef:
      name: %s
  - name: dagtask2
    params:
    - name: dagtask1-result
      value: $(tasks.dagtask1.results.result)
    taskRef:
      name: %s
`, helpers.ObjectNameForTest(t), namespace, finalTask1.Name, finalTask2.Name, task1.Name, task2.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := getPipelineRun(t, namespace, pipeline.Name)
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, Running(pipelineRun.Name), "PipelineRunRunning", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to start: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1.PipelineRunSpecStatusStoppedRunFinally,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		t.Fatalf("failed to marshal patch bytes in order to stop")
	}
	if _, err := c.V1PipelineRunClient.Patch(ctx, pipelineRun.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
		t.Fatalf("Failed to patch PipelineRun `%s` with graceful stop: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1.PipelineRunReasonCancelled.String(), pipelineRun.Name), "PipelineRunCancelled", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// verify dag task succeeded and final task failed
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Labels["tekton.dev/pipelineTask"]; n {
		case "dagtask1":
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("dag task %s should have succeeded", n)
			}
		case "finaltask1":
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("first final task %s should have succeeded", n)
			}
		case "finaltask2":
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("second final task %s should have succeeded", n)
			}
		default:
			t.Fatalf("TaskRuns were not found for both final and dag tasks")
		}
	}
}

func isSuccessful(t *testing.T, taskRunName string, conds duckv1.Conditions) bool {
	t.Helper()
	for _, c := range conds {
		if c.Type == apis.ConditionSucceeded {
			if c.Status != corev1.ConditionTrue {
				t.Errorf("TaskRun status %q is not succeeded, got %q", taskRunName, c.Status)
			}
			return true
		}
	}
	t.Errorf("TaskRun status %q had no Succeeded condition", taskRunName)
	return false
}

func isCancelled(t *testing.T, taskRunName string, conds duckv1.Conditions) bool {
	t.Helper()
	for _, c := range conds {
		if c.Type == apis.ConditionSucceeded {
			return true
		}
	}
	t.Errorf("TaskRun status %q had no Succeeded condition", taskRunName)
	return false
}

func getSuccessTask(t *testing.T, namespace string) *v1.Task {
	t.Helper()
	return parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: alpine
    script: 'exit 0'
`, helpers.ObjectNameForTest(t), namespace))
}

func getFailTask(t *testing.T, namespace string) *v1.Task {
	t.Helper()
	return parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: alpine
    script: 'exit 1'
`, helpers.ObjectNameForTest(t), namespace))
}

func getDelaySuccessTask(t *testing.T, namespace string) *v1.Task {
	t.Helper()
	return parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: alpine
    script: 'sleep 5; exit 0'
`, helpers.ObjectNameForTest(t), namespace))
}

func getTaskVerifyingStatus(t *testing.T, namespace string) *v1.Task {
	t.Helper()
	return parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: alpine
    script: 'exit 0'
  params:
  - name: dagtask1-status
  - name: dagtask2-status
  - name: dagtask3-status
  - name: dagtasks-aggregate-status
`, helpers.ObjectNameForTest(t), namespace))
}

func getSuccessTaskProducingResults(t *testing.T, namespace string) *v1.Task {
	t.Helper()
	return parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: alpine
    script: 'echo -n "Hello" > $(results.result.path)'
  results:
  - name: result
`, helpers.ObjectNameForTest(t), namespace))
}

func getDelaySuccessTaskProducingResults(t *testing.T, namespace string) *v1.Task {
	t.Helper()
	return parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: alpine
    script: 'sleep 5; echo -n "Hello" > $(results.result.path)'
  results:
  - name: result
`, helpers.ObjectNameForTest(t), namespace))
}

func getSuccessTaskConsumingResults(t *testing.T, namespace string, paramName string) *v1.Task {
	t.Helper()
	return parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: alpine
    script: 'exit 0'
  params:
  - name: %s
`, helpers.ObjectNameForTest(t), namespace, paramName))
}

func getPipelineRun(t *testing.T, namespace, p string) *v1.PipelineRun {
	t.Helper()
	return parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
`, helpers.ObjectNameForTest(t), namespace, p))
}
