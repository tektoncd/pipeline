//go:build e2e
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
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	apiVersion      = "wait.testing.tekton.dev/v1alpha1"
	kind            = "Wait"
	waitTaskDir     = "./custom-task-ctrls/wait-task-alpha"
	betaAPIVersion  = "wait.testing.tekton.dev/v1beta1"
	betaWaitTaskDir = "./custom-task-ctrls/wait-task-beta"
)

var (
	// We need this to be set because the way cancellation/timeout works, the full embedded status may not be updated
	// in the PipelineRun's status for the cancelled CustomRun/Run before the PipelineRun itself finishes reconciling,
	// so testing against "full" embedded status is inherently flaky. We currently only run these tests in
	// pull-tekton-pipeline-alpha-integration-tests, which has the needed feature flag(s) set for custom tasks to be
	// enabled, but that job also as "embedded-status" set to "minimal", so the flakiness never shows up in CI.
	// Therefore, there's no point in even having logic in these tests for full embedded status cases, because we never
	// exercise these tests in CI in that scenario.
	requiredEmbeddedStatusGate = map[string]string{
		"embedded-status": "minimal",
	}
	filterTypeMeta          = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	filterObjectMeta        = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields")
	filterCondition         = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time", "Message")
	filterCustomRunStatus   = cmpopts.IgnoreFields(v1beta1.CustomRunStatusFields{}, "StartTime", "CompletionTime")
	filterRunStatus         = cmpopts.IgnoreFields(v1alpha1.RunStatusFields{}, "StartTime", "CompletionTime")
	filterPipelineRunStatus = cmpopts.IgnoreFields(v1beta1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")
)

func TestCustomTask(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setUpCustomTask(ctx, t, requireAllGates(requiredEmbeddedStatusGate))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	customTaskRawSpec := []byte(`{"field1":123,"field2":"value"}`)
	metadataLabel := map[string]string{"test-label": "test"}
	// Create a PipelineRun that runs a Custom Task.
	pipelineRunName := helpers.ObjectNameForTest(t)
	if _, err := c.V1beta1PipelineRunClient.Create(
		ctx,
		parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineSpec:
    results:
    - name: prResult-ref
      value: $(tasks.custom-task-ref.results.runResult)
    - name: prResult-spec
      value: $(tasks.custom-task-spec.results.runResult)
    tasks:
    - name: custom-task-ref
      taskRef:
        apiVersion: %s
        kind: %s
    - name: custom-task-spec
      taskSpec:
        apiVersion: %s
        kind: %s
        metadata:
          labels:
            test-label: test
        spec: %s
    - name: result-consumer
      params:
      - name: input-result-from-custom-task-ref
        value: $(tasks.custom-task-ref.results.runResult)
      - name: input-result-from-custom-task-spec
        value: $(tasks.custom-task-spec.results.runResult)
      taskSpec:
        params:
        - name: input-result-from-custom-task-ref
          type: string
        - name: input-result-from-custom-task-spec
          type: string
        steps:
        - args: ['-c', 'echo $(input-result-from-custom-task-ref) $(input-result-from-custom-task-spec)']
          command: ['/bin/bash']
          image: ubuntu
`, pipelineRunName, apiVersion, kind, apiVersion, kind, customTaskRawSpec)),
		metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRunName, err)
	}

	// Wait for the PipelineRun to start.
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, time.Minute, Running(pipelineRunName), "PipelineRunRunning", v1beta1Version); err != nil {
		t.Fatalf("Waiting for PipelineRun to start running: %v", err)
	}

	// Get the status of the PipelineRun.
	pr, err := c.V1beta1PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRunName, err)
	}

	// Get the Run name.
	var runNames []string
	for _, cr := range pr.Status.ChildReferences {
		if cr.Kind == pipeline.RunControllerName {
			runNames = append(runNames, cr.Name)
		}
	}
	if len(runNames) != 2 {
		t.Fatalf("PipelineRun had unexpected number of Runs in .status.childReferences; got %d, want 2", len(runNames))
	}
	for _, runName := range runNames {
		// Get the Run.
		cr, err := c.V1alpha1RunClient.Get(ctx, runName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get Run %q: %v", runName, err)
		}
		if cr.IsDone() {
			t.Fatalf("Run unexpectedly done: %v", cr.Status.GetCondition(apis.ConditionSucceeded))
		}

		// Simulate a Custom Task controller updating the CustomRun to done/successful.
		cr.Status = v1alpha1.RunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			RunStatusFields: v1alpha1.RunStatusFields{
				Results: []v1alpha1.RunResult{{
					Name:  "runResult",
					Value: "aResultValue",
				}},
			},
		}

		if _, err := c.V1alpha1RunClient.UpdateStatus(ctx, cr, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Failed to update Run to successful: %v", err)
		}

		// Get the Run.
		cr, err = c.V1alpha1RunClient.Get(ctx, runName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get Run %q: %v", runName, err)
		}

		if strings.Contains(runName, "custom-task-spec") {
			if d := cmp.Diff(customTaskRawSpec, cr.Spec.Spec.Spec.Raw); d != "" {
				t.Fatalf("Unexpected value of Spec.Raw: %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(metadataLabel, cr.Spec.Spec.Metadata.Labels); d != "" {
				t.Fatalf("Unexpected value of Metadata.Labels: %s", diff.PrintWantGot(d))
			}
		}
		if !cr.IsDone() {
			t.Fatalf("Run unexpectedly not done after update (UpdateStatus didn't work): %v", cr.Status)
		}
	}
	// Wait for the PipelineRun to become done/successful.
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, time.Minute, PipelineRunSucceed(pipelineRunName), "PipelineRunCompleted", v1beta1Version); err != nil {
		t.Fatalf("Waiting for PipelineRun to complete successfully: %v", err)
	}

	// Get the updated status of the PipelineRun.
	pr, err = c.V1beta1PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q after it completed: %v", pipelineRunName, err)
	}

	// Get the TaskRun name.
	var taskRunName string

	for _, cr := range pr.Status.ChildReferences {
		if cr.Kind == "TaskRun" {
			taskRunName = cr.Name
		}
	}
	if taskRunName == "" {
		t.Fatal("PipelineRun does not have expected TaskRun in .status.childReferences")
	}

	// Get the TaskRun.
	taskRun, err := c.V1beta1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get TaskRun %q: %v", taskRunName, err)
	}

	// Validate the task's result reference to the custom task's result was resolved.
	expectedTaskRunParams := []v1beta1.Param{{
		Name: "input-result-from-custom-task-ref", Value: *v1beta1.NewStructuredValues("aResultValue"),
	}, {
		Name: "input-result-from-custom-task-spec", Value: *v1beta1.NewStructuredValues("aResultValue"),
	}}

	if d := cmp.Diff(expectedTaskRunParams, taskRun.Spec.Params); d != "" {
		t.Fatalf("Unexpected TaskRun Params: %s", diff.PrintWantGot(d))
	}

	// Validate that the pipeline's result reference to the custom task's result was resolved.

	expectedPipelineResults := []v1beta1.PipelineRunResult{{
		Name:  "prResult-ref",
		Value: *v1beta1.NewStructuredValues("aResultValue"),
	}, {
		Name:  "prResult-spec",
		Value: *v1beta1.NewStructuredValues("aResultValue"),
	}}

	if len(pr.Status.PipelineResults) != 2 {
		t.Fatalf("Expected 2 PipelineResults but there are %d.", len(pr.Status.PipelineResults))
	}
	if d := cmp.Diff(expectedPipelineResults, pr.Status.PipelineResults); d != "" {
		t.Fatalf("Unexpected PipelineResults: %s", diff.PrintWantGot(d))
	}
}

// WaitForRunSpecCancelled polls the spec.status of the Run until it is
// "RunCancelled", returns an error on timeout. desc will be used to name
// the metric that is emitted to track how long it took.
func WaitForRunSpecCancelled(ctx context.Context, c *clients, name string, desc string) error {
	metricName := fmt.Sprintf("WaitForRunSpecCancelled/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return pollImmediateWithContext(ctx, func() (bool, error) {
		cr, err := c.V1alpha1RunClient.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return cr.Spec.Status == v1alpha1.RunSpecStatusCancelled, nil
	})
}

// TestPipelineRunCustomTaskTimeout is an integration test that will
// verify that pipelinerun timeout works and leads to the the correct Run Spec.status
func TestPipelineRunCustomTaskTimeout(t *testing.T) {
	// cancel the context after we have waited a suitable buffer beyond the given deadline.
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Minute)
	defer cancel()
	c, namespace := setUpCustomTask(ctx, t, requireAllGates(requiredEmbeddedStatusGate))
	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	pipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: custom-task-ref
    taskRef:
      apiVersion: %s
      kind: %s
`, helpers.ObjectNameForTest(t), namespace, apiVersion, kind))
	pipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  timeout: 5s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))
	if _, err := c.V1beta1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.V1beta1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, Running(pipelineRun.Name), "PipelineRunRunning", v1beta1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be running: %s", pipelineRun.Name, err)
	}

	pr, err := c.V1beta1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
	}

	// Get the Run name.
	runName := ""

	if len(pr.Status.ChildReferences) != 1 {
		t.Fatalf("PipelineRun had unexpected .status.childReferences; got %d, want 1", len(pr.Status.ChildReferences))
	}
	runName = pr.Status.ChildReferences[0].Name

	// Get the Run.
	cr, err := c.V1alpha1RunClient.Get(ctx, runName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get Run %q: %v", runName, err)
	}
	if cr.IsDone() {
		t.Fatalf("Run unexpectedly done: %v", cr.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Simulate a Custom Task controller updating the Run to be started/running,
	// because, a Run that has not started cannot timeout.
	cr.Status = v1alpha1.RunStatus{
		RunStatusFields: v1alpha1.RunStatusFields{
			StartTime: &metav1.Time{Time: time.Now()},
		},
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionUnknown,
			}},
		},
	}
	if _, err := c.V1alpha1RunClient.UpdateStatus(ctx, cr, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to update CustomRun to successful: %v", err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be timed out", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1beta1.PipelineRunReasonTimedOut.String(), pipelineRun.Name), "PipelineRunTimedOut", v1beta1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	runList, err := c.V1alpha1RunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing Runs for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	t.Logf("Runs from PipelineRun %s in namespace %s must be cancelled", pipelineRun.Name, namespace)
	var wg sync.WaitGroup
	for _, crItem := range runList.Items {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			err := WaitForRunSpecCancelled(ctx, c, name, "RunCancelled")
			if err != nil {
				t.Errorf("Error waiting for Run %s to cancel: %s", name, err)
			}
		}(crItem.Name)
	}
	wg.Wait()

	if _, err := c.V1beta1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRun.Name, err)
	}
}

func applyController(t *testing.T) {
	t.Log("Creating Wait Custom Task Controller...")
	cmd := exec.Command("ko", "apply", "-f", "./config/controller.yaml")
	cmd.Dir = waitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func applyV1Beta1Controller(t *testing.T) {
	t.Log("Creating Wait v1beta1.CustomRun Custom Task Controller...")
	cmd := exec.Command("ko", "apply", "-f", "./config/controller.yaml")
	cmd.Dir = betaWaitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func cleanUpController(t *testing.T) {
	t.Log("Tearing down Wait Custom Task Controller...")
	cmd := exec.Command("ko", "delete", "-f", "./config/controller.yaml")
	cmd.Dir = waitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to tear down Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func cleanUpV1Beta1Controller(t *testing.T) {
	t.Log("Tearing down Wait v1beta1.CustomRun Custom Task Controller...")
	cmd := exec.Command("ko", "delete", "-f", "./config/controller.yaml")
	cmd.Dir = betaWaitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to tear down Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func TestWaitCustomTask_Run(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setUpCustomTask(ctx, t, requireAllGates(requiredEmbeddedStatusGate))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a custom task controller
	applyController(t)
	// Cleanup the controller after finishing the test
	defer cleanUpController(t)

	for _, tc := range []struct {
		name                string
		duration            string
		timeout             *metav1.Duration
		retries             int
		conditionAccessorFn func(string) ConditionAccessorFn
		wantCondition       apis.Condition
		wantRetriesStatus   []v1alpha1.RunStatus
	}{{
		name:                "Wait Task Has Passed",
		duration:            "1s",
		conditionAccessorFn: Succeed,
		wantCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
			Reason: "DurationElapsed",
		},
	}, {
		name:                "Wait Task Is Running",
		duration:            "2s",
		conditionAccessorFn: Running,
		wantCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: "Running",
		},
	}, {
		name:                "Wait Task Timed Out",
		duration:            "2s",
		timeout:             &metav1.Duration{Duration: time.Second},
		conditionAccessorFn: Failed,
		wantCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "TimedOut",
		},
	}, {
		name:                "Wait Task Retries on Timed Out",
		duration:            "2s",
		timeout:             &metav1.Duration{Duration: time.Second},
		retries:             2,
		conditionAccessorFn: Failed,
		wantCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "TimedOut",
		},
		wantRetriesStatus: []v1alpha1.RunStatus{
			{
				Status: v1.Status{
					Conditions: []apis.Condition{
						{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionFalse,
							Reason: "TimedOut",
						},
					},
					ObservedGeneration: 1,
				},
			},
			{
				Status: v1.Status{
					Conditions: []apis.Condition{
						{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionFalse,
							Reason: "TimedOut",
						},
					},
					ObservedGeneration: 1,
				},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			runName := helpers.ObjectNameForTest(t)

			run := &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name: runName,
				},
				Spec: v1alpha1.RunSpec{
					Timeout: tc.timeout,
					Retries: tc.retries,
					Ref: &v1beta1.TaskRef{
						APIVersion: apiVersion,
						Kind:       kind,
					},
					Params: []v1beta1.Param{{Name: "duration", Value: v1beta1.ParamValue{Type: "string", StringVal: tc.duration}}},
				},
			}

			if _, err := c.V1alpha1RunClient.Create(ctx, run, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Run %q: %v", runName, err)
			}

			// Wait for the Run
			if err := WaitForRunState(ctx, c, runName, time.Minute, tc.conditionAccessorFn(runName), tc.wantCondition.Reason); err != nil {
				t.Fatalf("Waiting for Run to reach status %s: %v", string(tc.wantCondition.Type), err)
			}

			// Get the actual Run
			gotRun, err := c.V1alpha1RunClient.Get(ctx, runName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("%v", err)
			}

			gotCondition := gotRun.GetStatus().GetCondition(apis.ConditionSucceeded)
			if gotCondition == nil {
				t.Fatal("The Run failed to succeed")
			}

			// Compose the expected Run
			wantRun := &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      runName,
					Namespace: namespace,
				},
				Spec: v1alpha1.RunSpec{
					Timeout: tc.timeout,
					Retries: tc.retries,
					Ref: &v1beta1.TaskRef{
						APIVersion: apiVersion,
						Kind:       kind,
					},
					Params:             []v1beta1.Param{{Name: "duration", Value: v1beta1.ParamValue{Type: "string", StringVal: tc.duration}}},
					ServiceAccountName: "default",
				},
				Status: v1alpha1.RunStatus{
					Status: v1.Status{
						Conditions:         []apis.Condition{tc.wantCondition},
						ObservedGeneration: 1,
					},
					RunStatusFields: v1alpha1.RunStatusFields{
						RetriesStatus: tc.wantRetriesStatus,
					},
				},
			}

			if d := cmp.Diff(wantRun, gotRun,
				filterTypeMeta,
				filterObjectMeta,
				filterCondition,
				filterRunStatus,
			); d != "" {
				t.Errorf("-want +got: %v", d)
			}
		})
	}
}

func TestWaitCustomTask_PipelineRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setUpCustomTask(ctx, t, requireAllGates(requiredEmbeddedStatusGate))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a custom task controller
	applyController(t)
	// Cleanup the controller after finishing the test
	defer cleanUpController(t)

	for _, tc := range []struct {
		name                  string
		runDuration           string
		runTimeout            *metav1.Duration
		runRetries            int
		prTimeout             *metav1.Duration
		prConditionAccessorFn func(string) ConditionAccessorFn
		wantPrCondition       apis.Condition
		wantRunStatus         v1alpha1.RunStatus
		wantRetriesStatus     []v1alpha1.RunStatus
	}{{
		name:                  "Wait Task Has Succeeded",
		runDuration:           "1s",
		prTimeout:             &metav1.Duration{Duration: time.Minute},
		prConditionAccessorFn: Succeed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
			Reason: "Succeeded",
		},
		wantRunStatus: v1alpha1.RunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
						Reason: "DurationElapsed",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Is Running",
		runDuration:           "2s",
		prTimeout:             &metav1.Duration{Duration: time.Second * 5},
		prConditionAccessorFn: Running,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: "Running",
		},
		wantRunStatus: v1alpha1.RunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
						Reason: "Running",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Failed When PipelineRun Is Timeout",
		runDuration:           "2s",
		prTimeout:             &metav1.Duration{Duration: time.Second},
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "PipelineRunTimeout",
		},
		wantRunStatus: v1alpha1.RunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "Cancelled",
					},
				},
				ObservedGeneration: 2,
			},
		},
	}, {
		name:                  "Wait Task Failed on Timeout",
		runDuration:           "2s",
		runTimeout:            &metav1.Duration{Duration: time.Second},
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "Failed",
		},
		wantRunStatus: v1alpha1.RunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "TimedOut",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Retries on Timeout",
		runDuration:           "2s",
		runTimeout:            &metav1.Duration{Duration: time.Second},
		runRetries:            1,
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "Failed",
		},
		wantRunStatus: v1alpha1.RunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "TimedOut",
					},
				},
				ObservedGeneration: 1,
			},
		},
		wantRetriesStatus: []v1alpha1.RunStatus{
			{
				Status: v1.Status{
					Conditions: []apis.Condition{
						{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionFalse,
							Reason: "TimedOut",
						},
					},
					ObservedGeneration: 1,
				},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prTimeout == nil {
				tc.prTimeout = &metav1.Duration{Duration: time.Minute}
			}
			p := &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helpers.ObjectNameForTest(t),
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:    "wait",
						Timeout: tc.runTimeout,
						Retries: tc.runRetries,
						TaskRef: &v1beta1.TaskRef{
							APIVersion: apiVersion,
							Kind:       kind,
						},
						Params: []v1beta1.Param{{Name: "duration", Value: v1beta1.ParamValue{Type: "string", StringVal: tc.runDuration}}},
					}},
				},
			}
			pipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helpers.ObjectNameForTest(t),
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{
						Name: p.Name,
					},
					Timeout: tc.prTimeout,
				},
			}
			if _, err := c.V1beta1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline %q: %v", p.Name, err)
			}
			if _, err := c.V1beta1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Wait for the PipelineRun to the desired state
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, tc.prConditionAccessorFn(pipelineRun.Name), string(tc.wantPrCondition.Type), v1beta1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %q to reach %s: %s", pipelineRun.Name, tc.wantPrCondition.Type, err)
			}

			// Get actual pipelineRun
			gotPipelineRun, err := c.V1beta1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Start to compose expected PipelineRun
			wantPipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
					Labels: map[string]string{
						"tekton.dev/pipeline": p.Name,
					},
				},
				Spec: v1beta1.PipelineRunSpec{
					ServiceAccountName: "default",
					PipelineRef:        &v1beta1.PipelineRef{Name: p.Name},
					Timeout:            tc.prTimeout,
				},
				Status: v1beta1.PipelineRunStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							tc.wantPrCondition,
						},
					},
					PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
						PipelineSpec: &v1beta1.PipelineSpec{
							Tasks: []v1beta1.PipelineTask{
								{
									Name:    "wait",
									Timeout: tc.runTimeout,
									Retries: tc.runRetries,
									TaskRef: &v1beta1.TaskRef{
										APIVersion: apiVersion,
										Kind:       kind,
									},
									Params: []v1beta1.Param{{
										Name:  "duration",
										Value: v1beta1.ParamValue{Type: "string", StringVal: tc.runDuration},
									}},
								},
							},
						},
					},
				},
			}

			// Compose wantStatus and wantChildStatusReferences.
			// We will look in the PipelineRunStatus.ChildReferences for wantChildStatusReferences, and will look at
			// the actual CustomRun's status for wantStatus.
			if len(gotPipelineRun.Status.ChildReferences) != 1 {
				t.Fatalf("PipelineRun had unexpected .status.childReferences; got %d, want 1", len(gotPipelineRun.Status.ChildReferences))
			}
			wantRunName := gotPipelineRun.Status.ChildReferences[0].Name
			wantPipelineRun.Status.PipelineRunStatusFields.ChildReferences = []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       pipeline.RunControllerName,
				},
				Name:             wantRunName,
				PipelineTaskName: "wait",
			}}

			wantStatus := tc.wantRunStatus
			if tc.wantRetriesStatus != nil {
				wantStatus.RetriesStatus = tc.wantRetriesStatus
			}

			// Get the Run.
			gotRun, err := c.V1alpha1RunClient.Get(ctx, wantRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get Run %q: %v", wantRunName, err)
			}

			if d := cmp.Diff(wantPipelineRun, gotPipelineRun,
				filterTypeMeta,
				filterObjectMeta,
				filterCondition,
				filterCustomRunStatus,
				filterPipelineRunStatus,
			); d != "" {
				t.Errorf("PipelineRun differed. -want, +got: %v", d)
			}

			// Compare the Run's status to what we're expecting.
			if d := cmp.Diff(wantStatus, gotRun.Status, filterCondition, filterRunStatus); d != "" {
				t.Errorf("Run status differed. -want, +got: %v", d)
			}
		})
	}
}

func TestWaitCustomTask_V1Beta1_PipelineRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setUpV1Beta1CustomTask(ctx, t, requireAllGates(requiredEmbeddedStatusGate))
	knativetest.CleanupOnInterrupt(func() { tearDownV1Beta1CustomTask(ctx, t, c, namespace) }, t.Logf)
	defer tearDownV1Beta1CustomTask(ctx, t, c, namespace)

	// Create a custom task controller
	applyV1Beta1Controller(t)
	// Cleanup the controller after finishing the test
	defer cleanUpV1Beta1Controller(t)

	for _, tc := range []struct {
		name                  string
		customRunDuration     string
		customRunTimeout      *metav1.Duration
		customRunRetries      int
		prTimeout             *metav1.Duration
		prConditionAccessorFn func(string) ConditionAccessorFn
		wantPrCondition       apis.Condition
		wantCustomRunStatus   v1beta1.CustomRunStatus
		wantRetriesStatus     []v1beta1.CustomRunStatus
	}{{
		name:                  "Wait Task Has Succeeded",
		customRunDuration:     "1s",
		prTimeout:             &metav1.Duration{Duration: time.Minute},
		prConditionAccessorFn: Succeed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
			Reason: "Succeeded",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
						Reason: "DurationElapsed",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Is Running",
		customRunDuration:     "2s",
		prTimeout:             &metav1.Duration{Duration: time.Second * 5},
		prConditionAccessorFn: Running,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: "Running",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
						Reason: "Running",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Failed When PipelineRun Is Timeout",
		customRunDuration:     "2s",
		prTimeout:             &metav1.Duration{Duration: time.Second},
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "PipelineRunTimeout",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "Cancelled",
					},
				},
				ObservedGeneration: 2,
			},
		},
	}, {
		name:                  "Wait Task Failed on Timeout",
		customRunDuration:     "2s",
		customRunTimeout:      &metav1.Duration{Duration: time.Second},
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "Failed",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "TimedOut",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Retries on Timeout",
		customRunDuration:     "2s",
		customRunTimeout:      &metav1.Duration{Duration: time.Second},
		customRunRetries:      1,
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "Failed",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "TimedOut",
					},
				},
				ObservedGeneration: 1,
			},
		},
		wantRetriesStatus: []v1beta1.CustomRunStatus{
			{
				Status: v1.Status{
					Conditions: []apis.Condition{
						{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionFalse,
							Reason: "TimedOut",
						},
					},
					ObservedGeneration: 1,
				},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prTimeout == nil {
				tc.prTimeout = &metav1.Duration{Duration: time.Minute}
			}
			p := &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helpers.ObjectNameForTest(t),
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:    "wait",
						Timeout: tc.customRunTimeout,
						Retries: tc.customRunRetries,
						TaskRef: &v1beta1.TaskRef{
							APIVersion: betaAPIVersion,
							Kind:       kind,
						},
						Params: []v1beta1.Param{{Name: "duration", Value: v1beta1.ParamValue{Type: "string", StringVal: tc.customRunDuration}}},
					}},
				},
			}
			pipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helpers.ObjectNameForTest(t),
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{
						Name: p.Name,
					},
					Timeout: tc.prTimeout,
				},
			}
			if _, err := c.V1beta1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline %q: %v", p.Name, err)
			}
			if _, err := c.V1beta1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Wait for the PipelineRun to the desired state
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, tc.prConditionAccessorFn(pipelineRun.Name), string(tc.wantPrCondition.Type), v1beta1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %q completion to be %s: %s", pipelineRun.Name, string(tc.wantPrCondition.Type), err)
			}

			// Get actual pipelineRun
			gotPipelineRun, err := c.V1beta1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Start to compose expected PipelineRun
			wantPipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
					Labels: map[string]string{
						"tekton.dev/pipeline": p.Name,
					},
				},
				Spec: v1beta1.PipelineRunSpec{
					ServiceAccountName: "default",
					PipelineRef:        &v1beta1.PipelineRef{Name: p.Name},
					Timeout:            tc.prTimeout,
				},
				Status: v1beta1.PipelineRunStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							tc.wantPrCondition,
						},
					},
					PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
						PipelineSpec: &v1beta1.PipelineSpec{
							Tasks: []v1beta1.PipelineTask{
								{
									Name:    "wait",
									Timeout: tc.customRunTimeout,
									Retries: tc.customRunRetries,
									TaskRef: &v1beta1.TaskRef{
										APIVersion: betaAPIVersion,
										Kind:       kind,
									},
									Params: []v1beta1.Param{{
										Name:  "duration",
										Value: v1beta1.ParamValue{Type: "string", StringVal: tc.customRunDuration},
									}},
								},
							},
						},
					},
				},
			}

			// Compose wantStatus and wantChildStatusReferences.
			// We will look in the PipelineRunStatus.ChildReferences for wantChildStatusReferences, and will look at
			// the actual Run's status for wantCustomRunStatus.
			if len(gotPipelineRun.Status.ChildReferences) != 1 {
				t.Fatalf("PipelineRun had unexpected .status.childReferences; got %d, want 1", len(gotPipelineRun.Status.ChildReferences))
			}
			wantCustomRunName := gotPipelineRun.Status.ChildReferences[0].Name
			wantPipelineRun.Status.PipelineRunStatusFields.ChildReferences = []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
				},
				Name:             wantCustomRunName,
				PipelineTaskName: "wait",
			}}

			wantStatus := tc.wantCustomRunStatus
			if tc.wantRetriesStatus != nil {
				wantStatus.RetriesStatus = tc.wantRetriesStatus
			}

			// Get the CustomRun.
			gotCustomRun, err := c.V1beta1CustomRunClient.Get(ctx, wantCustomRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get CustomRun %q: %v", wantCustomRunName, err)
			}

			if d := cmp.Diff(wantPipelineRun, gotPipelineRun,
				filterTypeMeta,
				filterObjectMeta,
				filterCondition,
				filterCustomRunStatus,
				filterPipelineRunStatus,
			); d != "" {
				t.Errorf("-want, +got: %v", d)
			}

			// Compare the CustomRun's status to what we're expecting.
			if d := cmp.Diff(wantStatus, gotCustomRun.Status, filterCondition, filterCustomRunStatus); d != "" {
				t.Errorf("CustomRun status differed. -want, +got: %v", d)
			}
		})
	}
}

func setUpCustomTask(ctx context.Context, t *testing.T, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string) {
	c, ns := setup(ctx, t, requireAnyGate(neededFeatureFlags))
	// Note that this may not work if we run e2e tests in parallel since this feature flag forces custom tasks to be
	// created as v1alpha1.Run with this value. i.e. Don't add t.Parallel() for this test.
	configMapData := map[string]string{
		"custom-task-version": "v1alpha1",
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}

	// Sleep 5s to

	return c, ns
}

func setUpV1Beta1CustomTask(ctx context.Context, t *testing.T, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string) {
	c, ns := setup(ctx, t, requireAnyGate(neededFeatureFlags))
	// Note that this may not work if we run e2e tests in parallel since this feature flag forces custom tasks to be
	// created as v1beta1.CustomRuns i.e. Don't add t.Parallel() for this test.
	configMapData := map[string]string{
		"custom-task-version": "v1beta1",
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}

	return c, ns
}

func tearDownV1Beta1CustomTask(ctx context.Context, t *testing.T, c *clients, namespace string) {
	t.Helper()
	configMapData := map[string]string{
		"custom-task-version": "v1alpha1",
	}
	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}

	tearDown(ctx, t, c, namespace)
}
