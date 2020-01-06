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
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1beta1"
	knativetest "knative.dev/pkg/test"
)

func TestWorkspaceReadOnlyDisallowsWrite(t *testing.T) {
	c, namespace := setup(t)

	taskName := "write-disallowed"
	taskRunName := "write-disallowed-tr"

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	task := tb.Task(taskName, namespace, tb.TaskSpec(
		tb.Step("alpine", tb.StepScript("echo foo > /workspace/test/file")),
		tb.TaskWorkspace("test", "test workspace", "/workspace/test", true),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	taskRun := tb.TaskRun(taskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef(taskName), tb.TaskRunServiceAccountName("default"),
		tb.TaskRunWorkspaceEmptyDir("test", ""),
	))
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish", namespace)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunFailed(taskRunName), "error"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish with error: %s", err)
	}

	tr, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}
	if tr.Status.PodName == "" {
		t.Fatal("Error getting a PodName (empty)")
	}
	p, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(tr.Status.PodName, metav1.GetOptions{})

	if err != nil {
		t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
	}
	for _, stat := range p.Status.ContainerStatuses {
		if strings.Contains(stat.Name, "step-attempt-write") {
			req := c.KubeClient.Kube.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
			logContent, err := req.Do().Raw()
			if err != nil {
				t.Fatalf("Error getting pod logs for pod `%s` and container `%s` in namespace `%s`", tr.Status.PodName, stat.Name, namespace)
			}
			if !strings.Contains(string(logContent), "Read-only file system") {
				t.Fatalf("Expected read-only file system error but received %v", logContent)
			}
		}
	}
}

func TestWorkspacePipelineRunDuplicateWorkspaceEntriesInvalid(t *testing.T) {
	c, namespace := setup(t)

	taskName := "read-workspace"
	pipelineName := "read-workspace-pipeline"
	pipelineRunName := "read-workspace-pipelinerun"

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	task := tb.Task(taskName, namespace, tb.TaskSpec(
		tb.Step("alpine", tb.StepScript("cat /workspace/test/file")),
		tb.TaskWorkspace("test", "test workspace", "/workspace/test/file", true),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	pipeline := tb.Pipeline(pipelineName, namespace, tb.PipelineSpec(
		tb.PipelineWorkspaceDeclaration("foo"),
		tb.PipelineTask("task1", taskName, tb.PipelineTaskWorkspaceBinding("test", "foo")),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := tb.PipelineRun(pipelineRunName, namespace,
		tb.PipelineRunSpec(
			pipelineName,
			// These are the duplicated workspace entries that are being tested.
			tb.PipelineRunWorkspaceBindingEmptyDir("foo"),
			tb.PipelineRunWorkspaceBindingEmptyDir("foo"),
		),
	)
	_, err := c.PipelineRunClient.Create(pipelineRun)

	if err == nil || !strings.Contains(err.Error(), "provided by pipelinerun more than once") {
		t.Fatalf("Expected error when creating pipelinerun with duplicate workspace entries but received: %v", err)
	}
}

func TestWorkspacePipelineRunMissingWorkspaceInvalid(t *testing.T) {
	c, namespace := setup(t)

	taskName := "read-workspace"
	pipelineName := "read-workspace-pipeline"
	pipelineRunName := "read-workspace-pipelinerun"

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	task := tb.Task(taskName, namespace, tb.TaskSpec(
		tb.Step("alpine", tb.StepScript("cat /workspace/test/file")),
		tb.TaskWorkspace("test", "test workspace", "/workspace/test/file", true),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	pipeline := tb.Pipeline(pipelineName, namespace, tb.PipelineSpec(
		tb.PipelineWorkspaceDeclaration("foo"),
		tb.PipelineTask("task1", taskName, tb.PipelineTaskWorkspaceBinding("test", "foo")),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := tb.PipelineRun(pipelineRunName, namespace,
		tb.PipelineRunSpec(
			pipelineName,
		),
	)
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	conditions := make(v1beta1.Conditions, 0)

	if err := WaitForPipelineRunState(c, pipelineRunName, 10*time.Second, func(pr *v1alpha1.PipelineRun) (bool, error) {
		if len(pr.Status.Conditions) > 0 {
			conditions = pr.Status.Conditions
			return true, nil
		}
		return false, nil
	}, "PipelineRunHasCondition"); err != nil {
		t.Fatalf("Failed to wait for PipelineRun %q to finish: %s", pipelineRunName, err)
	}

	for _, condition := range conditions {
		if condition.Type == apis.ConditionSucceeded && condition.Status == corev1.ConditionFalse && strings.Contains(condition.Message, `pipeline expects workspace with name "foo" be provided by pipelinerun`) {
			return
		}
	}

	t.Fatalf("Expected failure condition after creating pipelinerun with missing workspace but did not receive one.")
}
