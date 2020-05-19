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
)

// Pipeline results in failure since dag task fails but final tasks does get executed
func TestPipelineLevelFinallyWhenOneDAGTaskFailed(t *testing.T) {
	c, namespace := setup(t)

	taskName := "dagtask"
	finalTaskName := "finaltask"
	pipelineName := "pipeline-with-failed-dag-tasks"
	pipelineRunName := "pipelinerun-with-failed-dag-tasks"

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: taskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "exit 1",
			}},
		},
	}
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create non final Task: %s", err)
	}

	finalTask := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: finalTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "exit 0",
			}},
		},
	}
	if _, err := c.TaskClient.Create(finalTask); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineName, Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name:    "dagtask1",
				TaskRef: &v1beta1.TaskRef{Name: taskName},
			}},
			Finally: []v1beta1.PipelineTask{{
				Name:    "finaltask1",
				TaskRef: &v1beta1.TaskRef{Name: finalTaskName},
			}},
		},
	}
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineRunName, Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: pipelineName},
		},
	}
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRunName, err)
	}

	if err := WaitForPipelineRunState(c, pipelineRunName, timeout, PipelineRunFailed(pipelineRunName), "PipelineRunFailed"); err != nil {
		t.Fatalf("Waiting for PipelineRun %s to fail: %v", pipelineRunName, err)
	}

	taskrunList, err := c.TaskRunClient.List(metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRunName})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRunName, err)
	}

	// verify non final task failed and final task executed and succeeded
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Name; {
		case strings.HasPrefix(n, pipelineRunName+"-"+finalTaskName):
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for final task should have succedded", n)
			}
		case strings.HasPrefix(n, pipelineRunName+"-"+taskName):
			if !isFailed(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for non final task should have failed", n)
			}
		default:
			t.Fatalf("TaskRuns were not found for both final and dag tasks")
		}
	}
}

// Pipeline exits with success even if final task execution fails
func TestPipelineLevelFinallyWhenOneFinalTaskFailed(t *testing.T) {
	c, namespace := setup(t)

	taskName := "dagtask"
	finalTaskName := "finaltask"
	pipelineName := "pipeline-with-failed-final-tasks"
	pipelineRunName := "pipelinerun-with-failed-final-tasks"

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: taskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "exit 0",
			}},
		},
	}
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create non final Task: %s", err)
	}

	finalTask := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: finalTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "exit 1",
			}},
		},
	}
	if _, err := c.TaskClient.Create(finalTask); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineName, Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name:    "dagtask1",
				TaskRef: &v1beta1.TaskRef{Name: taskName},
			}},
			Finally: []v1beta1.PipelineTask{{
				Name:    "finaltask1",
				TaskRef: &v1beta1.TaskRef{Name: finalTaskName},
			}},
		},
	}
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineRunName, Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: pipelineName},
		},
	}
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRunName, err)
	}

	if err := WaitForPipelineRunState(c, pipelineRunName, timeout, PipelineRunFailed(pipelineRunName), "PipelineRunFailed"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRunName, err)
		t.Fatalf("PipelineRun execution failed")
	}

	taskrunList, err := c.TaskRunClient.List(metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRunName})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRunName, err)
	}

	// verify non final task succeeded and final task failed
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Name; {
		case strings.HasPrefix(n, pipelineRunName+"-"+finalTaskName):
			if !isFailed(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for final task should have failed", n)
			}
		case strings.HasPrefix(n, pipelineRunName+"-"+taskName):
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for non final task should have succedded", n)
			}
		default:
			t.Fatalf("TaskRuns were not found for both final and dag tasks")
		}
	}
}

// The dag task is skipped due to condition failure, final tasks should still be executed
func TestPipelineLevelFinallyWhenDAGTaskSkipped(t *testing.T) {
	c, namespace := setup(t)

	taskName := "dagtask"
	finalTaskName := "finaltask"
	pipelineName := "pipeline-with-skipped-dag-tasks"
	pipelineRunName := "pipelinerun-with-skipped-dag-tasks"
	condName := "failedcondition"

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	cond := &v1alpha1.Condition{
		ObjectMeta: metav1.ObjectMeta{Name: condName, Namespace: namespace},
		Spec: v1alpha1.ConditionSpec{
			Check: v1alpha1.Step{
				Container: corev1.Container{Image: "ubuntu"},
				Script:    "exit 1",
			},
		},
	}
	if _, err := c.ConditionClient.Create(cond); err != nil {
		t.Fatalf("Failed to create Condition `%s`: %s", cond1Name, err)
	}

	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: taskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "exit 0",
			}},
		},
	}
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create non final Task: %s", err)
	}

	finalTask := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: finalTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "exit 0",
			}},
		},
	}
	if _, err := c.TaskClient.Create(finalTask); err != nil {
		t.Fatalf("Failed to create final Task: %s", err)
	}

	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineName, Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name:    "dagtask1",
				TaskRef: &v1beta1.TaskRef{Name: taskName},
				Conditions: []v1beta1.PipelineTaskCondition{{
					ConditionRef: condName,
				}},
			}},
			Finally: []v1beta1.PipelineTask{{
				Name:    "finaltask1",
				TaskRef: &v1beta1.TaskRef{Name: finalTaskName},
			}},
		},
	}
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineRunName, Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: pipelineName},
		},
	}
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create Pipeline Run `%s`: %s", pipelineRunName, err)
	}

	if err := WaitForPipelineRunState(c, pipelineRunName, timeout, PipelineRunSucceed(pipelineRunName), "PipelineRunCompleted"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRunName, err)
		t.Fatalf("PipelineRun execution failed")
	}

	taskrunList, err := c.TaskRunClient.List(metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRunName})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRunName, err)
	}

	// verify non final task skipped but final task executed and succeeded
	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Name; {
		case strings.HasPrefix(n, pipelineRunName+"-"+finalTaskName):
			if !isSuccessful(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for final task should have succeeded", n)
			}
		case strings.HasPrefix(n, pipelineRunName+"-"+taskName):
			if !isSkipped(t, n, taskrunItem.Status.Conditions) {
				t.Fatalf("TaskRun %s for non final task should have skipped due to condition failure", n)
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
