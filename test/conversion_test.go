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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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
	ReleaseAnnotation     = "pipeline.tekton.dev/release"
	TaskRunsAnnotationKey = "tekton.dev/v1beta1TaskRuns"
	RunsAnnotationKey     = "tekton.dev/v1beta1Runs"

	ignoreUnpredictableStrings = func(k, v interface{}) bool {
		return k == TaskRunsAnnotationKey || k == RunsAnnotationKey
	}

	// release Annotation is ignored because it is populated by
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
	filterAnnotationsStrings       = cmpopts.IgnoreMapEntries(ignoreUnpredictableStrings)
	filterReleaseAnnotation        = cmpopts.IgnoreMapEntries(ignoreReleaseAnnotation)

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
  resources:
    inputs:
      - name: git-repo
        type: git
        description: "The input is code from a git repository"
        optional: true
    outputs:
      - name: optionalimage
        type: image
        description: "The output is a Docker image"
        optional: true
`

	v1TaskYaml = `
metadata:
  name: %s
  namespace: %s
  annotations: {
    tekton.dev/v1beta1Resources: '{"inputs":[{"name":"git-repo","type":"git","description":"The input is code from a git repository","optional":true}],"outputs":[{"name":"optionalimage","type":"image","description":"The output is a Docker image","optional":true}]}'
  }
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
  resources:
  - name: source-repo
    type: git    
`

	v1PipelineYaml = `
metadata:
  name: %s
  namespace: %s
  annotations: {
    tekton.dev/v1beta1Resources: '[{"name":"source-repo","type":"git"}]'
  }
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
    resources:
      inputs:
      - name: skaffold
        type: git
      outputs:
      - name: skaffoldout
        type: git
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
  resources:
    inputs:
    - name: skaffold
      resourceSpec:
        type: git
        params:
          - name: revision
            value: v0.32.0
          - name: url
            value: https://github.com/GoogleContainerTools/skaffold
    outputs:
    - name: skaffoldout
      resourceSpec:
        type: git
        params:
          - name: revision
            value: v0.32.0
          - name: url
            value: https://github.com/GoogleContainerTools/skaffold
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
  annotations: {
    tekton.dev/v1beta1Resources: '{"inputs":[{"name":"skaffold","resourceSpec":{"type":"git","params":[{"name":"revision","value":"v0.32.0"},{"name":"url","value":"https://github.com/GoogleContainerTools/skaffold"}]}}],"outputs":[{"name":"skaffoldout","resourceSpec":{"type":"git","params":[{"name":"revision","value":"v0.32.0"},{"name":"url","value":"https://github.com/GoogleContainerTools/skaffold"}]}}]}',
    tekton.dev/v1beta1ResourcesResult: '[{"key":"commit","value":"6ed7aad5e8a36052ee5f6079fc91368e362121f7","resourceName":"skaffold"},{"key":"url","value":"https://github.com/GoogleContainerTools/skaffold","resourceName":"skaffold"}]',
  }
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
  - container: step-create-dir-skaffoldout
    name: create-dir-skaffoldout
    terminated:
      reason: Completed
  - container: git-source-skaffold
    name: git-source-skaffold
    terminated:
      reason: Completed
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
    resources:
    - name: pipeline-git
      type: git
  resources:
    - name: pipeline-git
      resourceSpec:
        type: git
        params:
          - name: revision
            value: main
          - name: url
            value: https://github.com/tektoncd/pipeline
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
  taskRuns:
    %s-fetch-secure-data:
      pipelineTaskName: fetch-secure-data
      status:
        conditions:
        - reason: Succeeded
          status: "True"
          type: Succeeded
        podName: %s-fetch-secure-data-pod
        steps:
        - container: step-fetch-and-write-secure
          imageID: docker.io/library/ubuntu@sha256:4b1d0c4a2d2aaf63b37111f34eb9fa89fa1bf53dd6e4ca954d47caebca4005c2
          name: fetch-and-write-secure
          terminated:
            containerID: containerd://07b57fc6fd515e6d1d0de27149c60be9149697207aedc130dd0d284fce6df3fd
            exitCode: 0
            finishedAt: "2022-12-07T15:56:32Z"
            reason: Completed
            startedAt: "2022-12-07T15:56:32Z"
        taskSpec:
          steps:
          - name: "fetch-and-write-secure"
            image: "ubuntu"
            script: "echo hello"
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
  annotations: {
    tekton.dev/v1beta1Resources: '[{"name":"pipeline-git","resourceSpec":{"type":"git","params":[{"name":"revision","value":"main"},{"name":"url","value":"https://github.com/tektoncd/pipeline"}]}}]',
    tekton.dev/v1beta1TaskRuns: '{"%s-fetch-secure-data":{"pipelineTaskName":"fetch-secure-data","status":{"conditions":[{"type":"Succeeded","status":"True","reason":"Succeeded","message":"All Steps have completed executing"}],"podName":"%s-fetch-secure-data-pod","steps":[{"terminated":{"exitCode":0,"reason":"Completed","containerID":"containerd://978666d35ed0e20f370227a0a4c3048ef6ec59a3ad5a2f5d83a3210099a9108f"},"name":"fetch-and-write-secure","container":"step-fetch-and-write-secure","imageID":"docker.io/library/ubuntu@sha256:4b1d0c4a2d2aaf63b37111f34eb9fa89fa1bf53dd6e4ca954d47caebca4005c2"}],"taskSpec":{"steps":[{"name":"fetch-and-write-secure","image":"ubuntu","resources":{},"script":"echo hello"}]}}}}',
  }
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
    - typeMeta:
      kind: TaskRun
      apiVersion: v1beta1
      name: %s-fetch-secure-data
      pipelineTaskName: fetch-secure-data
`

	// Alpha CRD examples serve for the purpose of testing those fields that
	// requires the feature flags
	v1beta1TaskRunYamlAlpha = `
metadata:
  name: %s
  namespace: %s
spec:
  timeout: 60s
  serviceAccountName: default
  taskRef:
    name: hello-world
    bundle: docker.io/ptasci67/example-oci@sha256:053a6cb9f3711d4527dd0d37ac610e8727ec0288a898d5dfbd79b25bcaa29828
`

	v1TaskRunYamlAlphaExpected = `
metadata:
  name: %s
  namespace: %s
  annotations: {}
spec:
  serviceAccountName: default
  taskRef:
    kind: Task
    params:
    - name: bundle
      value: docker.io/ptasci67/example-oci@sha256:053a6cb9f3711d4527dd0d37ac610e8727ec0288a898d5dfbd79b25bcaa29828
    - name: name
      value: hello-world
    - name: kind
      value: Task
    resolver: bundles
  timeout: 1m0s
status:
  conditions:
  - reason: Succeeded
    status: "True"
    type: Succeeded
  podName: %s-pod
  steps:
  - container: step
    terminated:
      reason: Completed
  taskSpec:
    steps:
    - computeResources: {}
      image: ubuntu
      name: hellp
      script: echo "Hello World!"
`

	v1TaskRunYamlAlpha = `
metadata:
  name: %s
  namespace: %s
spec:
  workspaces:
    - name: output
      emptyDir: {}
  podTemplate:
    securityContext:
      fsGroup: 65532
  taskRef:
    resolver: bundles
    params:
      - name: bundle
        value: gcr.io/tekton-releases/catalog/upstream/git-clone@sha256:8e2c3fb0f719d6463e950f3e44965aa314e69b800833e29e68ba2616bb82deeb
      - name: name
        value: git-clone
      - name: kind
        value: task
  params:
    - name: url
      value: https://github.com/kelseyhightower/nocode
    - name: revision
      value: master
`

	v1beta1TaskRunExpectedYamlAlpha = `
metadata:
  name: %s
  namespace: %s
spec:
  timeout: 1h
  serviceAccountName: default
  taskRef:
    resolver: "bundles"
    kind: Task
    params:
    - name: bundle
      value: gcr.io/tekton-releases/catalog/upstream/git-clone@sha256:8e2c3fb0f719d6463e950f3e44965aa314e69b800833e29e68ba2616bb82deeb
    - name: name
      value: git-clone
    - name: kind
      value: task
    resolver: bundles
  podTemplate:
    securityContext:
      fsGroup: 65532
  workspaces:
    - name: output
      emptyDir: {}
  params:
  - name: url
    value: https://github.com/kelseyhightower/nocode
  - name: revision
    value: master
status:
  conditions:
  - reason: Succeeded
    status: "True"
    type: Succeeded
  podName: %s-pod
  steps:
  - container: step-clone
    terminated:
      reason: Completed
  taskSpec:
    workspaces:
    - name: output
`

	v1beta1PipelineRunYamlAlpha = `
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: hello-world
    bundle: docker.io/ptasci67/example-oci@sha256:053a6cb9f3711d4527dd0d37ac610e8727ec0288a898d5dfbd79b25bcaa29828
`

	v1PipelineRunYamlAlphaExpected = `
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: "bundles"
    kind: Pipeline
    params:
    - name: bundle
      type: string
      value: "test-bundle"
    - name: name
      type: string
      value: pr
    - name: kind
      type: string
      value: Task
  timeouts:
    pipeline: 60s
  taskRunTemplate: {
    serviceAccountName: default
  }
`

	v1beta1PipelineRunExpectedYamlAlpha = `
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: "bundles"
    kind: Pipeline
    params:
    - name: bundle
      type: string
      value: "test-bundle"
    - name: name
      type: string
      value: pr
    - name: kind
      type: string
      value: Task
  timeouts:
    pipeline: 60s
  serviceAccountName: default
`

	v1PipelineRunYamlAlpha = `
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: "bundles"
    kind: Pipeline
    params:
    - name: bundle
      type: string
      value: "test-bundle"
    - name: name
      type: string
      value: pr
    - name: kind
      type: string
      value: Task
  timeouts:
    pipeline: 60s
  taskRunTemplate: {
    serviceAccountName: default
  }
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

	if d := cmp.Diff(v1TaskExpected, v1TaskGot, filterTypeMeta, filterObjectMeta); d != "" {
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

	if d := cmp.Diff(v1beta1TaskExpected, v1beta1TaskGot, filterTypeMeta, filterObjectMeta); d != "" {
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

	if d := cmp.Diff(v1TaskRunExpected, v1TaskRunGot, filterTypeMeta, filterObjectMeta, filterLabels, filterCondition, filterReleaseAnnotation, filterV1TaskRunStatus, filterContainerStateTerminated, filterV1StepState); d != "" {
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

	if d := cmp.Diff(v1beta1TaskRunExpected, v1beta1TaskRunGot, filterTypeMeta, filterObjectMeta, filterLabels, filterV1beta1TaskRunStatus, filterCondition, filterReleaseAnnotation, filterContainerStateTerminated, filterV1beta1StepState); d != "" {
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

	if d := cmp.Diff(v1PipelineGot, v1PipelineExpected, filterTypeMeta, filterObjectMeta); d != "" {
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

	// TODO:
	// Putting together we are ignoring....
	if d := cmp.Diff(v1beta1PipelineExpected, v1beta1PipelineGot, filterTypeMeta, filterObjectMeta); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

// TestPipelineRunCRDConversion first creates a v1beta1 PipelineRun CRD using v1beta1Clients and
// requests it by v1Clients to compare with v1 if the conversion has been
// correctly executed by the webhook. And then it creates the v1 PipelineRun CRD using v1Clients
// and requests it by v1beta1Clients to compare with v1beta1.
func TestPipelineRunCRDConversion(t *testing.T) {
	ctx := withFullEmbeddedStatus(context.Background())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	t.Parallel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	v1beta1ToV1PipelineRunName := helpers.ObjectNameForTest(t)
	v1beta1PipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunYaml, v1beta1ToV1PipelineRunName, namespace))
	v1PipelineRunExpected := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunExpectedYaml, v1beta1ToV1PipelineRunName, namespace, v1beta1ToV1PipelineRunName, v1beta1ToV1PipelineRunName, v1beta1ToV1PipelineRunName))

	if _, err := c.V1beta1PipelineRunClient.Create(ctx, v1beta1PipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1beta1 PipelineRun: %s", err)
	}

	if err := WaitForPipelineRunState(ctx, c, v1beta1ToV1PipelineRunName, timeout, Succeed(v1beta1ToV1PipelineRunName), v1beta1ToV1PipelineRunName, "v1beta1"); err != nil {
		t.Fatalf("Failed waiting for V1 PipelineRun done: %v", err)
	}

	v1PipelineRunGot, err := c.V1PipelineRunClient.Get(ctx, v1beta1ToV1PipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1 PipelineRun for %s: %s", v1beta1ToV1PipelineRunName, err)
	}

	if d := cmp.Diff(v1PipelineRunExpected, v1PipelineRunGot, filterTypeMeta, filterObjectMeta, filterLabels, filterCondition, filterV1PipelineRunStatus, filterAnnotationsStrings); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	v1ToV1beta1PRName := helpers.ObjectNameForTest(t)
	v1PipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunYaml, v1ToV1beta1PRName, namespace))
	v1beta1PipelineRunExpected := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunExpectedYaml, v1ToV1beta1PRName, namespace, v1ToV1beta1PRName, v1ToV1beta1PRName))

	if _, err := c.V1PipelineRunClient.Create(ctx, v1PipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1 PipelineRun: %s", err)
	}

	if err := WaitForPipelineRunState(ctx, c, v1ToV1beta1PRName, timeout, Succeed(v1ToV1beta1PRName), v1ToV1beta1PRName, "v1"); err != nil {
		t.Fatalf("Failed waiting for v1beta1 pipelineRun done: %v", err)
	}

	v1beta1PipelineRunGot, err := c.V1beta1PipelineRunClient.Get(ctx, v1ToV1beta1PRName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1beta1 PipelineRun for %s: %s", v1ToV1beta1PRName, err)
	}

	if d := cmp.Diff(v1beta1PipelineRunExpected, v1beta1PipelineRunGot, filterTypeMeta, filterObjectMeta, filterLabels, filterCondition, filterV1beta1PipelineRunStatus, filterV1beta1TaskRunStatus, filterContainerStateTerminated); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

// TestTaskRunCRDConversionAlpha is a duplicate of TestTaskRunfieldsCRDConversion that covers
// all alpha features included.
func TestTaskRunCRDConversionAlpha(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	t.Parallel()

	c, namespace := setup(ctx, t, requireAnyGate(supportedV1ConversionFeatureGates))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	v1beta1TaskRunName := helpers.ObjectNameForTest(t)
	v1beta1TaskRun := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(v1beta1TaskRunYamlAlpha, v1beta1TaskRunName, namespace))
	v1TaskRunExpected := parse.MustParseV1TaskRun(t, fmt.Sprintf(v1TaskRunYamlAlphaExpected, v1beta1TaskRunName, namespace, v1beta1TaskRunName))

	if _, err := c.V1beta1TaskRunClient.Create(ctx, v1beta1TaskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, v1beta1TaskRunName, Succeed(v1beta1TaskRunName), v1beta1TaskRunName, "v1beta1"); err != nil {
		t.Fatalf("Failed waiting for v1beta1 TaskRun done: %v", err)
	}

	v1TaskRunGot, err := c.V1TaskRunClient.Get(ctx, v1beta1TaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1 Task for %s: %s", v1beta1TaskRunName, err)
	}

	if d := cmp.Diff(v1TaskRunExpected, v1TaskRunGot, filterTypeMeta, filterObjectMeta, filterLabels, filterV1TaskRunStatus, filterReleaseAnnotation, filterCondition, filterContainerStateTerminated, filterV1StepState); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	// TODO: confirm if all taskSpec and results need to be tested

	v1TaskRunName := helpers.ObjectNameForTest(t)
	v1TaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(v1TaskRunYamlAlpha, v1TaskRunName, namespace))
	v1beta1TaskRunExpected := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(v1beta1TaskRunExpectedYamlAlpha, v1TaskRunName, namespace, v1TaskRunName))

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

	if d := cmp.Diff(v1beta1TaskRunExpected, v1beta1TaskRunGot, filterTypeMeta, filterObjectMeta, filterLabels, filterReleaseAnnotation, filterCondition, filterV1beta1TaskRunStatus); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

// TestPipelineRunCRDConversionAlpha is a duplicate of TestPipelineRunfieldsCRDConversion that covers
// all alpha features included.
func TestPipelineRunCRDConversionAlpha(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	t.Parallel()

	c, namespace := setup(ctx, t, requireAnyGate(supportedV1ConversionFeatureGates))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	v1beta1ToV1PipelineRunName := helpers.ObjectNameForTest(t)
	v1beta1PipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunYamlAlpha, v1beta1ToV1PipelineRunName, namespace))
	v1PipelineRunExpected := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunYamlAlpha, v1beta1ToV1PipelineRunName, namespace))

	if _, err := c.V1beta1PipelineRunClient.Create(ctx, v1beta1PipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	if err := WaitForPipelineRunState(ctx, c, v1beta1ToV1PipelineRunName, timeout, Succeed(v1beta1ToV1PipelineRunName), v1beta1ToV1PipelineRunName, "v1beta1"); err != nil {
		t.Fatalf("Failed waiting for V1 PipelineRun done: %v", err)
	}

	v1PipelineRunGot, err := c.V1PipelineRunClient.Get(ctx, v1beta1ToV1PipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected Task for %s: %s", v1beta1ToV1PipelineRunName, err)
	}

	if d := cmp.Diff(v1PipelineRunExpected, v1PipelineRunGot, filterTypeMeta, filterObjectMeta); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	v1ToV1beta1PRName := helpers.ObjectNameForTest(t)
	v1PipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunYamlAlpha, v1ToV1beta1PRName, namespace))
	v1beta1PipelineRunExpected := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunExpectedYamlAlpha, v1ToV1beta1PRName, namespace))

	if _, err := c.V1PipelineRunClient.Create(ctx, v1PipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create v1beta1 Task: %s", err)
	}

	if err := WaitForPipelineRunState(ctx, c, v1ToV1beta1PRName, timeout, Succeed(v1ToV1beta1PRName), v1ToV1beta1PRName, "v1"); err != nil {
		t.Fatalf("Failed waiting for v1beta1 pipelineRun done: %v", err)
	}

	v1beta1PipelineRunGot, err := c.V1beta1PipelineRunClient.Get(ctx, v1ToV1beta1PRName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected v1beta1 Task for %s: %s", v1ToV1beta1PRName, err)
	}

	if d := cmp.Diff(v1beta1PipelineRunExpected, v1beta1PipelineRunGot, filterTypeMeta, filterObjectMeta, filterLabels); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

func withFullEmbeddedStatus(ctx context.Context) context.Context {
	featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
		"embedded-status": config.FullEmbeddedStatus,
	})
	cfg := &config.Config{FeatureFlags: featureFlags}
	return config.ToContext(context.Background(), cfg)
}
