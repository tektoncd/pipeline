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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
)

var (
	ignoreTaskRunStatusFields = cmpopts.IgnoreFields(v1beta1.TaskRunStatusFields{}, "Steps", "TaskRunResults")
	ignoreSidecarState        = cmpopts.IgnoreFields(v1beta1.SidecarState{}, "ImageID")

	requireSidecarLogResultsGate = map[string]string{
		"results-from": "sidecar-logs",
	}
)

func TestLargerResultsSidecarLogs(t *testing.T) {
	t.Parallel()
	type tests struct {
		name            string
		pipelineName    string
		pipelineRunFunc func(*testing.T, string) (*v1beta1.PipelineRun, *v1beta1.PipelineRun, []*v1beta1.TaskRun)
	}

	tds := []tests{{
		name:            "larger results via sidecar logs",
		pipelineName:    "larger-results-sidecar-logs",
		pipelineRunFunc: getLargerResultsPipelineRun,
	}}

	for _, td := range tds {
		td := td
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			c, namespace := setUpSidecarLogs(ctx, t, requireAllGates(requireSidecarLogResultsGate))

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			pipelineRun, expectedResolvedPipelineRun, expectedTaskRuns := td.pipelineRunFunc(t, namespace)
			prName := pipelineRun.Name
			_, err := c.V1beta1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1beta1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
			}
			cl, _ := c.V1beta1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
			d := cmp.Diff(expectedResolvedPipelineRun, cl,
				ignoreTypeMeta,
				ignoreObjectMeta,
				ignoreCondition,
				ignorePipelineRunStatus,
				ignoreTaskRunStatus,
				ignoreConditions,
				ignoreContainerStates,
				ignoreStepState,
			)
			if d != "" {
				t.Fatalf(`The resolved spec does not match the expected spec. Here is the diff: %v`, d)
			}
			for _, tr := range expectedTaskRuns {
				t.Logf("Checking Taskrun %s", tr.Name)
				taskrun, _ := c.V1beta1TaskRunClient.Get(ctx, tr.Name, metav1.GetOptions{})
				d = cmp.Diff(tr, taskrun,
					ignoreTypeMeta,
					ignoreObjectMeta,
					ignoreCondition,
					ignoreTaskRunStatus,
					ignoreContainerStates,
					ignoreStepState,
					ignoreTaskRunSpec,
					ignoreTaskRunStatusFields,
					ignoreSidecarState,
				)
				if d != "" {
					t.Fatalf(`The expected taskrun does not match created taskrun. Here is the diff: %v`, d)
				}
			}
			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func getLargerResultsPipelineRun(t *testing.T, namespace string) (*v1beta1.PipelineRun, *v1beta1.PipelineRun, []*v1beta1.TaskRun) {
	t.Helper()
	pipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: larger-results-sidecar-logs
  namespace: %s
spec:
  pipelineSpec:
    tasks:
      - name: task1
        taskSpec:
          results:
            - name: result1
            - name: result2
          steps:
           - name: step1
             image: alpine
             script: |
               echo -n "%s"| tee $(results.result1.path);
               echo -n "%s"| tee $(results.result2.path);
      - name: task2
        params:
          - name: param1
            value: "$(tasks.task1.results.result1)"
          - name: param2
            value: "$(tasks.task1.results.result2)"
        taskSpec:
          params:
            - name: param1
              type: string
              default: abc
            - name: param2
              type: string
              default: def
          results:
            - name: large-result
          steps:
            - name: step1
              image: alpine
              script: |
                echo -n "$(params.param1)">> $(results.large-result.path);
                echo -n "$(params.param2)">> $(results.large-result.path);
    results:
      - name: large-result 
        value: $(tasks.task2.results.large-result)
`, namespace, strings.Repeat("a", 2000), strings.Repeat("b", 2000)))
	expectedPipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: larger-results-sidecar-logs
  namespace: %s
spec:
  serviceAccountName: default
  timeout: 1h
  pipelineSpec:
    tasks:
      - name: task1
        taskSpec:
          results:
            - name: result1
              type: string
            - name: result2
              type: string
          steps:
           - name: step1
             image: alpine
             script: |
               echo -n "%s"| tee $(results.result1.path);
               echo -n "%s"| tee $(results.result2.path);
      - name: task2
        params:
          - name: param1
            value: "$(tasks.task1.results.result1)"
          - name: param2
            value: "$(tasks.task1.results.result2)"
        taskSpec:
          params:
            - name: param1
              type: string
              default: abc
            - name: param2
              type: string
              default: def
          results:
            - name: large-result
              type: string
          steps:
            - name: step1
              image: alpine
              script: |
                echo -n "$(params.param1)">> $(results.large-result.path);
                echo -n "$(params.param2)">> $(results.large-result.path);
    results:
      - name: large-result 
        value: $(tasks.task2.results.large-result)
status:
  pipelineSpec:
    tasks:
      - name: task1
        taskSpec:
          results:
            - name: result1
              type: string
            - name: result2
              type: string
          steps:
            - name: step1
              image: alpine
              script: |
                echo -n "%s"| tee $(results.result1.path);
                echo -n "%s"| tee $(results.result2.path);
      - name: task2
        params:
          - name: param1
            value: "$(tasks.task1.results.result1)"
          - name: param2
            value: "$(tasks.task1.results.result2)"
        taskSpec:
          params:
            - name: param1
              type: string
              default: abc
            - name: param2
              type: string
              default: def
          results:
            - name: large-result
              type: string
          steps:
            - name: step1
              image: alpine
              script: |
                echo -n "$(params.param1)">> $(results.large-result.path);
                echo -n "$(params.param2)">> $(results.large-result.path);
    results:
      - name: large-result 
        value: $(tasks.task2.results.large-result)
  pipelineResults:
    - name: large-result
      value: %s%s
`, namespace, strings.Repeat("a", 2000), strings.Repeat("b", 2000), strings.Repeat("a", 2000), strings.Repeat("b", 2000), strings.Repeat("a", 2000), strings.Repeat("b", 2000)))
	taskRun1 := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(`
metadata:
  name: larger-results-sidecar-logs-task1
  namespace: %s
spec:
  serviceAccountName: default
  timeout: 1h
  taskSpec:
    results:
      - name: result1
        type: string
      - name: result2
        type: string
    steps:
      - name: step1
        image: alpine
        script: |
          echo -n "%s"| tee $(results.result1.path);
          echo -n "%s"| tee $(results.result2.path);
status:
  conditions:
    - type: "Succeeded"
      status: "True"
      reason: "Succeeded"
  podName: larger-results-sidecar-logs-task1-pod
  taskSpec:
    results:
      - name: result1
        type: string
      - name: result2
        type: string
    steps:
      - name: step1
        image: alpine
        script: |
          echo -n "%s"| tee /tekton/results/result1;
          echo -n "%s"| tee /tekton/results/result2;
  taskResults:
    - name: result1
      type: string
      value: %s
    - name: result2
      type: string
      value: %s
  sidecars:
    - name: tekton-log-results
      container: sidecar-tekton-log-results    
`, namespace, strings.Repeat("a", 2000), strings.Repeat("b", 2000), strings.Repeat("a", 2000), strings.Repeat("b", 2000), strings.Repeat("a", 2000), strings.Repeat("b", 2000)))
	taskRun2 := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(`
metadata:
  name: larger-results-sidecar-logs-task2
  namespace: %s
spec:
  serviceAccountName: default
  timeout: 1h
  params:
    - name: param1
      type: string
      value: %s 
    - name: param2
      type: string 
      value: %s
  taskSpec:
    params:
      - name: param1
        type: string
        default: abc
      - name: param2
        type: string
        default: def
    results:
      - name: large-result
        type: string
    steps:
     - name: step1
       image: alpine
       script: |
         echo -n "$(params.param1)">> $(results.large-result.path);
         echo -n "$(params.param2)">> $(results.large-result.path);
status:
  conditions:
    - type: "Succeeded"
      status: "True"
      reason: "Succeeded"
  podName: larger-results-sidecar-logs-task2-pod
  taskSpec:
    params:
      - name: param1
        type: string
        default: abc
      - name: param2
        type: string
        default: def
    results:
      - name: large-result
        type: string
    steps:
     - name: step1
       image: alpine
       script: |
         echo -n "%s">> /tekton/results/large-result;
         echo -n "%s">> /tekton/results/large-result;
  taskResults:
    - name: large-result
      type: string
      value: %s%s
  sidecars:
    - name: tekton-log-results
      container: sidecar-tekton-log-results    
`, namespace, strings.Repeat("a", 2000), strings.Repeat("b", 2000), strings.Repeat("a", 2000), strings.Repeat("b", 2000), strings.Repeat("a", 2000), strings.Repeat("b", 2000)))
	return pipelineRun, expectedPipelineRun, []*v1beta1.TaskRun{taskRun1, taskRun2}
}

func setUpSidecarLogs(ctx context.Context, t *testing.T, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string) {
	t.Helper()
	c, ns := setup(ctx, t)
	configMapData := map[string]string{
		"results-from": "sidecar-logs",
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
	return c, ns
}
