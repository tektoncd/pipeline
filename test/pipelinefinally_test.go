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
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"

	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestPipelineLevelFinally_OneDAGTaskFailed_Failure(t *testing.T) {
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
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	delayedTask := getDelaySuccessTask(t, namespace)
	if _, err := c.TaskClient.Create(ctx, delayedTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag Task: %s", err)
	}

	finalTask := getSuccessTask(t, namespace)
	if _, err := c.TaskClient.Create(ctx, finalTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pipeline := getPipeline(t,
		namespace,
		map[string]string{
			"dagtask1": task.Name,
			"dagtask2": delayedTask.Name,
			"dagtask3": finalTask.Name,
		},
		map[string]string{
			"dagtask3": cond.Name,
		},
		map[string]string{
			"finaltask1": finalTask.Name,
		},
	)
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

	taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=pipelinerun-failed-dag-tasks"})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	var dagTask1EndTime, dagTask2EndTime, finalTaskStartTime *metav1.Time
	// verify dag task failed, parallel dag task succeeded, and final task succeeded
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Name; {
		case strings.HasPrefix(n, "pipelinerun-failed-dag-tasks-dagtask1"):
			if !isFailed(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for dag task should have failed", n)
			}
			dagTask1EndTime = taskrunItem.Status.CompletionTime
		case strings.HasPrefix(n, "pipelinerun-failed-dag-tasks-dagtask2"):
			if err := WaitForTaskRunState(ctx, c, n, TaskRunSucceed(n), "TaskRunSuccess"); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			dagTask2EndTime = taskrunItem.Status.CompletionTime
		case strings.HasPrefix(n, "pipelinerun-failed-dag-tasks-dagtask3"):
			if !isSkipped(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for dag task should have skipped due to condition failure", n)
			}
		case strings.HasPrefix(n, "pipelinerun-failed-dag-tasks-finaltask1"):
			if err := WaitForTaskRunState(ctx, c, n, TaskRunSucceed(n), "TaskRunSuccess"); err != nil {
				t.Errorf("Error waiting for TaskRun to succeed: %v", err)
			}
			finalTaskStartTime = taskrunItem.Status.StartTime
		default:
			t.Fatalf("TaskRuns were not found for both final and dag tasks")
		}
	}
	// final task should start executing after dagtask1 fails and dagtask2 is done
	if finalTaskStartTime.Before(dagTask1EndTime) || finalTaskStartTime.Before(dagTask2EndTime) {
		t.Fatalf("Final Tasks should start getting executed after all DAG tasks finishes")
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

	pipeline := getPipeline(t,
		namespace,
		map[string]string{
			"dagtask1": task.Name,
		},
		map[string]string{},
		map[string]string{
			"finaltask1": finalTask.Name,
		},
	)
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

	taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=pipelinerun-failed-final-tasks"})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// verify dag task succeeded and final task failed
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Name; {
		case strings.HasPrefix(n, "pipelinerun-failed-final-tasks-dagtask1"):
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for dag task should have succeeded", n)
			}
		case strings.HasPrefix(n, "pipelinerun-failed-final-tasks-finaltask1"):
			if !isFailed(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for final task should have failed", n)
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

func getTaskDef(n, namespace, script string) *v1beta1.Task {
	return &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    script,
			}},
		},
	}
}

func getSuccessTask(t *testing.T, namespace string) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "exit 0")
}

func getFailTask(t *testing.T, namespace string) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "exit 1")
}

func getDelaySuccessTask(t *testing.T, namespace string) *v1beta1.Task {
	return getTaskDef(helpers.ObjectNameForTest(t), namespace, "sleep 5; exit 0")
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

func getPipeline(t *testing.T, namespace string, ts map[string]string, c map[string]string, f map[string]string) *v1beta1.Pipeline {
	var pt []v1beta1.PipelineTask
	var fpt []v1beta1.PipelineTask
	for k, v := range ts {
		task := v1beta1.PipelineTask{
			Name:    k,
			TaskRef: &v1beta1.TaskRef{Name: v},
		}
		if _, ok := c[k]; ok {
			task.Conditions = []v1beta1.PipelineTaskCondition{{
				ConditionRef: c[k],
			}}
		}
		pt = append(pt, task)
	}
	for k, v := range f {
		fpt = append(fpt, v1beta1.PipelineTask{
			Name:    k,
			TaskRef: &v1beta1.TaskRef{Name: v},
		})
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
