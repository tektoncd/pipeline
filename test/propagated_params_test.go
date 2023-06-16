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
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	knativetest "knative.dev/pkg/test"
)

var (
	ignoreTypeMeta          = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	ignoreObjectMeta        = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields", "Labels", "Annotations", "OwnerReferences")
	ignoreCondition         = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time", "Message")
	ignorePipelineRunStatus = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "StartTime", "CompletionTime", "FinallyStartTime", "ChildReferences")
	ignoreTaskRunStatus     = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime", "CompletionTime")
	ignoreConditions        = cmpopts.IgnoreFields(duckv1.Status{}, "Conditions")
	ignoreContainerStates   = cmpopts.IgnoreFields(corev1.ContainerState{}, "Terminated")
	ignoreStepState         = cmpopts.IgnoreFields(v1.StepState{}, "ImageID")
	// ignoreSATaskRunSpec ignores the service account in the TaskRunSpec as it may differ across platforms
	ignoreSATaskRunSpec = cmpopts.IgnoreFields(v1.TaskRunSpec{}, "ServiceAccountName")
	// ignoreSAPipelineRunSpec ignores the service account in the PipelineRunSpec as it may differ across platforms
	ignoreSAPipelineRunSpec = cmpopts.IgnoreFields(v1.PipelineTaskRunTemplate{}, "ServiceAccountName")
)

func TestPropagatedParams(t *testing.T) {
	t.Parallel()

	expectedFeatureFlags := getFeatureFlagsBaseOnAPIFlag(t)
	type tests struct {
		name            string
		pipelineName    string
		taskName        string
		pipelineRunFunc func(*testing.T, string) (*v1.PipelineRun, *v1.PipelineRun, []*v1.TaskRun)
	}

	tds := []tests{{
		name:            "propagated parameters fully",
		pipelineName:    "propagated-parameters-fully",
		taskName:        "echo-hello",
		pipelineRunFunc: getPropagatedParamPipelineRun,
	}, {
		name:            "propagated parameters with task level",
		pipelineName:    "propagated-parameters-task-level",
		taskName:        "echo-hello",
		pipelineRunFunc: getPropagatedParamTaskLevelPipelineRun,
	}, {
		name:            "propagated parameters with default task level",
		pipelineName:    "propagated-parameters-default-task-level",
		taskName:        "echo-hello",
		pipelineRunFunc: getPropagatedParamTaskLevelDefaultPipelineRun,
	}}

	for _, td := range tds {
		td := td
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			c, namespace := setup(ctx, t)

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			pipelineRun, expectedResolvedPipelineRun, expectedTaskRuns := td.pipelineRunFunc(t, namespace)

			expectedResolvedPipelineRun.Status.Provenance = &v1.Provenance{
				FeatureFlags: expectedFeatureFlags,
			}

			prName := pipelineRun.Name
			_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
			}
			cl, _ := c.V1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
			d := cmp.Diff(expectedResolvedPipelineRun, cl,
				ignoreTypeMeta,
				ignoreObjectMeta,
				ignoreCondition,
				ignorePipelineRunStatus,
				ignoreTaskRunStatus,
				ignoreConditions,
				ignoreContainerStates,
				ignoreStepState,
				ignoreSAPipelineRunSpec,
			)
			if d != "" {
				t.Fatalf(`The resolved spec does not match the expected spec. Here is the diff: %v`, d)
			}
			for _, tr := range expectedTaskRuns {
				t.Logf("Checking Taskrun %s", tr.Name)
				tr.Status.Provenance = &v1.Provenance{
					FeatureFlags: expectedFeatureFlags,
				}
				taskrun, _ := c.V1TaskRunClient.Get(ctx, tr.Name, metav1.GetOptions{})
				d = cmp.Diff(tr, taskrun,
					ignoreTypeMeta,
					ignoreObjectMeta,
					ignoreCondition,
					ignoreTaskRunStatus,
					ignoreConditions,
					ignoreContainerStates,
					ignoreStepState,
					ignoreSATaskRunSpec,
				)
				if d != "" {
					t.Fatalf(`The expected taskrun does not match created taskrun. Here is the diff: %v`, d)
				}
			}
			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func getPropagatedParamPipelineRun(t *testing.T, namespace string) (*v1.PipelineRun, *v1.PipelineRun, []*v1.TaskRun) {
	t.Helper()
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-fully
  namespace: %s
spec:
  params:
  - name: HELLO
    value: "Hello World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo $(params.HELLO)
    finally:
      - name: echo-hello-finally
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo $(params.HELLO)
`, namespace))
	expectedPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-fully
  namespace: %s
spec:
  timeouts:
    pipeline: 1h
  params:
  - name: HELLO
    value: "Hello World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo $(params.HELLO)
    finally:
      - name: echo-hello-finally
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo $(params.HELLO)
status:
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo Hello World!
    finally:
      - name: echo-hello-finally
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo Hello World!
`, namespace))
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-fully-echo-hello
  namespace: %s
spec:
  timeout: 1h
  taskSpec:
    steps:
      - name: echo
        image: ubuntu
        script: echo Hello World!
status:
   podName: propagated-parameters-fully-echo-hello-pod
   steps:
     - name: echo
       container: step-echo
   taskSpec:
     steps:
       - name: echo
         image: ubuntu
         script: echo Hello World!
`, namespace))
	finallyTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-fully-echo-hello-finally
  namespace: %s
spec:
  timeout: 1h
  taskSpec:
    steps:
      - name: echo
        image: ubuntu
        script: echo Hello World!
status:
   podName: propagated-parameters-fully-echo-hello-finally-pod
   steps:
     - name: echo
       container: step-echo
   taskSpec:
     steps:
       - name: echo
         image: ubuntu
         script: echo Hello World!
`, namespace))
	return pipelineRun, expectedPipelineRun, []*v1.TaskRun{taskRun, finallyTaskRun}
}

func getPropagatedParamTaskLevelPipelineRun(t *testing.T, namespace string) (*v1.PipelineRun, *v1.PipelineRun, []*v1.TaskRun) {
	t.Helper()
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-task-level
  namespace: %s
spec:
  params:
  - name: HELLO
    value: "Pipeline Hello World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        params:
          - name: HELLO
            value: "Hello World!"
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo $(params.HELLO)
`, namespace))
	expectedPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-task-level
  namespace: %s
spec:
  timeouts:
    pipeline: 1h
  params:
  - name: HELLO
    value: "Pipeline Hello World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        params:
          - name: HELLO
            value: "Hello World!"
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo $(params.HELLO)
status:
  pipelineSpec:
    tasks:
      - name: echo-hello
        params:
          - name: HELLO
            value: "Hello World!"
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: echo Hello World!
`, namespace))
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-task-level-echo-hello
  namespace: %s
spec:
  timeout: 1h
  params:
    - name: HELLO
      value: "Hello World!"
  taskSpec:
    steps:
      - name: echo
        image: ubuntu
        script: echo Hello World!
status:
  podName: propagated-parameters-task-level-echo-hello-pod
  steps:
    - name: echo
      container: step-echo
  taskSpec:
    steps:
      - name: echo
        image: ubuntu
        script: echo Hello World!
`, namespace))
	return pipelineRun, expectedPipelineRun, []*v1.TaskRun{taskRun}
}

func getPropagatedParamTaskLevelDefaultPipelineRun(t *testing.T, namespace string) (*v1.PipelineRun, *v1.PipelineRun, []*v1.TaskRun) {
	t.Helper()
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-default-task-level
  namespace: %s
spec:
  params:
  - name: HELLO
    value: "Hello World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskSpec:
          params:
            - name: HELLO
              type: string
              default: "Default Hello World"
          steps:
            - name: echo
              image: ubuntu
              script: echo $(params.HELLO)
`, namespace))
	expectedPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-default-task-level
  namespace: %s
spec:
  timeouts:
    pipeline: 1h
  params:
  - name: HELLO
    value: "Hello World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskSpec:
          params:
            - name: HELLO
              type: string
              default: "Default Hello World"
          steps:
            - name: echo
              image: ubuntu
              script: echo $(params.HELLO)
status:
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskSpec:
          params:
            - name: HELLO
              type: string
              default: "Default Hello World"
          steps:
            - name: echo
              image: ubuntu
              script: echo Hello World!
`, namespace))
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: propagated-parameters-default-task-level-echo-hello
  namespace: %s
spec:
  timeout: 1h
  taskSpec:
    params:
      - name: HELLO
        type: string
        default: "Default Hello World"
    steps:
      - name: echo
        image: ubuntu
        script: echo Hello World!
status:
  podName: propagated-parameters-default-task-level-echo-hello-pod
  steps:
    - name: echo
      container: step-echo
  taskSpec:
    params:
     - name: HELLO
       type: string
       default: "Default Hello World"
    steps:
      - name: echo
        image: ubuntu
        script: echo Hello World!
`, namespace))
	return pipelineRun, expectedPipelineRun, []*v1.TaskRun{taskRun}
}
