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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var (
	filterLabels                      = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Labels")
	filterAnnotations                 = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Annotations")
	filterV1TaskRunStatus             = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime", "CompletionTime")
	filterV1PipelineRunStatus         = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")
	filterV1beta1TaskRunStatus        = cmpopts.IgnoreFields(v1beta1.TaskRunStatusFields{}, "StartTime", "CompletionTime")
	filterV1beta1PipelineRunStatus    = cmpopts.IgnoreFields(v1beta1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")
	filterContainerStateTerminated    = cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID", "Message")
	filterV1StepState                 = cmpopts.IgnoreFields(v1.StepState{}, "Name", "ImageID", "Container")
	filterV1beta1StepState            = cmpopts.IgnoreFields(v1beta1.StepState{}, "Name", "ImageID", "ContainerName")
	filterV1TaskRunSA                 = cmpopts.IgnoreFields(v1.TaskRunSpec{}, "ServiceAccountName")
	filterV1beta1TaskRunSA            = cmpopts.IgnoreFields(v1beta1.TaskRunSpec{}, "ServiceAccountName")
	filterV1PipelineRunSA             = cmpopts.IgnoreFields(v1.PipelineTaskRunTemplate{}, "ServiceAccountName")
	filterV1beta1PipelineRunSA        = cmpopts.IgnoreFields(v1beta1.PipelineRunSpec{}, "ServiceAccountName")
	filterV1RefSourceImageDigest      = cmpopts.IgnoreFields(v1.RefSource{}, "Digest")
	filterV1beta1RefSourceImageDigest = cmpopts.IgnoreFields(v1beta1.RefSource{}, "Digest")

	filterMetadata                 = []cmp.Option{filterTypeMeta, filterObjectMeta, filterAnnotations}
	filterV1TaskRunFields          = []cmp.Option{filterTypeMeta, filterObjectMeta, filterLabels, filterAnnotations, filterCondition, filterV1TaskRunStatus, filterContainerStateTerminated, filterV1StepState}
	filterV1beta1TaskRunFields     = []cmp.Option{filterTypeMeta, filterObjectMeta, filterLabels, filterAnnotations, filterV1beta1TaskRunStatus, filterCondition, filterContainerStateTerminated, filterV1beta1StepState}
	filterV1PipelineRunFields      = []cmp.Option{filterTypeMeta, filterObjectMeta, filterLabels, filterAnnotations, filterCondition, filterV1PipelineRunStatus}
	filterV1beta1PipelineRunFields = []cmp.Option{filterTypeMeta, filterObjectMeta, filterLabels, filterAnnotations, filterCondition, filterV1beta1PipelineRunStatus, filterV1beta1TaskRunStatus, filterV1beta1StepState, filterContainerStateTerminated}

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
    volumeMounts:
      - name: messages
        mountPath: /messages
    imagePullPolicy: IfNotPresent
    securityContext:
      runAsNonRoot: true
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
        runAsNonRoot: true
  sidecars:
  - name: server
    image: alpine/git:v2.26.2
    command: ['/bin/bash']
    args: ['-c', 'gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key']
    workingDir: /dir
    env:
    - name: MY_VAR1
      value: foo
    volumeMounts:
    - name: messages
      mountPath: /messages
    readinessProbe:
      periodSeconds: 1
    securityContext:
      runAsNonRoot: true
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
    volumeMounts:
      - name: messages
        mountPath: /messages
    imagePullPolicy: IfNotPresent
    securityContext:
      runAsNonRoot: true
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
        runAsNonRoot: true
  sidecars:
  - name: server
    image: alpine/git:v2.26.2
    command: ['/bin/bash']
    args: ['-c', 'gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key']
    workingDir: /dir
    env:
    - name: MY_VAR1
      value: foo
    readinessProbe:
      periodSeconds: 1
    volumeMounts:
    - name: messages
      mountPath: /messages
    securityContext:
      runAsNonRoot: true
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
      runAsNonRoot: true
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
      runAsNonRoot: true
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
  timeout: 60s
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
  serviceAccountName: default
  workspaces:
  - name: empty-dir
    emptyDir: {}
  timeout: 60s
  pipelineSpec:
    workspaces:
    - name: empty-dir
    tasks:
    - name: hello-task
      workspaces:
      - name: dir
        workspace: empty-dir
      taskSpec:
        steps:
        - name: echo-hello
          image: ubuntu
          script: |
            ls $(workspaces.dir.path)
            echo hello
`

	v1beta1PipelineRunExpectedYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: STRING_LENGTH
    value: 1
  timeouts:
    pipeline: 60s
  workspaces:
  - name: empty-dir
    emptyDir: {}
  serviceAccountName: default
  pipelineSpec:
    workspaces:
    - name: empty-dir
    tasks:
    - name: hello-task
      workspaces:
      - name: dir
        workspace: empty-dir
      taskSpec:
        steps:
        - name: echo-hello
          image: ubuntu
          script: |
            ls $(workspaces.dir.path)
            echo hello
status:
  conditions:
  - type: Succeeded
    status: "True"
    reason: "Succeeded"
  pipelineSpec:
    tasks:
    - name: hello-task
      taskSpec:
        name: cluster-task-pipeline-4
        steps:
        - name: "echo-hello"
          image: "ubuntu"
          script: |
            ls $(workspaces.dir.path)
            echo hello
      workspaces:
      - name: dir
        workspace: empty-dir
    workspaces:
    - name: empty-dir
  childReferences:
    - name: %s-hello-task
      pipelineTaskName: hello-task
      apiVersion: tekton.dev/v1
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
  workspaces:
  - name: empty-dir
    emptyDir: {}
  pipelineSpec:
    workspaces:
    - name: empty-dir
    tasks:
    - name: hello-task
      workspaces:
      - name: dir
        workspace: empty-dir
      taskSpec:
        steps:
        - name: echo-hello
          image: ubuntu
          script: |
            ls $(workspaces.dir.path)
            echo hello
  timeouts:
    pipeline: 60s
  workspaces:
  - name: empty-dir
    emptyDir: {}
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
  pipelineSpec:
    workspaces:
    - name: empty-dir
    tasks:
    - name: hello-task
      workspaces:
      - name: dir
        workspace: empty-dir
      taskSpec:
        steps:
        - name: echo-hello
          image: ubuntu
          script: |
            ls $(workspaces.dir.path)
            echo hello
  timeouts:
    pipeline: 60s
  workspaces:
  - name: empty-dir
    emptyDir: {}
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
    - name: hello-task
      taskSpec:
        name: cluster-task-pipeline-4
        steps:
        - name: "echo-hello"
          image: "ubuntu"
          script: |
            ls $(workspaces.dir.path)
            echo hello
      workspaces:
        - name: dir
          workspace: empty-dir
    workspaces:
    - name: empty-dir
  childReferences:
    - apiVersion: tekton.dev/v1
      kind: TaskRun
      name: %s-hello-task
      pipelineTaskName: hello-task
`

	v1beta1TaskWithBundleYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: alpine
    script: 'echo Hello'
`

	v1beta1PipelineWithBundleYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: hello-world
    taskRef:
      resolver: bundles
      params:
      - name: bundle
        value: %s
      - name: name
        value: %s
`

	v1beta1TaskRunWithBundleYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
    bundle: %s
`

	v1beta1PipelineRunWithBundleYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
    bundle: %s
`

	v1TaskRunWithBundleExpectedYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  serviceAccountName: default
  timeout: 1h
  taskRef:
    kind: Task
    resolver: bundles
    params:
    - name: bundle
      value: %s
    - name: name
      value: %s
    - name: kind
      value: Task
status:
  conditions:
  - type: Succeeded
    status: "True"
    reason: "Succeeded"
  podName: %s-pod
  taskSpec:
    steps:
    - computeResources: {}
      image: alpine
      name: hello
      script: 'echo Hello'
  steps:
  - image: alpine
    name: hello
    script: 'echo Hello'
    terminated:
      reason: Completed
`

	v1PipelineRunWithBundleExpectedYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  taskRunTemplate:
  timeouts: 
    pipeline: 1h
  pipelineRef:
    kind: Pipeline
    resolver: bundles
    params:
    - name: bundle
      value: %s
    - name: name
      value: %s
    - name: kind
      value: Pipeline
status:
  conditions:
  - type: Succeeded
    status: "True"
    reason: "Succeeded"
  pipelineSpec:
    tasks:
    - name: hello-world
      taskRef:
        kind: Task
        resolver: bundles
        params:
        - name: bundle
          value: %s
        - name: name
          value: %s
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: %s-hello-world
    pipelineTaskName: hello-world
`

	v1beta1TaskRunWithBundleRoundTripYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  timeout: 1h
  taskRef:
    kind: Task
    resolver: bundles
    params:
    - name: bundle
      value: %s
    - name: name
      value: %s
    - name: kind
      value: Task
status:
  conditions:
  - type: Succeeded
    status: "True"
    reason: "Succeeded"
  podName: %s-pod
  taskSpec:
    steps:
    - computeResources: {}
      image: alpine
      name: hello
      script: 'echo Hello'
  steps:
  - image: alpine
    name: hello
    script: 'echo Hello'
    terminated:
      reason: Completed
`

	v1beta1PipelineRunWithBundleRoundTripYaml = `
metadata:
  name: %s
  namespace: %s
spec:
  timeouts: 
    pipeline: 1h
  pipelineRef:
    kind: Pipeline
    resolver: bundles
    params:
    - name: bundle
      value: %s
    - name: name
      value: %s
    - name: kind
      value: Pipeline
status:
  conditions:
  - type: Succeeded
    status: "True"
    reason: "Succeeded"
  pipelineSpec:
    tasks:
    - name: hello-world
      taskRef:
        kind: Task
        resolver: bundles
        params:
        - name: bundle
          value: %s
        - name: name
          value: %s
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: %s-hello-world
    pipelineTaskName: hello-world
`
)

// TestTaskCRDConversion first creates a v1beta1 Task CRD using v1beta1Clients and
// requests it by v1Clients to compare with v1 if the conversion has been correctly
// executed by the webhook for roundtrip. And then it creates the v1 Task CRD using v1Clients
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
		t.Errorf("-want, +got: %v", d)
	}

	v1beta1TaskRoundTrip := &v1beta1.Task{}
	if err := v1beta1TaskRoundTrip.ConvertFrom(context.Background(), v1TaskGot); err != nil {
		t.Fatalf("Failed to convert roundtrip v1beta1TaskGot ConvertFrom v1 = %v", err)
	}
	if d := cmp.Diff(v1beta1Task, v1beta1TaskRoundTrip, filterMetadata...); d != "" {
		t.Errorf("-want, +got: %v", d)
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
		t.Errorf("-want, +got: %v", d)
	}

	v1TaskRoundTrip := &v1.Task{}
	if err := v1beta1TaskGot.ConvertTo(context.Background(), v1TaskRoundTrip); err != nil {
		t.Fatalf("Failed to convert roundtrip v1beta1TaskGot ConvertTo v1 = %v", err)
	}
	if d := cmp.Diff(v1Task, v1TaskRoundTrip, filterMetadata...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}
}

// TestTaskRunCRDConversion first creates a v1beta1 TaskRun CRD using v1beta1Clients
// and requests it by v1Clients to compare with v1 if the conversion has been correctly
// executed by the webhook for roundtrip. And then it creates the v1 TaskRun CRD using
// v1Clients and requests it by v1beta1Clients to compare with v1beta1.
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
	v1beta1TaskRunRoundTripExpected := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(v1beta1TaskRunExpectedYaml, v1beta1TaskRunName, namespace, v1beta1TaskRunName))

	v1TaskRunExpected.Status.Provenance = &v1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
	}
	v1beta1TaskRunRoundTripExpected.Status.Provenance = &v1beta1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
	}
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
		t.Errorf("-want, +got: %v", d)
	}

	v1beta1TaskRunRoundTrip := &v1beta1.TaskRun{}
	if err := v1beta1TaskRunRoundTrip.ConvertFrom(context.Background(), v1TaskRunGot); err != nil {
		t.Fatalf("Failed to convert roundtrip v1beta1TaskRunGot ConvertFrom v1 = %v", err)
	}
	if d := cmp.Diff(v1beta1TaskRunRoundTripExpected, v1beta1TaskRunRoundTrip, filterV1beta1TaskRunFields...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}

	v1TaskRunName := helpers.ObjectNameForTest(t)
	v1TaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(v1TaskRunYaml, v1TaskRunName, namespace))
	v1beta1TaskRunExpected := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(v1beta1TaskRunExpectedYaml, v1TaskRunName, namespace, v1TaskRunName))
	v1TaskRunRoundTripExpected := parse.MustParseV1TaskRun(t, fmt.Sprintf(v1TaskRunExpectedYaml, v1TaskRunName, namespace, v1TaskRunName))

	v1beta1TaskRunExpected.Status.Provenance = &v1beta1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
	}
	v1TaskRunRoundTripExpected.Status.Provenance = &v1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
	}

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
		t.Errorf("-want, +got: %v", d)
	}

	v1TaskRunRoundTrip := &v1.TaskRun{}
	if err := v1beta1TaskRunGot.ConvertTo(context.Background(), v1TaskRunRoundTrip); err != nil {
		t.Fatalf("Failed to convert roundtrip v1beta1TaskRunGot ConvertTo v1 = %v", err)
	}
	if d := cmp.Diff(v1TaskRunRoundTripExpected, v1TaskRunRoundTrip, filterV1TaskRunFields...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}
}

// TestPipelineCRDConversion first creates a v1beta1 Pipeline CRD using v1beta1Clients and
// requests it by v1Clients to compare with v1 if the conversion has been correctly executed
// by the webhook for roundtrip. And then it creates the v1 Pipeline CRD using v1Clients
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
		t.Errorf("-want, +got: %v", d)
	}

	v1beta1PipelineRoundTrip := &v1beta1.Pipeline{}
	if err := v1beta1PipelineRoundTrip.ConvertFrom(context.Background(), v1PipelineGot); err != nil {
		t.Fatalf("Filed to convert roundtrip v1beta1PipelineGot ConvertFrom v1 = %v", err)
	}
	if d := cmp.Diff(v1beta1Pipeline, v1beta1PipelineRoundTrip, filterMetadata...); d != "" {
		t.Errorf("-want, +got: %v", d)
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
		t.Errorf("-want, +got: %v", d)
	}

	v1PipelineRoundTrip := &v1.Pipeline{}
	if err := v1beta1PipelineGot.ConvertTo(context.Background(), v1PipelineRoundTrip); err != nil {
		t.Fatalf("Failed to convert roundtrip v1beta1PipelineGot ConvertTo v1 = %v", err)
	}
	if d := cmp.Diff(v1Pipeline, v1PipelineRoundTrip, filterMetadata...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}
}

// TestPipelineRunCRDConversion first creates a v1beta1 PipelineRun CRD using v1beta1Clients and
// requests it by v1Clients to compare with v1 if the conversion has been correctly executed by
// the webhook for roundtrip. And then it creates the v1 PipelineRun CRD using v1Clients
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
	v1beta1PRRoundTripExpected := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunExpectedYaml, v1beta1ToV1PipelineRunName, namespace, v1beta1ToV1PipelineRunName))

	v1PipelineRunExpected.Status.Provenance = &v1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
	}
	v1beta1PRRoundTripExpected.Status.Provenance = &v1beta1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
	}

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
		t.Errorf("-want, +got: %v", d)
	}

	v1beta1PRRoundTrip := &v1beta1.PipelineRun{}
	if err := v1beta1PRRoundTrip.ConvertFrom(context.Background(), v1PipelineRunGot); err != nil {
		t.Fatalf("Error roundtrip v1beta1PipelineRun ConvertFrom v1PipelineRunGot = %v", err)
	}
	if d := cmp.Diff(v1beta1PRRoundTripExpected, v1beta1PRRoundTrip, filterV1beta1PipelineRunFields...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}

	v1ToV1beta1PRName := helpers.ObjectNameForTest(t)
	v1PipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunYaml, v1ToV1beta1PRName, namespace))
	v1beta1PipelineRunExpected := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunExpectedYaml, v1ToV1beta1PRName, namespace, v1ToV1beta1PRName))
	v1PRRoundTripExpected := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunExpectedYaml, v1ToV1beta1PRName, namespace, v1ToV1beta1PRName))

	v1beta1PipelineRunExpected.Status.Provenance = &v1beta1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
	}
	v1PRRoundTripExpected.Status.Provenance = &v1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
	}

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
		t.Errorf("-want, +got: %v", d)
	}

	v1PRRoundTrip := &v1.PipelineRun{}
	if err := v1beta1PipelineRunGot.ConvertTo(context.Background(), v1PRRoundTrip); err != nil {
		t.Fatalf("Error roundtrip v1beta1PipelineRunGot ConvertTo v1 = %v", err)
	}
	if d := cmp.Diff(v1PRRoundTripExpected, v1PRRoundTrip, filterV1PipelineRunFields...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}
}

// TestBundleConversion tests v1beta1 bundle syntax converted into v1 since it has
// been deprecated in v1 and it would be converted into bundle resolver in pipelineRef
// and taskRef. It sets up a registry for a bundle of a v1beta1 Task and Pipeline
// and uses the v1beta1 TaskRef/ PipelineRef to test the conversion from v1beta1 bundle
// syntax to a v1 bundle resolver and then it tests roundtrip back to v1beta1 bundle
// resolver syntax.
func TestBundleConversion(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	t.Parallel()

	c, namespace := setup(ctx, t, withRegistry, bundleFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	repo := fmt.Sprintf("%s:5000/tektonbundlessimple", getRegistryServiceIP(ctx, t, c, namespace))
	taskName := helpers.ObjectNameForTest(t)
	pipelineName := helpers.ObjectNameForTest(t)
	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(v1beta1TaskWithBundleYaml, taskName, namespace))
	pipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(v1beta1PipelineWithBundleYaml, pipelineName, namespace, repo, taskName))
	setupBundle(ctx, t, c, namespace, repo, task, pipeline)

	v1beta1TaskRunName := helpers.ObjectNameForTest(t)
	v1beta1TaskRun := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(v1beta1TaskRunWithBundleYaml, v1beta1TaskRunName, namespace, taskName, repo))
	v1TaskRunExpected := parse.MustParseV1TaskRun(t, fmt.Sprintf(v1TaskRunWithBundleExpectedYaml, v1beta1TaskRunName, namespace, repo, taskName, v1beta1TaskRunName))
	v1beta1TaskRunRoundTripExpected := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(v1beta1TaskRunWithBundleRoundTripYaml, v1beta1TaskRunName, namespace, repo, taskName, v1beta1TaskRunName))

	v1TaskRunExpected.Status.Provenance = &v1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
		RefSource: &v1.RefSource{
			URI:        repo,
			Digest:     map[string]string{"sha256": "a123"},
			EntryPoint: taskName,
		},
	}
	v1beta1TaskRunRoundTripExpected.Status.Provenance = &v1beta1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
		RefSource: &v1beta1.RefSource{
			URI:        repo,
			Digest:     map[string]string{"sha256": "a123"},
			EntryPoint: taskName,
		},
	}

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
	if d := cmp.Diff(v1TaskRunExpected, v1TaskRunGot, append([]cmp.Option{filterV1RefSourceImageDigest, filterV1TaskRunSA}, filterV1TaskRunFields...)...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}

	v1beta1TaskRunRoundTrip := &v1beta1.TaskRun{}
	if err := v1beta1TaskRunRoundTrip.ConvertFrom(context.Background(), v1TaskRunGot); err != nil {
		t.Fatalf("Failed to convert roundtrip v1beta1TaskRunGot ConvertFrom v1 = %v", err)
	}
	if d := cmp.Diff(v1beta1TaskRunRoundTripExpected, v1beta1TaskRunRoundTrip, append([]cmp.Option{filterV1beta1RefSourceImageDigest, filterV1beta1TaskRunSA}, filterV1beta1TaskRunFields...)...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}

	v1beta1ToV1PipelineRunName := helpers.ObjectNameForTest(t)
	v1beta1PipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunWithBundleYaml, v1beta1ToV1PipelineRunName, namespace, pipelineName, repo))
	v1PipelineRunExpected := parse.MustParseV1PipelineRun(t, fmt.Sprintf(v1PipelineRunWithBundleExpectedYaml, v1beta1ToV1PipelineRunName, namespace, repo, pipelineName, repo, taskName, v1beta1ToV1PipelineRunName))
	v1beta1PRRoundTripExpected := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(v1beta1PipelineRunWithBundleRoundTripYaml, v1beta1ToV1PipelineRunName, namespace, repo, pipelineName, repo, taskName, v1beta1ToV1PipelineRunName))

	v1PipelineRunExpected.Status.Provenance = &v1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
		RefSource: &v1.RefSource{
			URI:        repo,
			Digest:     map[string]string{"sha256": "a123"},
			EntryPoint: pipelineName,
		},
	}
	v1beta1PRRoundTripExpected.Status.Provenance = &v1beta1.Provenance{
		FeatureFlags: getFeatureFlagsBaseOnAPIFlag(t),
		RefSource: &v1beta1.RefSource{
			URI:        repo,
			Digest:     map[string]string{"sha256": "a123"},
			EntryPoint: pipelineName,
		},
	}

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
	if d := cmp.Diff(v1PipelineRunExpected, v1PipelineRunGot, append([]cmp.Option{filterV1RefSourceImageDigest, filterV1PipelineRunSA}, filterV1PipelineRunFields...)...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}

	v1beta1PRRoundTrip := &v1beta1.PipelineRun{}
	if err := v1beta1PRRoundTrip.ConvertFrom(context.Background(), v1PipelineRunGot); err != nil {
		t.Fatalf("Error roundtrip v1beta1PipelineRun ConvertFrom v1PipelineRunGot = %v", err)
	}
	if d := cmp.Diff(v1beta1PRRoundTripExpected, v1beta1PRRoundTrip, append([]cmp.Option{filterV1beta1RefSourceImageDigest, filterV1beta1PipelineRunSA}, filterV1beta1PipelineRunFields...)...); d != "" {
		t.Errorf("-want, +got: %v", d)
	}
}
