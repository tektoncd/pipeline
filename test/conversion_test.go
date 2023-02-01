//go:build e2e
// +build e2e

/*
Copyright 2022 The Tekton Authors
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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var (
	ReleaseAnnotation = "pipeline.tekton.dev/release"

	// release Annotation is ignored when populated by TaskRuns
	ignoreReleaseAnnotation = func(k string, v string) bool {
		return k == ReleaseAnnotation
	}

	filterLabels                   = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Labels")
	filterV1TaskRunStatus          = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime", "CompletionTime")
	filterV1PipelineRunStatus      = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")
	filterV1beta1TaskRunStatus     = cmpopts.IgnoreFields(v1beta1.TaskRunStatusFields{}, "StartTime", "CompletionTime")
	filterV1beta1PipelineRunStatus = cmpopts.IgnoreFields(v1beta1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")
	filterContainerStateTerminated = cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID", "Message")
	filterV1StepState              = cmpopts.IgnoreFields(v1.StepState{}, "Name", "ImageID", "Container")
	filterV1beta1StepState         = cmpopts.IgnoreFields(v1beta1.StepState{}, "Name", "ImageID", "ContainerName")
	filterReleaseAnnotation        = cmpopts.IgnoreMapEntries(ignoreReleaseAnnotation)

	filterMetadata                 = []cmp.Option{filterTypeMeta, filterObjectMeta}
	filterV1TaskRunFields          = []cmp.Option{filterTypeMeta, filterObjectMeta, filterLabels, filterCondition, filterReleaseAnnotation, filterV1TaskRunStatus, filterContainerStateTerminated, filterV1StepState}
	filterV1beta1TaskRunFields     = []cmp.Option{filterTypeMeta, filterObjectMeta, filterLabels, filterV1beta1TaskRunStatus, filterCondition, filterReleaseAnnotation, filterContainerStateTerminated, filterV1beta1StepState}
	filterV1PipelineRunFields      = []cmp.Option{filterTypeMeta, filterObjectMeta, filterLabels, filterCondition, filterV1PipelineRunStatus}
	filterV1beta1PipelineRunFields = []cmp.Option{filterTypeMeta, filterObjectMeta, filterLabels, filterCondition, filterV1beta1PipelineRunStatus, filterV1beta1TaskRunStatus, filterV1beta1StepState, filterContainerStateTerminated}

	v1beta1TaskYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: step
    image: gcr.io/google.com/cloudsdktool/cloud-sdk:alpine
    command: ['/bin/bash']
    args: ['-c', 'gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key']
    workingDir: /dir
    env:
    - name: MY_VAR1
      value: foo
    resources:
      inputs:
      - name: workspace
        resource: source-repo
      outputs:
      - name: workspace
        resource: source-repo
    volumeMounts:
      - name: messages
        mountPath: /messages
    imagePullPolicy: IfNotPresent
    securityContext: 
      runAsUser: 2000
    timeout: 60s
    secret:
      secretName: test-ssh-credentials
    onError: continue
  stepTemplate:
    image: gcr.io/google.com/cloudsdktool/cloud-sdk:alpine
    command: ['/bin/bash']
    env:
    - name: QUX
      value: original
    args: ['-c', 'gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key']
    workingDir: /dir
    env:
    - name: MY_VAR1
      value: foo
    resources:
      inputs:
      - name: workspaces
        resource: source-repo
      outputs:
      - name: workspace
        resource: source-repo
    volumeMounts:
    - name: messages
      mountPath: /messages
    imagePullPolicy: IfNotPresent
    securityContext: 
        runAsUser: 2000
  sidecars:
  - name: server
    image: alpine/git:v2.26.2
    command: ['/bin/bash']
    args: ['-c', 'gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key']
    workingDir: /dir
    env:
    - name: MY_VAR1
      value: foo
    resources:
      inputs:
      - name: workspace
        resource: source-repo
      outputs:
      - name: workspace
        resource: source-repo
    volumeMounts:
    - name: messages
      mountPath: /messages
    readinessProbe:
      periodSeconds: 1
    securityContext:
      runAsUser: 0
    volumeMounts:
    - name: messages
      mountPath: /messages
    script: echo test
  volumes:
  - name: messages
    emptyDir: {}
  params:
  - name: PARAM
    description: param des
    type: string
    default: "1"
  workspaces:
  - name: workspace
    description: description
    mountPath: /foo
    readOnly: true
    optional: true
`

	v1TaskYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: step
    image: gcr.io/google.com/cloudsdktool/cloud-sdk:alpine
    command: ['/bin/bash']
    args: ['-c', 'gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key']
    workingDir: /dir
    env:
    - name: MY_VAR1
      value: foo
    resources:
      inputs:
      - name: workspace
        resource: source-repo
      outputs:
      - name: workspace
        resource: source-repo
    volumeMounts:
      - name: messages
        mountPath: /messages
    imagePullPolicy: IfNotPresent
    securityContext: 
      runAsUser: 2000
    timeout: 60s
    secret:
      secretName: test-ssh-credentials
    onError: continue
  stepTemplate:
    image: gcr.io/google.com/cloudsdktool/cloud-sdk:alpine
    command: ['/bin/bash']
    env:
    - name: QUX
      value: original
    args: ['-c', 'gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key']
    workingDir: /dir
    env:
    - name: MY_VAR1
      value: foo
    volumeMounts:
    - name: messages
      mountPath: /messages
    imagePullPolicy: IfNotPresent
    securityContext: 
        runAsUser: 2000
  sidecars:
  - name: server
    image: alpine/git:v2.26.2
    command: ['/bin/bash']
    args: ['-c', 'gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key']
    workingDir: /dir
    env:
    - name: MY_VAR1
      value: foo
    resources:
      inputs:
      - name: workspace
        resource: source-repo
      outputs:
      - name: workspace
        resource: source-repo
    readinessProbe:
      periodSeconds: 1
    volumeMounts:
    - name: messages
      mountPath: /messages
    securityContext:
      runAsUser: 0
    script: echo test
  volumes:
    - name: messages
      emptyDir: {}
  params:
  - name: PARAM
    description: param des
    type: string
    default: "1"
  workspaces:
  - name: workspace
    description: description
    mountPath: /foo
    readOnly: true
    optional: true
`

	v1beta1PipelineYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  description: foo
  tasks:
  - name: generate-result
    taskRef:
      kind: Task
      name: generate-result
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  serviceAccountName: default
  timeouts:
    pipeline: 1h30m
    tasks: 1h15m
  taskSpec:
    params:
      - name: task1-result
        value: task1-val
    steps:
      - image: alpine
        onError: continue
        name: exit-with-255
        script: |
          exit 255
  timeout: 60s
  podTemplate:
    securityContext:
      fsGroup: 65532
  workspaces:
  - name: password-vault
  finally:
  - name: echo-status
    params:
      - name: echoStatus
        value: "status"
    taskSpec:
      params:
        - name: echoStatus
          type: string
      steps:
        - name: verify-status
          image: ubuntu
          script: |
            if [ $(params.echoStatus) == "Succeeded" ]
            then
              echo " Good night! echoed successfully"
            fi
`

	v1PipelineYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  description: foo
  tasks:
  - name: generate-result
    taskRef:
      kind: Task
      name: generate-result
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  serviceAccountName: default
  timeouts:
    pipeline: 1h30m
    tasks: 1h15m
  taskSpec:
    params:
      - name: task1-result
        value: task1-val
    steps:
      - image: alpine
        onError: continue
        name: exit-with-255
        script: |
          exit 255
  timeout: 60s
  podTemplate:
    securityContext:
      fsGroup: 65532
  workspaces:
  - name: password-vault
  finally:
  - name: echo-status
    params:
      - name: echoStatus
        value: "status"
    taskSpec:
      params:
        - name: echoStatus
          type: string
      steps:
        - name: verify-status
          image: ubuntu
          script: |
            if [ $(params.echoStatus) == "Succeeded" ]
            then
              echo " Good night! echoed successfully"
            fi
`

	v1beta1TaskRunYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  serviceAccountName: default
  taskSpec:
    steps:
      - name: echo
        image: ubuntu
        script: |
          #!/usr/bin/env bash
          echo "Hello World!"
    workspaces:
    - name: output
  timeout: 20s
  workspaces:
    - name: output
      emptyDir: {}
  podTemplate:
    securityContext:
      fsGroup: 65532
`

	v1beta1TaskRunExpectedYaml = `
metadata:
  name: %s
  namespace: %s
  annotations: {}
spec:
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  serviceAccountName: default
  timeout: 60s
  podTemplate:
    securityContext:
      fsGroup: 65532
  taskSpec:
    steps:
    - computeResources: {}
      image: ubuntu
      name: echo
      script: |
        #!/usr/bin/env bash
        echo "Hello World!"
    workspaces:
    - name: output
  workspaces:
  - emptyDir: {}
    name: output   
status:
  conditions:
  - reason: Succeeded
    status: "True"
    type: Succeeded
  podName: %s-pod
  taskSpec:
    steps:
    - computeResources: {}
      image: ubuntu
      name: echo
      script: |
        #!/usr/bin/env bash
        echo "Hello World!"
    workspaces:
    - name: output
  workspaces:
    - name: output
  steps:
  - container: echo
    name: echo
    terminated:
      reason: Completed
`

	v1TaskRunYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  serviceAccountName: default
  timeout: 60s
  podTemplate:
    securityContext:
      fsGroup: 65532
  workspaces:
  - emptyDir: {}
    name: output     
  taskSpec:
    steps:
    - computeResources: {}
      image: ubuntu
      name: echo
      script: |
        #!/usr/bin/env bash
        echo "Hello World!"
    workspaces:
    - name: output
`

	v1TaskRunExpectedYaml = `
metadata:
  name: %s
  namespace: %s
  annotations: {}
spec:
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  serviceAccountName: default
  timeout: 20s
  podTemplate:
    securityContext:
      fsGroup: 65532
  workspaces:
    - emptyDir: {}
      name: output 
  taskSpec:
    steps:
    - computeResources: {}
      image: ubuntu
      name: echo
      script: |
        #!/usr/bin/env bash
        echo "Hello World!"
    workspaces:
    - name: output
status:
  conditions:
  - reason: Succeeded
    status: "True"
    type: Succeeded
  podName: %s-pod
  taskSpec:
    steps:
    - computeResources: {}
      image: ubuntu
      name: echo
      script: |
        #!/usr/bin/env bash
        echo "Hello World!"
    workspaces:
    - name: output
  steps:
  - container: step-echo
    name: step-echo
    terminated:
      reason: Completed
`

	v1beta1PipelineRunYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  serviceAccountName: default
  workspaces:
  - name: password-vault
    secret:
      secretName: secret-password
  timeout: 60s
  pipelineSpec:
    tasks:
    - name: fetch-secure-data
      taskSpec:
        steps:
        - name: fetch-and-write-secure
          image: ubuntu
          script: echo hello
`

	v1beta1PipelineRunExpectedYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  timeouts:
    pipeline: 60s
  workspaces:
  - name: password-vault
    secret:
      secretName: secret-password
  serviceAccountName: default
  pipelineSpec:
    tasks:
    - name: fetch-secure-data
      taskSpec:
        steps:
        - name: fetch-and-write-secure
          image: ubuntu
          script: echo hello
status:
  conditions:
  - type: Succeeded
    status: "True"
    reason: "Succeeded"
  pipelineSpec:
    tasks:
    - name: fetch-secure-data
      taskSpec:
        name: cluster-task-pipeline-4
        steps:
        - name: "fetch-and-write-secure"
          image: "ubuntu"
          script: "echo hello"
  childReferences:
    - name: %s-fetch-secure-data
      pipelineTaskName: fetch-secure-data
      apiVersion: tekton.dev/v1beta1
      kind: TaskRun
`

	v1PipelineRunYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  pipelineSpec:
    tasks:
    - name: fetch-secure-data
      taskSpec:
        steps:
        - name: fetch-and-write-secure
          image: ubuntu
          script: echo hello
  timeouts:
    pipeline: 60s
  workspaces:
  - name: password-vault
    secret:
      secretName: secret-password
  taskRunTemplate: {
    serviceAccountName: default
  }
`

	v1PipelineRunExpectedYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: STRING_LENGTH
    value: 1
    type: string
  pipelineSpec:
    tasks:
    - name: fetch-secure-data
      taskSpec:
        steps:
        - name: fetch-and-write-secure
          image: ubuntu
          script: echo hello
  timeouts:
    pipeline: 60s
  workspaces:
  - name: password-vault
    secret:
      secretName: secret-password
  taskRunTemplate: {
    serviceAccountName: default
  }
status:
  conditions:
  - type: Succeeded
    status: "True"
    reason: "Succeeded"
  pipelineSpec:
    tasks:
    - name: fetch-secure-data
      taskSpec:
        name: cluster-task-pipeline-4
        steps:
        - name: "fetch-and-write-secure"
          image: "ubuntu"
          script: "echo hello"
  childReferences:
    - apiVersion: tekton.dev/v1beta1
      kind: TaskRun
      name: %s-fetch-secure-data
      pipelineTaskName: fetch-secure-data
`
)

// TestTaskCRDConversion first creates a v1beta1 Task CRD using v1beta1Clients and
// requests it by v1Clients to compare with v1 if the conversion has been
// correctly executed by the webhook. And then it creates the v1 Task CRD using v1Clients
// and requests it by v1beta1Clients to compare with v1beta1.
func TestTaskCRDConversion(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	t.Parallel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	v1beta1TaskName := helpers.ObjectNameForTest(t)
	v1beta1Task := parse.MustParseV1beta1Task(t, fmt.Sprintf(v1beta1TaskYaml, v1beta1TaskName, namespace))
	v1TaskExpected := parse.MustParseV1Task(t, fmt.Sprintf(v1TaskYaml, v1beta1TaskName, namespace))

	if _, err := c.V1beta1TaskClient.Create(ctx, v1beta1Task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1beta1 Task: %s", err)
	}
	v1TaskGot, err := c.V1TaskClient.Get(ctx, v1beta1TaskName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1 Task for %s: %s", v1beta1TaskName, err)
	}

	if d := cmp.Diff(v1TaskExpected, v1TaskGot, filterMetadata...); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	v1TaskName := helpers.ObjectNameForTest(t)
	v1Task := parse.MustParseV1Task(t, fmt.Sprintf(v1TaskYaml, v1TaskName, namespace))
	v1beta1TaskExpected := parse.MustParseV1beta1Task(t, fmt.Sprintf(v1beta1TaskYaml, v1TaskName, namespace))

	if _, err := c.V1TaskClient.Create(ctx, v1Task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1beta1 Task: %s", err)
	}

	v1beta1TaskGot, err := c.V1beta1TaskClient.Get(ctx, v1TaskName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1beta1 Task for %s: %s", v1TaskName, err)
	}

	if d := cmp.Diff(v1beta1TaskExpected, v1beta1TaskGot, filterMetadata...); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

// TestTaskRunCRDConversion first creates a v1beta1 TaskRun CRD using v1beta1Clients
// and requests it by v1Clients to compare with v1 if the conversion has been correctly
// executed by the webhook. And then it creates the v1 TaskRun CRD using v1Clients
// and requests it by v1beta1Clients to compare with v1beta1.
func TestTaskRunCRDConversion(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	t.Parallel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	v1beta1TaskRunName := helpers.ObjectNameForTest(t)
	v1beta1TaskRun := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(v1beta1TaskRunYaml, v1beta1TaskRunName, namespace))
	v1TaskRunExpected := parse.MustParseV1TaskRun(t, fmt.Sprintf(v1TaskRunExpectedYaml, v1beta1TaskRunName, namespace, v1beta1TaskRunName))

	if _, err := c.V1beta1TaskRunClient.Create(ctx, v1beta1TaskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1beta1 TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, v1beta1TaskRunName, Succeed(v1beta1TaskRunName), v1beta1TaskRunName, "v1beta1"); err != nil {
		t.Fatalf("Failed waiting for v1beta1 TaskRun done: %v", err)
	}

	v1TaskRunGot, err := c.V1TaskRunClient.Get(ctx, v1beta1TaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1 TaskRun for %s: %s", v1beta1TaskRunName, err)
	}

	if d := cmp.Diff(v1TaskRunExpected, v1TaskRunGot, filterV1TaskRunFields...); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	v1TaskRunName := helpers.ObjectNameForTest(t)
	v1TaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(v1TaskRunYaml, v1TaskRunName, namespace))
	v1beta1TaskRunExpected := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(v1beta1TaskRunExpectedYaml, v1TaskRunName, namespace, v1TaskRunName))

	if _, err := c.V1TaskRunClient.Create(ctx, v1TaskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1 TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, v1TaskRunName, Succeed(v1TaskRunName), v1TaskRunName, "v1"); err != nil {
		t.Fatalf("Failed waiting for v1 TaskRun done: %v", err)
	}

	v1beta1TaskRunGot, err := c.V1beta1TaskRunClient.Get(ctx, v1TaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1beta1 TaskRun for %s: %s", v1TaskRunName, err)
	}

	if d := cmp.Diff(v1beta1TaskRunExpected, v1beta1TaskRunGot, filterV1beta1TaskRunFields...); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

// TestPipelineCRDConversion first creates a v1beta1 Pipeline CRD using v1beta1Clients and
// requests it by v1Clients to compare with v1 if the conversion has been
// correctly executed by the webhook. And then it creates the v1 Pipeline CRD using v1Clients
// and requests it by v1beta1Clients to compare with v1beta1.
func TestPipelineCRDConversion(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	t.Parallel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	v1beta1PipelineName := helpers.ObjectNameForTest(t)
	v1beta1Pipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(v1beta1PipelineYaml, v1beta1PipelineName, namespace))
	v1PipelineExpected := parse.MustParseV1Pipeline(t, fmt.Sprintf(v1PipelineYaml, v1beta1PipelineName, namespace))

	if _, err := c.V1beta1PipelineClient.Create(ctx, v1beta1Pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1beta1 Pipeline: %s", err)
	}

	v1PipelineGot, err := c.V1PipelineClient.Get(ctx, v1beta1PipelineName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1 Pipeline for %s: %s", v1beta1PipelineName, err)
	}

	if d := cmp.Diff(v1PipelineGot, v1PipelineExpected, filterMetadata...); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	v1PipelineName := helpers.ObjectNameForTest(t)
	v1Pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(v1PipelineYaml, v1PipelineName, namespace))
	v1beta1PipelineExpected := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(v1beta1PipelineYaml, v1PipelineName, namespace))

	if _, err := c.V1PipelineClient.Create(ctx, v1Pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1 Pipeline: %s", err)
	}

	v1beta1PipelineGot, err := c.V1beta1PipelineClient.Get(ctx, v1PipelineName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1beta1 Pipeline for %s: %s", v1PipelineName, err)
	}

	if d := cmp.Diff(v1beta1PipelineExpected, v1beta1PipelineGot, filterMetadata...); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

// TestPipelineRunCRDConversion first creates a v1beta1 PipelineRun CRD using v1beta1Clients and
// requests it by v1Clients to compare with v1 if the conversion has been
// correctly executed by the webhook. And then it creates the v1 PipelineRun CRD using v1Clients
// and requests it by v1beta1Clients to compare with v1beta1.
func TestPipelineRunCRDConversion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Parallel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	v1beta1ToV1PipelineRunName := helpers.ObjectNameForTest(t)
	v1beta1PipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunYaml, v1beta1ToV1PipelineRunName, namespace))
	v1PipelineRunExpected := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunExpectedYaml, v1beta1ToV1PipelineRunName, namespace, v1beta1ToV1PipelineRunName))

	if _, err := c.V1beta1PipelineRunClient.Create(ctx, v1beta1PipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1beta1 PipelineRun: %s", err)
	}

	if err := WaitForPipelineRunState(ctx, c, v1beta1ToV1PipelineRunName, timeout, Succeed(v1beta1ToV1PipelineRunName), v1beta1ToV1PipelineRunName, "v1beta1"); err != nil {
		t.Fatalf("Failed waiting for v1beta1 PipelineRun done: %v", err)
	}

	v1PipelineRunGot, err := c.V1PipelineRunClient.Get(ctx, v1beta1ToV1PipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1 PipelineRun for %s: %s", v1beta1ToV1PipelineRunName, err)
	}

	if d := cmp.Diff(v1PipelineRunExpected, v1PipelineRunGot, filterV1PipelineRunFields...); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	v1ToV1beta1PRName := helpers.ObjectNameForTest(t)
	v1PipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunYaml, v1ToV1beta1PRName, namespace))
	v1beta1PipelineRunExpected := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunExpectedYaml, v1ToV1beta1PRName, namespace, v1ToV1beta1PRName))

	if _, err := c.V1PipelineRunClient.Create(ctx, v1PipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1 PipelineRun: %s", err)
	}

	if err := WaitForPipelineRunState(ctx, c, v1ToV1beta1PRName, timeout, Succeed(v1ToV1beta1PRName), v1ToV1beta1PRName, "v1"); err != nil {
		t.Fatalf("Failed waiting for v1 pipelineRun done: %v", err)
	}

	v1beta1PipelineRunGot, err := c.V1beta1PipelineRunClient.Get(ctx, v1ToV1beta1PRName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1beta1 PipelineRun for %s: %s", v1ToV1beta1PRName, err)
	}

	if d := cmp.Diff(v1beta1PipelineRunExpected, v1beta1PipelineRunGot, filterV1beta1PipelineRunFields...); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}
