//go:build featureflags
// +build featureflags

/*
Copyright 2023 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed enabled an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package test

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	sleepDuration = 15 * time.Second
	enabled       = "true"
	disabled      = "false"
)

var (
	alphaFeatureFlags = []string{"enable-param-enum", "keep-pod-enabled-cancel", "enable-cel-in-whenexpression", "enable-artifacts"}
	betaFeatureFlags  = []string{"enable-step-actions"}
	perFeatureFlags   = map[string][]string{
		"alpha": alphaFeatureFlags,
		"beta":  betaFeatureFlags,
	}

	ignorePipelineRunStatus = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "StartTime", "CompletionTime", "FinallyStartTime", "ChildReferences", "Provenance")
	ignoreTaskRunStatus     = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime", "CompletionTime", "Provenance")
)

// TestPerFeatureFlagOptInAlpha tests the behavior with all alpha Per-feature
// flags enabled. It first turns ON all per-feature flags by default and turns
// OFF one feature flag at a time to mock opt-in alpha test env.
func TestPerFeatureFlagOptInAlpha(t *testing.T) {
	configMapData := createExpectedConfigMap(t, true)
	for _, alphaFlag := range alphaFeatureFlags {
		configMapData[alphaFlag] = disabled
		testMinimumEndToEndSubSet(t, configMapData)
		configMapData[alphaFlag] = enabled
	}
}

// TestPerFeatureFlagOptInBeta tests the behavior with all beta Per-feature
// flags enabled. It first turns ON all beta per-feature flags by default and
// turns ON one alpha feature flag at a time to mock opt-in beta test env.
func TestFeatureFlagOptInBeta(t *testing.T) {
	configMapData := createExpectedConfigMap(t, false)
	for _, betaFlag := range betaFeatureFlags {
		configMapData[betaFlag] = enabled
	}
	for _, alphaFlag := range alphaFeatureFlags {
		configMapData[alphaFlag] = enabled
		testMinimumEndToEndSubSet(t, configMapData)
		configMapData[alphaFlag] = disabled
	}
}

// TestPerFeatureFlagOptInStable tests all Per-feature flags while opting in
// stable features. It turns OFF all per-feature flags by default and turns
// OFF one feature flag at a time to mock opt-in stable feature test env.
func TestPerFeatureFlagOptInStable(t *testing.T) {
	configMapData := createExpectedConfigMap(t, false)
	for _, betaFlag := range betaFeatureFlags {
		configMapData[betaFlag] = enabled
		testMinimumEndToEndSubSet(t, configMapData)
		configMapData[betaFlag] = disabled
	}
	for _, alphaFlag := range alphaFeatureFlags {
		configMapData[alphaFlag] = enabled
		testMinimumEndToEndSubSet(t, configMapData)
		configMapData[alphaFlag] = disabled
	}
}

func createExpectedConfigMap(t *testing.T, enabled bool) map[string]string {
	expectedConfigMap := make(map[string]string)
	for _, flags := range perFeatureFlags {
		for _, f := range flags {
			expectedConfigMap[f] = strconv.FormatBool(enabled)
		}
	}
	return expectedConfigMap
}

// testMinimumEndToEndSubSet examines the basic stable Pipeline functionalities
// with minimum inputs given the limitation of time to setup a Pipeline and run
// from it under the condition of
func testMinimumEndToEndSubSet(t *testing.T, configMapData map[string]string) {
	testFanInFanOut(t, configMapData)
	testResultsAndFinally(t, configMapData)
	testParams(t, configMapData)
}

// testFanInFanOut tests DAG built with a small fan-in fan-out pipeline and
// examines the sequence of the PipelineTasks being run.
func testFanInFanOut(t *testing.T, configMapData map[string]string) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	ns := os.Getenv("SYSTEM_NAMESPACE")
	if ns == "" {
		ns = "tekton-pipelines"
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}

	echoTask := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: text
    type: string
    description: 'The text that should be echoed'
  results:
  - name: result
  steps:
  - image: busybox
    script: 'echo $(params["text"])'
  - image: busybox
    # Sleep for N seconds so that we can check that tasks that
    # should be run in parallel have overlap.
    script: |
      sleep %d
      echo $(params.text) | tee $(results.result.path)
`, helpers.ObjectNameForTest(t), namespace, int(sleepDuration.Seconds())))
	if _, err := c.V1TaskClient.Create(ctx, echoTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create echo Task: %s", err)
	}

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineSpec:
    tasks:
    - name: pipeline-task-3
      params:
      - name: text
        value: wow
      - name: result-p2
        value: $(tasks.pipeline-task-2-parallel-2.results.result)
      - name: result-p1
        value: $(tasks.pipeline-task-2-parallel-1.results.result)
      taskRef:
        name: %[3]s
    - name: pipeline-task-2-parallel-2
      params:
      - name: text
        value: such parallel
      - name: result1
        value: $(tasks.pipeline-task-1.results.result)
      taskRef:
        name: %[3]s
    - name: pipeline-task-2-parallel-1
      params:
      - name: text
        value: much graph
      taskRef:
        name: %[3]s
      runAfter:
      - pipeline-task-1
    - name: pipeline-task-1
      params:
      - name: text
        value: how to ci/cd?
      taskRef:
        name: %[3]s
`, helpers.ObjectNameForTest(t), namespace, echoTask.Name))
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag-pipeline-run PipelineRun: %s", err)
	}
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunSucceed(pipelineRun.Name), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %s", err)
	}

	taskRunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't get TaskRuns (so that we could check when they executed): %v", err)
	}

	taskRuns := taskRunList.Items
	sort.Slice(taskRuns, func(i, j int) bool {
		return (taskRuns[i].Status.StartTime.Time).Before(taskRuns[j].Status.StartTime.Time)
	})

	wantPrefixes := []string{"-pipeline-task-1", "-pipeline-task-2-parallel", "-pipeline-task-2-parallel", "-pipeline-task-3"}
	for i, wp := range wantPrefixes {
		if !strings.Contains(taskRuns[i].Name, wp) {
			t.Errorf("Expected task %q to execute first, but %q was first", wp, taskRuns[i].Name)
		}
	}

	paralleTask1StartTime := taskRuns[1].Status.StartTime.Time
	paralleTask2StartTime := taskRuns[2].Status.StartTime.Time
	absDiff := time.Duration(math.Abs(float64(paralleTask2StartTime.Sub(paralleTask1StartTime))))
	if absDiff > sleepDuration {
		t.Errorf("Expected parallel tasks to execute around the same time, but they were %v apart", absDiff)
	}
}

// testResultsAndFinally tests both Results and Finally functionalities. It
// verifies the TaskRun produced by the Finally Task after a failed TaskRun
// with its results.
func testResultsAndFinally(t *testing.T, configMapData map[string]string) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	ns := os.Getenv("SYSTEM_NAMESPACE")
	if ns == "" {
		ns = "tekton-pipelines"
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineSpec:
    tasks:
    - name: task1
      taskSpec:
        results:
        - name: result1
        steps:
        - name: failing-step
          image: busybox
          script: 'echo -n 123 | tee $(results.result1.path); exit 1'
    finally:
    - name: finaltask1
      params:
      - name: param1
        value: $(tasks.task1.results.result1)
      taskSpec:
        params:
        - name: param1
        steps:
        - image: busybox
          script: 'echo $(params.param1);exit 0'
`, helpers.ObjectNameForTest(t)))

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1.PipelineRunReasonFailed.String(), pipelineRun.Name), "InvalidTaskResultReference", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to fail: %s", err)
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}
	if len(taskrunList.Items) != 2 {
		t.Fatalf("Expect PipelineRun %s to have 2 taskRuns, instead it has %d taskRuns", pipelineRun.Name, len(taskrunList.Items))
	}

	for _, taskrunItem := range taskrunList.Items {
		switch n := taskrunItem.Labels["tekton.dev/pipelineTask"]; n {
		case "task1":
			if !isFailed(t, "", taskrunItem.Status.Conditions) {
				t.Fatalf("task1 should have been a failure")
			}
			if len(taskrunItem.Status.Results) != 1 {
				t.Fatalf("task1 should have produced a result even with the failing step")
			}
			for _, r := range taskrunItem.Status.Results {
				if r.Name == "result1" && r.Value.StringVal != "123" {
					t.Fatalf("task1 should have initialized a result \"result1\" to \"123\"")
				}
			}
		case "finaltask1":
			if !isSuccessful(t, "", taskrunItem.Status.Conditions) {
				t.Fatalf("finaltask1 should have been successful")
			}
		default:
			t.Fatalf("TaskRuns were not found for both final and dag tasks")
		}
	}
}

// testParams tests the parameter propagation by comparing the expected
// TaskRuns run from the PipelineRun specified in Finally Task.
func testParams(t *testing.T, configMapData map[string]string) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	ns := os.Getenv("SYSTEM_NAMESPACE")
	if ns == "" {
		ns = "tekton-pipelines"
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}

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

	prName := pipelineRun.Name
	_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
	prResolved, err := c.V1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun `%s`: %s", prName, err)
	}

	expectedResolvedPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
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

	expectedFeatureFlags := getFeatureFlagsBaseOnAPIFlag(t)
	expectedResolvedPipelineRun.Status.Provenance = &v1.Provenance{
		FeatureFlags: expectedFeatureFlags,
	}
	if d := cmp.Diff(expectedResolvedPipelineRun, prResolved, ignoreTypeMeta, ignoreObjectMeta, ignoreCondition,
		ignorePipelineRunStatus, ignoreTaskRunStatus, ignoreConditions, ignoreContainerStates,
		ignoreStepState, ignoreSAPipelineRunSpec,
	); d != "" {
		t.Fatalf(`The resolved spec does not match the expected spec. Here is the diff: %v`, d)
	}
	expectedTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
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

	expectedFinallyTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
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

	for _, tr := range []*v1.TaskRun{expectedTaskRun, expectedFinallyTaskRun} {
		tr.Status.Provenance = &v1.Provenance{
			FeatureFlags: expectedFeatureFlags,
		}
		taskrun, _ := c.V1TaskRunClient.Get(ctx, tr.Name, metav1.GetOptions{})
		if d := cmp.Diff(tr, taskrun, ignoreTypeMeta, ignoreObjectMeta, ignoreCondition, ignoreTaskRunStatus,
			ignoreConditions, ignoreContainerStates, ignoreStepState, ignoreSATaskRunSpec,
		); d != "" {
			t.Fatalf(`The expected taskrun does not match created taskrun. Here is the diff: %v`, d)
		}
	}
}
