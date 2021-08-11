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
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"

	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
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

	cond := getCondition(t, namespace)
	if _, err := c.ConditionClient.Create(ctx, cond, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Condition `%s`: %s", cond1Name, err)
	}

	task := getFailTask(t, namespace)
	task.Spec.Results = append(task.Spec.Results, v1beta1.TaskResult{
		Name: "result",
	})
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	delayedTask := getDelaySuccessTask(t, namespace)
	if _, err := c.TaskClient.Create(ctx, delayedTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	successTask := getSuccessTask(t, namespace)
	successTask.Spec.Results = append(successTask.Spec.Results, v1beta1.TaskResult{
		Name: "result",
	})
	if _, err := c.TaskClient.Create(ctx, successTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	finalTaskWithStatus := getTaskVerifyingStatus(t, namespace)
	if _, err := c.TaskClient.Create(ctx, finalTaskWithStatus, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task checking executing status: %s", err)
	}

	taskProducingResult := getSuccessTaskProducingResults(t, namespace)
	taskProducingResult.Spec.Results = append(taskProducingResult.Spec.Results, v1beta1.TaskResult{
		Name: "result",
	})
	if _, err := c.TaskClient.Create(ctx, taskProducingResult, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task producing task results: %s", err)
	}

	taskConsumingResultInParam := getSuccessTaskConsumingResults(t, namespace, []v1beta1.ParamSpec{{Name: "dagtask-result"}}, []v1beta1.TaskResult{})
	if _, err := c.TaskClient.Create(ctx, taskConsumingResultInParam, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task consuming task results in param: %s", err)
	}

	taskConsumingResultInWhenExpression := getSuccessTaskConsumingResults(t, namespace, []v1beta1.ParamSpec{}, []v1beta1.TaskResult{})
	if _, err := c.TaskClient.Create(ctx, taskConsumingResultInWhenExpression, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task consuming task results in when expressions: %s", err)
	}

	we := v1beta1.WhenExpressions{{
		Input:    "foo",
		Operator: "notin",
		Values:   []string{"foo"},
	}}

	dag := map[string]pipelineTask{
		"dagtask1": {
			TaskName: task.Name,
			RunAfter: []string{"dagtask2", "dagtask3", "dagtask4", "dagtask5"},
		},
		"dagtask2": {
			TaskName: delayedTask.Name,
		},
		"dagtask3": {
			TaskName:  successTask.Name,
			Condition: cond.Name,
		},
		"dagtask4": {
			TaskName: successTask.Name,
			When:     we,
		},
		"dagtask5": {
			TaskName: taskProducingResult.Name,
		},
	}

	f := map[string]pipelineTask{
		"finaltask1": {
			TaskName: successTask.Name,
		},
		"finaltask2": {
			TaskName: finalTaskWithStatus.Name,
			Param: []v1beta1.Param{{
				Name: "dagtask1-status",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask1.status)",
				},
			}, {
				Name: "dagtask2-status",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask2.status)",
				},
			}, {
				Name: "dagtask3-status",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask3.status)",
				},
			}, {
				Name: "dagtasks-aggregate-status",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.status)",
				},
			}},
		},
		// final task consuming result from a failed dag task
		"finaltaskconsumingdagtask1": {
			TaskName: taskConsumingResultInParam.Name,
			Param: []v1beta1.Param{{
				Name: "dagtask-result",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask1.results.result)",
				},
			}},
		},
		// final task consuming result from a skipped dag task due to when expression
		"finaltaskconsumingdagtask4": {
			TaskName: taskConsumingResultInParam.Name,
			Param: []v1beta1.Param{{
				Name: "dagtask-result",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask4.results.result)",
				},
			}},
		},
		"finaltaskconsumingdagtask5": {
			TaskName: taskConsumingResultInParam.Name,
			Param: []v1beta1.Param{{
				Name: "dagtask-result",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask5.results.result)",
				},
			}},
		},
		// final task with when expressions using results from skipped dag task
		"guardedfinaltaskconsumingdagtask4": {
			TaskName: taskConsumingResultInWhenExpression.Name,
			When: v1beta1.WhenExpressions{{
				Input:    "$(tasks.dagtask4.results.result)",
				Operator: "in",
				Values:   []string{"aResult"},
			}},
		},
		// final task with when expressions using results from successful dag task
		// when expressions evaluate to true
		"guardedfinaltaskusingdagtask5result1": {
			TaskName: taskConsumingResultInWhenExpression.Name,
			When: v1beta1.WhenExpressions{{
				Input:    "$(tasks.dagtask5.results.result)",
				Operator: "in",
				Values:   []string{"Hello"},
			}},
		},
		// final task with when expressions using results from successful dag task
		// when expressions evaluate to false
		"guardedfinaltaskusingdagtask5result2": {
			TaskName: taskConsumingResultInWhenExpression.Name,
			When: v1beta1.WhenExpressions{{
				Input:    "$(tasks.dagtask5.results.result)",
				Operator: "notin",
				Values:   []string{"Hello"},
			}},
		},
		// final task with when expressions using execution status of a dag task
		// when expressions evaluate to true
		"guardedfinaltaskusingdagtask5status1": {
			TaskName: taskConsumingResultInWhenExpression.Name,
			When: v1beta1.WhenExpressions{{
				Input:    "$(tasks.dagtask5.status)",
				Operator: "in",
				Values:   []string{"Succeeded"},
			}, {
				Input:    "$(tasks.status)",
				Operator: "in",
				Values:   []string{"Failed"},
			}},
		},
		// final task with when expressions using execution status of a dag task
		// when expressions evaluate to false
		"guardedfinaltaskusingdagtask5status2": {
			TaskName: taskConsumingResultInWhenExpression.Name,
			When: v1beta1.WhenExpressions{{
				Input:    "$(tasks.dagtask5.status)",
				Operator: "in",
				Values:   []string{"Failed"},
			}, {
				Input:    "$(tasks.status)",
				Operator: "notin",
				Values:   []string{"Failed"},
			}},
		},
	}

	pipeline := getPipeline(t, namespace, dag, f)

	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := getPipelineRun(t, namespace, pipeline.Name)
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "PipelineRunFailed"); err != nil {
		t.Fatalf("Waiting for PipelineRun %s to fail: %v", pipelineRun.Name, err)
	}

	// Get the status of the PipelineRun.
	pr, err := c.PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
	}

	taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// expecting taskRuns for dagtask1, dagtask2, dagtask3 (with condition failure), dagtask5, finaltask1, finaltask2,
	// finaltaskconsumingdagtask5, guardedfinaltaskusingdagtask5result1, guardedfinaltaskusingdagtask5status1
	expectedTaskRunsCount := 9
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
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess"); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			dagTask2EndTime = taskrunItem.Status.CompletionTime
		case n == "dagtask3":
			if !isSkipped(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("dag task %s should have skipped due to condition failure", n)
			}
		case n == "dagtask4":
			t.Fatalf("task %s should have skipped due to when expression", n)
		case n == "dagtask5":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess"); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
		case n == "finaltask1":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess"); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			finalTaskStartTime = taskrunItem.Status.StartTime
		case n == "finaltask2":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess"); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			for _, p := range taskrunItem.Spec.Params {
				switch param := p.Name; param {
				case "dagtask1-status":
					if p.Value.StringVal != v1beta1.TaskRunReasonFailed.String() {
						t.Errorf("Task param \"%s\" is set to \"%s\", expected it to resolve to \"%s\"", param, p.Value.StringVal, v1beta1.TaskRunReasonFailed.String())
					}
				case "dagtask2-status":
					if p.Value.StringVal != v1beta1.TaskRunReasonSuccessful.String() {
						t.Errorf("Task param \"%s\" is set to \"%s\", expected it to resolve to \"%s\"", param, p.Value.StringVal, v1beta1.TaskRunReasonSuccessful.String())
					}

				case "dagtask3-status":
					if p.Value.StringVal != resources.PipelineTaskStateNone {
						t.Errorf("Task param \"%s\" is set to \"%s\", expected it to resolve to \"%s\"", param, p.Value.StringVal, resources.PipelineTaskStateNone)
					}
				case "dagtasks-aggregate-status":
					if p.Value.StringVal != v1beta1.PipelineRunReasonFailed.String() {
						t.Errorf("Task param \"%s\" is set to \"%s\", expected it to resolve to \"%s\"", param, p.Value.StringVal, v1beta1.PipelineRunReasonFailed.String())
					}
				}
			}
		case n == "finaltaskconsumingdagtask5":
			if err := WaitForTaskRunState(ctx, c, taskrunItem.Name, TaskRunSucceed(taskrunItem.Name), "TaskRunSuccess"); err != nil {
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
	expectedSkippedTasks := []v1beta1.SkippedTask{{
		Name: "dagtask3",
	}, {
		Name:            "dagtask4",
		WhenExpressions: we,
	}, {
		Name: "finaltaskconsumingdagtask1",
	}, {
		Name: "finaltaskconsumingdagtask4",
	}, {
		Name: "guardedfinaltaskconsumingdagtask4",
	}, {
		Name: "guardedfinaltaskusingdagtask5result2",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "Hello",
			Operator: "notin",
			Values:   []string{"Hello"},
		}},
	}, {
		Name: "guardedfinaltaskusingdagtask5status2",
		WhenExpressions: v1beta1.WhenExpressions{{
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
	sort.Slice(actualSkippedTasks, func(i int, j int) bool { return actualSkippedTasks[i].Name < actualSkippedTasks[j].Name })

	if d := cmp.Diff(actualSkippedTasks, expectedSkippedTasks); d != "" {
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
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	finalTask := getFailTask(t, namespace)
	if _, err := c.TaskClient.Create(ctx, finalTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pt := map[string]pipelineTask{
		"dagtask1": {TaskName: task.Name},
	}

	fpt := map[string]pipelineTask{
		"finaltask1": {TaskName: finalTask.Name},
	}

	pipeline := getPipeline(t, namespace, pt, fpt)
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := getPipelineRun(t, namespace, pipeline.Name)
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "PipelineRunFailed"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
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
	task1.Spec.Results = append(task1.Spec.Results, v1beta1.TaskResult{
		Name: "result",
	})
	if _, err := c.TaskClient.Create(ctx, task1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	task2 := getSuccessTaskConsumingResults(t, namespace, []v1beta1.ParamSpec{{Name: "dagtask1-result"}}, []v1beta1.TaskResult{})
	if _, err := c.TaskClient.Create(ctx, task2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	finalTask1 := getSuccessTask(t, namespace)
	if _, err := c.TaskClient.Create(ctx, finalTask1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	finalTask2 := getSuccessTaskConsumingResults(t, namespace, []v1beta1.ParamSpec{{Name: "dagtask1-result"}}, []v1beta1.TaskResult{})
	if _, err := c.TaskClient.Create(ctx, finalTask2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pt := map[string]pipelineTask{
		"dagtask1": {
			TaskName: task1.Name,
		},
		"dagtask2": {
			TaskName: task2.Name,
			Param: []v1beta1.Param{{
				Name: "dagtask1-result",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask1.results.result)",
				},
			}},
		},
	}

	fpt := map[string]pipelineTask{
		"finaltask1": {
			TaskName: finalTask1.Name,
		},
		"finaltask2": {
			TaskName: finalTask2.Name,
			Param: []v1beta1.Param{{
				Name: "dagtask1-result",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask1.results.result)",
				},
			}},
		},
	}

	pipeline := getPipeline(t, namespace, pt, fpt)
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := getPipelineRun(t, namespace, pipeline.Name)
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, Running(pipelineRun.Name), "PipelineRunRunning"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to start: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1beta1.PipelineRunSpecStatusCancelledRunFinally,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		t.Fatalf("failed to marshal patch bytes in order to stop")
	}
	if _, err := c.PipelineRunClient.Patch(ctx, pipelineRun.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
		t.Fatalf("Failed to patch PipelineRun `%s` with graceful stop: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1beta1.PipelineRunReasonCancelled.String(), pipelineRun.Name), "PipelineRunCancelled"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
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
	task1.Spec.Results = append(task1.Spec.Results, v1beta1.TaskResult{
		Name: "result",
	})
	if _, err := c.TaskClient.Create(ctx, task1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	task2 := getSuccessTaskConsumingResults(t, namespace, []v1beta1.ParamSpec{{Name: "dagtask1-result"}}, []v1beta1.TaskResult{})
	if _, err := c.TaskClient.Create(ctx, task2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	finalTask1 := getSuccessTask(t, namespace)
	if _, err := c.TaskClient.Create(ctx, finalTask1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	finalTask2 := getSuccessTaskConsumingResults(t, namespace, []v1beta1.ParamSpec{{Name: "dagtask1-result"}}, []v1beta1.TaskResult{})
	if _, err := c.TaskClient.Create(ctx, finalTask2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pt := map[string]pipelineTask{
		"dagtask1": {
			TaskName: task1.Name,
		},
		"dagtask2": {
			TaskName: task2.Name,
			Param: []v1beta1.Param{{
				Name: "dagtask1-result",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask1.results.result)",
				},
			}},
		},
	}

	fpt := map[string]pipelineTask{
		"finaltask1": {
			TaskName: finalTask1.Name,
		},
		"finaltask2": {
			TaskName: finalTask2.Name,
			Param: []v1beta1.Param{{
				Name: "dagtask1-result",
				Value: v1beta1.ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.dagtask1.results.result)",
				},
			}},
		},
	}

	pipeline := getPipeline(t, namespace, pt, fpt)
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := getPipelineRun(t, namespace, pipeline.Name)
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, Running(pipelineRun.Name), "PipelineRunRunning"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to start: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1beta1.PipelineRunSpecStatusStoppedRunFinally,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		t.Fatalf("failed to marshal patch bytes in order to stop")
	}
	if _, err := c.PipelineRunClient.Patch(ctx, pipelineRun.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
		t.Fatalf("Failed to patch PipelineRun `%s` with graceful stop: %s", pipelineRun.Name, err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1beta1.PipelineRunReasonCancelled.String(), pipelineRun.Name), "PipelineRunCancelled"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
		t.Fatalf("PipelineRun execution failed")
	}

	taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
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

func isSuccessful(t *testing.T, taskRunName string, conds duckv1beta1.Conditions) bool {
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

func isCancelled(t *testing.T, taskRunName string, conds duckv1beta1.Conditions) bool {
	for _, c := range conds {
		if c.Type == apis.ConditionSucceeded {
			return true
		}
	}
	t.Errorf("TaskRun status %q had no Succeeded condition", taskRunName)
	return false
}

func isSkipped(t *testing.T, taskRunName string, conds duckv1beta1.Conditions) bool {
	for _, c := range conds {
		if c.Type == apis.ConditionSucceeded {
			if c.Status != corev1.ConditionFalse && c.Reason != resources.ReasonConditionCheckFailed {
				t.Errorf("TaskRun status %q is not skipped due to condition failure, got %q", taskRunName, c.Status)
			}
			return true
		}
	}
	t.Errorf("TaskRun status %q had no Succeeded condition", taskRunName)
	return false
}

func getTaskDef(n, namespace, script string, params []v1beta1.ParamSpec, results []v1beta1.TaskResult) *v1beta1.Task {
	return &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    script,
			}},
			Params:  params,
			Results: results,
		},
	}
}

func getSuccessTask(t *testing.T, namespace string) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "exit 0", []v1beta1.ParamSpec{}, []v1beta1.TaskResult{})
}

func getFailTask(t *testing.T, namespace string) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "exit 1", []v1beta1.ParamSpec{}, []v1beta1.TaskResult{})
}

func getDelaySuccessTask(t *testing.T, namespace string) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "sleep 5; exit 0", []v1beta1.ParamSpec{}, []v1beta1.TaskResult{})
}

func getTaskVerifyingStatus(t *testing.T, namespace string) *v1beta1.Task {
	params := []v1beta1.ParamSpec{{
		Name: "dagtask1-status",
	}, {
		Name: "dagtask2-status",
	}, {
		Name: "dagtask3-status",
	}, {
		Name: "dagtasks-aggregate-status",
	}}
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "exit 0", params, []v1beta1.TaskResult{})
}

func getSuccessTaskProducingResults(t *testing.T, namespace string) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "echo -n \"Hello\" > $(results.result.path)", []v1beta1.ParamSpec{}, []v1beta1.TaskResult{{Name: "result"}})
}

func getDelaySuccessTaskProducingResults(t *testing.T, namespace string) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "sleep 5; echo -n \"Hello\" > $(results.result.path)", []v1beta1.ParamSpec{}, []v1beta1.TaskResult{{Name: "result"}})
}

func getSuccessTaskConsumingResults(t *testing.T, namespace string, params []v1beta1.ParamSpec, results []v1beta1.TaskResult) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "exit 0", params, results)
}

func getCondition(t *testing.T, namespace string) *v1alpha1.Condition {
	return &v1alpha1.Condition{
		ObjectMeta: metav1.ObjectMeta{Name: helpers.ObjectNameForTest(t), Namespace: namespace},
		Spec: v1alpha1.ConditionSpec{
			Check: v1alpha1.Step{
				Container: corev1.Container{Image: "ubuntu"},
				Script:    "exit 1",
			},
		},
	}
}

type pipelineTask struct {
	TaskName  string
	Condition string
	Param     []v1beta1.Param
	When      v1beta1.WhenExpressions
	RunAfter  []string
}

func getPipeline(t *testing.T, namespace string, dag map[string]pipelineTask, f map[string]pipelineTask) *v1beta1.Pipeline {
	var pt []v1beta1.PipelineTask
	var fpt []v1beta1.PipelineTask
	for k, v := range dag {
		task := v1beta1.PipelineTask{
			Name:    k,
			TaskRef: &v1beta1.TaskRef{Name: v.TaskName},
		}
		if v.Condition != "" {
			task.Conditions = []v1beta1.PipelineTaskCondition{{
				ConditionRef: v.Condition,
			}}
		}
		if len(v.Param) != 0 {
			task.Params = v.Param
		}
		if len(v.When) != 0 {
			task.WhenExpressions = v.When
		}
		if len(v.RunAfter) != 0 {
			task.RunAfter = v.RunAfter
		}
		pt = append(pt, task)
	}
	for k, v := range f {
		finalTask := v1beta1.PipelineTask{
			Name:    k,
			TaskRef: &v1beta1.TaskRef{Name: v.TaskName},
		}
		if len(v.Param) != 0 {
			finalTask.Params = v.Param
		}
		if len(v.When) != 0 {
			finalTask.WhenExpressions = v.When
		}
		fpt = append(fpt, finalTask)
	}
	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: helpers.ObjectNameForTest(t), Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Tasks:   pt,
			Finally: fpt,
		},
	}
	return pipeline
}

func getPipelineRun(t *testing.T, namespace, p string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: helpers.ObjectNameForTest(t), Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: p},
		},
	}
}
