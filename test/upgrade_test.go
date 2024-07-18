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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var (
	filterV1SidecarState          = cmpopts.IgnoreFields(v1.SidecarState{}, "ImageID")
	filterVolumeMountsName        = cmpopts.IgnoreFields(corev1.VolumeMount{}, "Name")
	filterTaskRunStatusFields     = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime", "CompletionTime", "Provenance", "PodName")
	filterPipelineRunStatusFields = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "StartTime", "CompletionTime", "Provenance")

	simpleTaskYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: rsp
    type: string
    default: "response"
  steps:
  - name: echo-param
    image: docker.io/library/alpine:3.20.1
    script: |
      echo "$(params.rsp)"
  - name: check-workspace
    image: docker.io/library/alpine:3.20.1
    script: |
      if [ "$(workspaces.taskWorkspace.bound)" == "true" ]; then
        echo "Workspace provided"
      fi
  workspaces:
  - name: taskWorkspace
`

	simplePipelineYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: %s
    taskRef:
      name: %s
    workspaces:
    - name: taskWorkspace
      workspace: workspace
  workspaces:
    - name: workspace
`

	simpleTaskRunYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
    kind: Task
  timeout: 30m
  workspaces:
    - name: taskWorkspace
      emptyDir: {}
`

	simplePipelineRunYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  workspaces:
    - name: workspace
      emptyDir: {}
  timeouts:
    pipeline: 180s
`

	expectedSimpleTaskRunYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
    kind: Task
  timeout: 30m
  workspaces:
    - name: taskWorkspace
      emptyDir: {}
status:
  conditions:
  - type: Succeeded
    reason: Succeeded
    status: "True"
  taskSpec:
    steps:
    - image: docker.io/library/alpine:3.20.1
      name: echo-param
      script: |
       echo "response"
    - name: check-workspace
      image: docker.io/library/alpine:3.20.1
      script: |
        if [ "true" == "true" ]; then
          echo "Workspace provided"
        fi
    workspaces:
    - name: taskWorkspace
    params:
    - name: rsp
      type: string
      default: response
  steps:
  - container: step-echo
    name: step-echo
    terminationReason: Completed
    terminated:
      reason: Completed
  - container: check-workspace
    name: check-workspace
    terminationReason: Completed
    terminated:
      reason: Completed
`

	expectedSimplePipelineRunYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  timeouts:
    pipeline: 3m
  pipelineRef:
    name: %s
    kind: Pipeline
  workspaces:
    - name: workspace
      emptyDir: {}
status:
  conditions:
  - type: Succeeded
    reason: Succeeded
    status: "True"
  pipelineSpec:
    tasks:
    - name: task1
      taskRef:
        name: task1
        kind: Task
      workspaces:
      - name: taskWorkspace
        workspace: workspace
    workspaces:
    - name: workspace
      emptyDir: {}
  childReferences:
  - name: %s
    pipelineTaskName: task1
    apiVersion: "tekton.dev/v1"
    kind: TaskRun
`
)

// TestSimpleTaskRun creates a taskRun with a basic task and verifies the
// runs created are successful and as expected.
func TestSimpleTaskRun(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, namespace := setup(ctx, t)

	taskRunName := helpers.ObjectNameForTest(t)

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	t.Logf("Creating Task in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(simpleTaskYaml, task1Name, namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}

	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(simpleTaskRunYaml, taskRunName, namespace, task1Name))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", taskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", taskRunName, namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", taskRunName, err)
	}

	r, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	expectedTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(expectedSimpleTaskRunYaml, taskRunName, namespace, task1Name))
	if d := cmp.Diff(expectedTaskRun, r, append([]cmp.Option{filterV1TaskRunSA, filterV1StepState, filterV1TaskRunStatus, filterV1SidecarState, filterVolumeMountsName, filterTaskRunStatusFields, cmpopts.EquateEmpty()}, filterV1TaskRunFields...)...); d != "" {
		t.Errorf("Cannot get expected TaskRun, -want, +got: %v", d)
	}
}

// TestSimplePipelineRun creates a pipelineRun with a basic Pipeline
// and verifies the runs created are successful and as expected.
func TestSimplePipelineRun(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	t.Logf("Creating Task in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(simpleTaskYaml, task1Name, namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(simplePipelineYaml, helpers.ObjectNameForTest(t), namespace, task1Name, task.Name))
	pipelineName := pipeline.Name
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(simplePipelineRunYaml, helpers.ObjectNameForTest(t), namespace, pipeline.Name))
	pipelineRunName := pipelineRun.Name

	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRunName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", pipelineRunName, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, timeout, PipelineRunSucceed(pipelineRunName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", pipelineRunName, err)
	}

	taskRunName := strings.Join([]string{pipelineRunName, task1Name}, "-")

	pr, err := c.V1PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun %s: %s", pipelineRunName, err)
	}

	expectedPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(expectedSimplePipelineRunYaml, pipelineRunName, namespace, pipelineName, taskRunName))
	if d := cmp.Diff(expectedPipelineRun, pr, append([]cmp.Option{filterV1PipelineRunStatus, filterV1PipelineRunSA, filterPipelineRunStatusFields}, filterV1PipelineRunFields...)...); d != "" {
		t.Errorf("Cannot get expected PipelineRun, -want, +got: %v", d)
	}
}
