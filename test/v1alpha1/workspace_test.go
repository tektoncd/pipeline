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
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/test/parse"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestWorkspaceReadOnlyDisallowsWrite(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	taskName := "write-disallowed"
	taskRunName := "write-disallowed-tr"

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	task := parse.MustParseAlphaTask(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  steps:
  - image: alpine
    script: 'echo foo > /workspace/test/file'
  workspaces:
  - name: test
    description: 'test workspace'
    mountPath: /workspace/test
    readOnly: true
`, taskName))
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	taskRun := parse.MustParseAlphaTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  taskRef:
    name: %s
  serviceAccountName: default
  workspaces:
  - name: test
    emptyDir: {}
`, taskRunName, taskName))
	if _, err := c.TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunFailed(taskRunName), "error"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish with error: %s", err)
	}

	tr, err := c.TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}
	if tr.Status.PodName == "" {
		t.Fatal("Error getting a PodName (empty)")
	}
	p, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, tr.Status.PodName, metav1.GetOptions{})

	if err != nil {
		t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
	}
	for _, stat := range p.Status.ContainerStatuses {
		if strings.Contains(stat.Name, "step-attempt-write") {
			req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
			logContent, err := req.Do(ctx).Raw()
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
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	taskName := "read-workspace"
	pipelineName := "read-workspace-pipeline"
	pipelineRunName := "read-workspace-pipelinerun"

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	task := parse.MustParseAlphaTask(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  steps:
  - image: alpine
    script: 'cat /workspace/test/file'
  workspaces:
  - name: test
    description: 'test workspace'
    mountPath: /workspace/test
    readOnly: true
`, taskName))
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	pipeline := parse.MustParseAlphaPipeline(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  workspaces:
  - name: foo
  tasks:
  - name: %s
    taskRef:
      name: %s
    workspaces:
    - name: test
      workspace: foo
`, pipelineName, taskName, taskName))
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := parse.MustParseAlphaPipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    name: %s
  workspaces:
  - name: foo
    emptyDir: {}
  - name: foo
    emptyDir: {}
`, pipelineRunName, pipelineName))
	_, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})

	if err == nil || !strings.Contains(err.Error(), "provided by pipelinerun more than once") {
		t.Fatalf("Expected error when creating pipelinerun with duplicate workspace entries but received: %v", err)
	}
}

func TestWorkspacePipelineRunMissingWorkspaceInvalid(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	taskName := "read-workspace"
	pipelineName := "read-workspace-pipeline"
	pipelineRunName := "read-workspace-pipelinerun"

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	task := parse.MustParseAlphaTask(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  steps:
  - image: alpine
    script: 'cat /workspace/test/file'
  workspaces:
  - name: test
    description: 'test workspace'
    mountPath: /workspace/test/file
    readOnly: true
`, taskName))
	if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	pipeline := parse.MustParseAlphaPipeline(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  workspaces:
  - name: foo
  tasks:
  - name: %s
    taskRef:
      name: %s
    workspaces:
    - name: test
      workspace: foo
`, pipelineName, taskName, taskName))
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pipelineRun := parse.MustParseAlphaPipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    name: %s
`, pipelineRunName, pipelineName))
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, 10*time.Second, FailedWithMessage(`pipeline requires workspace with name "foo" be provided by pipelinerun`, pipelineRunName), "PipelineRunHasCondition"); err != nil {
		t.Fatalf("Failed to wait for PipelineRun %q to finish: %s", pipelineRunName, err)
	}

}
