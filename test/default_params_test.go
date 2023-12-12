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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestDefaultParamReferencedInPipeline_Success(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	pipelineName := helpers.ObjectNameForTest(t)
	examplePipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: hello
      properties:
        project_id:
          type: string
      type: object
  tasks:
    - name: task1
      params:
      - name: foo
        value:
          project_id: $(params.hello.project_id)
      taskSpec:
        params:
          - name: foo
            type: object
            properties:
              project_id:
                type: string
        steps:
          - name: echo
            image: ubuntu
            script: |
              #!/bin/bash
              echo $(params.provider.project_id)
              echo $(params.foo.project_id)
`, pipelineName, namespace))

	_, err := c.V1PipelineClient.Create(ctx, examplePipeline, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineName, err)
	}

	prName := helpers.ObjectNameForTest(t)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: provider
      value:
        project_id: foo
    - name: hello
      value:
        project_id: foo
  pipelineRef:
    resolver: cluster
    params:
    - name: kind
      value: pipeline
    - name: name
      value: %s
    - name: namespace
      value: %s
`, prName, namespace, pipelineName, namespace))

	_, err = c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
}

func TestDefaultParamReferencedInTask_Success(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	exampleTask := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: foo
      type: object
      properties:
        project_id:
          type: string
  steps:
    - image: ubuntu
      name: echo
      script: |
        echo -n $(params.foo.project_id)
        echo -n $(params.provider.project_id)
`, taskName, namespace))

	_, err := c.V1TaskClient.Create(ctx, exampleTask, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	trName := helpers.ObjectNameForTest(t)

	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: provider
      value:
        project_id: foo
    - name: foo
      value:
        project_id: foo
  taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: %s
    - name: namespace
      value: %s
`, trName, namespace, taskName, namespace))

	_, err = c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", trName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", trName, err)
	}
}

func TestDefaultParamReferencedInPipeline_Failure(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	pipelineName := helpers.ObjectNameForTest(t)
	examplePipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: hello
      properties:
        project_id:
          type: string
      type: object
  tasks:
    - name: task1
      params:
      - name: foo
        value:
          project_id: $(params.hello.project_id)
      taskSpec:
        params:
          - name: foo
            type: object
            properties:
              project_id:
                type: string
        steps:
          - name: echo
            image: ubuntu
            script: |
              #!/bin/bash
              echo $(params.provider.not_existent)
              echo $(params.foo.project_id)
`, pipelineName, namespace))

	_, err := c.V1PipelineClient.Create(ctx, examplePipeline, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineName, err)
	}

	prName := helpers.ObjectNameForTest(t)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: provider
      value:
        project_id: foo
    - name: hello
      value:
        project_id: foo
  pipelineRef:
    resolver: cluster
    params:
    - name: kind
      value: pipeline
    - name: name
      value: %s
    - name: namespace
      value: %s
`, prName, namespace, pipelineName, namespace))

	_, err = c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout,
		Chain(
			FailedWithReason(v1.PipelineRunReasonParamKeyNotExistent.String(), prName),
		), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish with expected error: %s", err)
	}
}

func TestDefaultParamReferencedInTask_Failure(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	exampleTask := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: foo
      type: object
      properties:
        project_id:
          type: string
  steps:
    - image: ubuntu
      name: echo
      script: |
        echo -n $(params.foo.project_id)
        echo -n $(params.provider.not-existent)
`, taskName, namespace))

	_, err := c.V1TaskClient.Create(ctx, exampleTask, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	trName := helpers.ObjectNameForTest(t)

	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: provider
      value:
        project_id: foo
    - name: foo
      value:
        project_id: foo
  taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: %s
    - name: namespace
      value: %s
`, trName, namespace, taskName, namespace))

	_, err = c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", trName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
	if err := WaitForTaskRunState(ctx, c, trName,
		Chain(
			FailedWithReason(v1.TaskRunReasonParamKeyNotExistent.String(), trName),
		), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to finish with expected error: %s", err)
	}
}
