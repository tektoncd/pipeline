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
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
)

var (
	ignoreProvenance = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "Provenance")
)

func TestStepResultsStepActions(t *testing.T) {
	featureFlags := getFeatureFlagsBaseOnAPIFlag(t)
	previousResultExtractionMethod := featureFlags.ResultExtractionMethod

	type tests struct {
		name           string
		taskRunFunc    func(*testing.T, string) (*v1.TaskRun, *v1.TaskRun)
		stepActionFunc func(*testing.T, string) *v1beta1.StepAction
	}

	tds := []tests{{
		name:           "step results",
		taskRunFunc:    getStepActionsTaskRun,
		stepActionFunc: getStepAction,
	}}

	for _, td := range tds {
		t.Run(td.name, func(t *testing.T) {
			ctx := t.Context()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			c, namespace := setUpStepActionsResults(ctx, t)

			// reset configmap
			knativetest.CleanupOnInterrupt(func() { resetStepActionsResults(ctx, t, c, previousResultExtractionMethod) }, t.Logf)
			defer resetSidecarLogs(ctx, t, c, previousResultExtractionMethod)

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			taskRun, expectedResolvedTaskRun := td.taskRunFunc(t, namespace)
			stepAction := td.stepActionFunc(t, namespace)

			trName := taskRun.Name

			_, err := c.V1beta1StepActionClient.Create(ctx, stepAction, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create StepAction : %s", err)
			}

			_, err = c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create TaskRun `%s`: %s", trName, err)
			}

			t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
			if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess", v1Version); err != nil {
				t.Fatalf("Error waiting for TaskRun %s to finish: %s", trName, err)
			}
			taskrun, _ := c.V1TaskRunClient.Get(ctx, trName, metav1.GetOptions{})
			d := cmp.Diff(expectedResolvedTaskRun, taskrun,
				ignoreTypeMeta,
				ignoreObjectMeta,
				ignoreCondition,
				ignoreTaskRunStatus,
				ignoreContainerStates,
				ignoreProvenance,
				ignoreSidecarState,
				ignoreStepState,
			)
			if d != "" {
				t.Fatalf(`The expected taskrun does not match created taskrun. Here is the diff: %v`, d)
			}

			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func getStepAction(t *testing.T, namespace string) *v1beta1.StepAction {
	t.Helper()
	return parse.MustParseV1beta1StepAction(t, fmt.Sprintf(`
metadata:
  name: step-action
  namespace: %s
spec:
  results:
    - name: result1
      type: string
  image: mirror.gcr.io/alpine
  script: |
    echo -n step-action >> $(step.results.result1.path)
`, namespace))
}

func getStepActionsTaskRun(t *testing.T, namespace string) (*v1.TaskRun, *v1.TaskRun) {
	t.Helper()
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: step-results-task-run
  namespace: %s
spec:
  taskSpec:
    steps:
     - name: step1
       image: mirror.gcr.io/alpine
       script: |
         echo -n inlined-step >> $(step.results.result1.path)
       results:
         - name: result1
           type: string
     - name: step2
       ref:
         name: step-action
    results:
      - name: inlined-step-result
        type: string
        value: $(steps.step1.results.result1)
      - name: step-action-result
        type: string
        value: $(steps.step2.results.result1)
`, namespace))

	expectedTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: step-results-task-run
  namespace: %s
spec:
  serviceAccountName: default
  timeout: 1h
  taskSpec:
    steps:
      - name: step1
        image: mirror.gcr.io/alpine
        script: |
          echo -n inlined-step >> $(step.results.result1.path)
        results:
          - name: result1
            type: string
      - name: step2
        ref:
          name: step-action
    results:
      - name: inlined-step-result
        type: string
        value: $(steps.step1.results.result1)
      - name: step-action-result
        type: string
        value: $(steps.step2.results.result1)
status:
  conditions:
    - type: "Succeeded"
      status: "True"
      reason: "Succeeded"
  podName: step-results-task-run-pod
  artifacts: {}
  taskSpec:
    steps:
      - name: step1
        image: mirror.gcr.io/alpine
        results:
          - name: result1
            type: string
        script: |
          echo -n inlined-step >> /tekton/steps/step-step1/results/result1
      - name: step2
        image: mirror.gcr.io/alpine
        results:
          - name: result1
            type: string
        script: |
          echo -n step-action >> /tekton/steps/step-step2/results/result1
    results:
      - name: inlined-step-result
        type: string
        value: $(steps.step1.results.result1)
      - name: step-action-result
        type: string
        value: $(steps.step2.results.result1)
  results:
    - name: inlined-step-result
      type: string
      value: inlined-step
    - name: step-action-result
      type: string
      value: step-action
  steps:
    - name: step1
      container: step-step1
      results:
        - name: result1
          type: string
          value: inlined-step
    - name: step2
      container: step-step2
      results:
        - name: result1
          type: string
          value: step-action
`, namespace))
	return taskRun, expectedTaskRun
}

func setUpStepActionsResults(ctx context.Context, t *testing.T) (*clients, string) {
	t.Helper()
	c, ns := setup(ctx, t)
	configMapData := map[string]string{
		"results-from": "termination-message",
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
	return c, ns
}

func resetStepActionsResults(ctx context.Context, t *testing.T, c *clients, previousResultExtractionMethod string) {
	t.Helper()
	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{"results-from": previousResultExtractionMethod}); err != nil {
		t.Fatal(err)
	}
}
