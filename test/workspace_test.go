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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestWorkspaceReadOnlyDisallowsWrite(t *testing.T) {
	c, namespace := setup(t)

	taskName := "write-disallowed"
	taskRunName := "write-disallowed-tr"

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: taskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "echo foo > /workspace/test/file",
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:        "test",
				Description: "test workspace",
				MountPath:   "/workspace/test",
				ReadOnly:    true,
			}},
		},
	}
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	taskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: taskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: taskName},
			ServiceAccountName: "default",
			Workspaces: []v1beta1.WorkspaceBinding{{
				Name:     "test",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}},
		},
	}
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

	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: taskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "cat /workspace/test/file",
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:        "test",
				Description: "test workspace",
				MountPath:   "/workspace/test/file",
				ReadOnly:    true,
			}},
		},
	}
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineName, Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
				Name: "foo",
			}},
			Tasks: []v1beta1.PipelineTask{{
				Name:    "task1",
				TaskRef: &v1beta1.TaskRef{Name: taskName},
				Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
					Name:      "test",
					Workspace: "foo",
				}},
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
			Workspaces: []v1beta1.WorkspaceBinding{{
				Name:     "foo",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}, {
				Name:     "foo",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}},
		},
	}
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

	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: taskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "alpine"},
				Script:    "cat /workspace/test/file",
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:        "test",
				Description: "test workspace",
				MountPath:   "/workspace/test/file",
				ReadOnly:    true,
			}},
		},
	}
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineName, Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
				Name: "foo",
			}},
			Tasks: []v1beta1.PipelineTask{{
				Name:    "task1",
				TaskRef: &v1beta1.TaskRef{Name: taskName},
				Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
					Name:      "test",
					Workspace: "foo",
				}},
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
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	if err := WaitForPipelineRunState(c, pipelineRunName, 10*time.Second, FailedWithMessage(`pipeline expects workspace with name "foo" be provided by pipelinerun`, pipelineRunName), "PipelineRunHasCondition"); err != nil {
		t.Fatalf("Failed to wait for PipelineRun %q to finish: %s", pipelineRunName, err)
	}
}
