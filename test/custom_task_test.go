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

	"github.com/tektoncd/pipeline/test/parse"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	apiVersion  = "wait.testing.tekton.dev/v1alpha1"
	kind        = "Wait"
	waitTaskDir = "./custom-task-ctrls/wait-task"
)

var (
	supportedFeatureGates = map[string]string{
		"enable-custom-tasks": "true",
		"enable-api-fields":   "alpha",
	}

	filterTypeMeta          = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	filterObjectMeta        = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields")
	filterCondition         = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time", "Message")
	filterRunStatus         = cmpopts.IgnoreFields(v1alpha1.RunStatusFields{}, "StartTime", "CompletionTime")
	filterPipelineRunStatus = cmpopts.IgnoreFields(v1beta1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")
)

func TestCustomTask(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(supportedFeatureGates))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	embeddedStatusValue := GetEmbeddedStatus(ctx, t, c.KubeClient)

	customTaskRawSpec := []byte(`{"field1":123,"field2":"value"}`)
	metadataLabel := map[string]string{"test-label": "test"}
	// Create a PipelineRun that runs a Custom Task.
	pipelineRunName := helpers.ObjectNameForTest(t)
	if _, err := c.PipelineRunClient.Create(
		ctx,
		parse.MustParsePipelineRun(t, fmt.Sprintf(`
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
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, time.Minute, Running(pipelineRunName), "PipelineRunRunning"); err != nil {
		t.Fatalf("Waiting for PipelineRun to start running: %v", err)
	}

	// Get the status of the PipelineRun.
	pr, err := c.PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRunName, err)
	}

	// Get the Run name.
	var runNames []string
	if embeddedStatusValue != config.MinimalEmbeddedStatus {
		if len(pr.Status.Runs) != 2 {
			t.Fatalf("PipelineRun had unexpected .status.runs; got %d, want 2", len(pr.Status.Runs))
		}
		for rn := range pr.Status.Runs {
			runNames = append(runNames, rn)
		}
	}
	if embeddedStatusValue != config.FullEmbeddedStatus {
		for _, cr := range pr.Status.ChildReferences {
			if cr.Kind == "Run" {
				runNames = append(runNames, cr.Name)
			}
		}
		if len(runNames) != 2 {
			t.Fatalf("PipelineRun had unexpected number of Runs in .status.childReferences; got %d, want 2", len(runNames))
		}
	}
	for _, runName := range runNames {
		// Get the Run.
		r, err := c.RunClient.Get(ctx, runName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get Run %q: %v", runName, err)
		}
		if r.IsDone() {
			t.Fatalf("Run unexpectedly done: %v", r.Status.GetCondition(apis.ConditionSucceeded))
		}

		// Simulate a Custom Task controller updating the Run to done/successful.
		r.Status = v1alpha1.RunStatus{
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

		if _, err := c.RunClient.UpdateStatus(ctx, r, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Failed to update Run to successful: %v", err)
		}

		// Get the Run.
		r, err = c.RunClient.Get(ctx, runName, metav1.GetOptions{})

		if strings.Contains(runName, "custom-task-spec") {
			if d := cmp.Diff(customTaskRawSpec, r.Spec.Spec.Spec.Raw); d != "" {
				t.Fatalf("Unexpected value of Spec.Raw: %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(metadataLabel, r.Spec.Spec.Metadata.Labels); d != "" {
				t.Fatalf("Unexpected value of Metadata.Labels: %s", diff.PrintWantGot(d))
			}
		}
		if err != nil {
			t.Fatalf("Failed to get Run %q: %v", runName, err)
		}
		if !r.IsDone() {
			t.Fatalf("Run unexpectedly not done after update (UpdateStatus didn't work): %v", r.Status)
		}
	}
	// Wait for the PipelineRun to become done/successful.
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, time.Minute, PipelineRunSucceed(pipelineRunName), "PipelineRunCompleted"); err != nil {
		t.Fatalf("Waiting for PipelineRun to complete successfully: %v", err)
	}

	// Get the updated status of the PipelineRun.
	pr, err = c.PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q after it completed: %v", pipelineRunName, err)
	}

	// Get the TaskRun name.
	var taskRunName string

	if embeddedStatusValue != config.MinimalEmbeddedStatus {
		if len(pr.Status.TaskRuns) != 1 {
			t.Fatalf("PipelineRun had unexpected .status.taskRuns; got %d, want 1", len(pr.Status.TaskRuns))
		}
		for k := range pr.Status.TaskRuns {
			taskRunName = k
			break
		}
	}
	if embeddedStatusValue != config.FullEmbeddedStatus {
		for _, cr := range pr.Status.ChildReferences {
			if cr.Kind == "TaskRun" {
				taskRunName = cr.Name
			}
		}
		if taskRunName == "" {
			t.Fatal("PipelineRun does not have expected TaskRun in .status.childReferences")
		}
	}

	// Get the TaskRun.
	taskRun, err := c.TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
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
		r, err := c.RunClient.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return r.Spec.Status == v1alpha1.RunSpecStatusCancelled, nil
	})
}

// TestPipelineRunCustomTaskTimeout is an integration test that will
// verify that pipelinerun timeout works and leads to the the correct Run Spec.status
func TestPipelineRunCustomTaskTimeout(t *testing.T) {
	// cancel the context after we have waited a suitable buffer beyond the given deadline.
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Minute)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(supportedFeatureGates))

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	embeddedStatusValue := GetEmbeddedStatus(ctx, t, c.KubeClient)

	pipeline := parse.MustParsePipeline(t, fmt.Sprintf(`
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
	pipelineRun := parse.MustParsePipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  timeout: 5s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, Running(pipelineRun.Name), "PipelineRunRunning"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be running: %s", pipelineRun.Name, err)
	}

	pr, err := c.PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
	}

	// Get the Run name.
	runName := ""

	if embeddedStatusValue != config.MinimalEmbeddedStatus {
		if len(pr.Status.Runs) != 1 {
			t.Fatalf("PipelineRun had unexpected .status.runs; got %d, want 1", len(pr.Status.Runs))
		}
		for rn := range pr.Status.Runs {
			runName = rn
		}
	}
	if embeddedStatusValue != config.FullEmbeddedStatus {
		if len(pr.Status.ChildReferences) != 1 {
			t.Fatalf("PipelineRun had unexpected .status.childReferences; got %d, want 1", len(pr.Status.ChildReferences))
		}
		runName = pr.Status.ChildReferences[0].Name
	}

	// Get the Run.
	r, err := c.RunClient.Get(ctx, runName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get Run %q: %v", runName, err)
	}
	if r.IsDone() {
		t.Fatalf("Run unexpectedly done: %v", r.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Simulate a Custom Task controller updating the Run to be started/running,
	// because, a run that has not started cannot timeout.
	r.Status = v1alpha1.RunStatus{
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
	if _, err := c.RunClient.UpdateStatus(ctx, r, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to update Run to successful: %v", err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be timed out", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1beta1.PipelineRunReasonTimedOut.String(), pipelineRun.Name), "PipelineRunTimedOut"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	runList, err := c.RunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing Runs for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	t.Logf("Runs from PipelineRun %s in namespace %s must be cancelled", pipelineRun.Name, namespace)
	var wg sync.WaitGroup
	for _, runItem := range runList.Items {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			err := WaitForRunSpecCancelled(ctx, c, name, "RunCancelled")
			if err != nil {
				t.Errorf("Error waiting for Run %s to cancel: %s", name, err)
			}
		}(runItem.Name)
	}
	wg.Wait()

	if _, err := c.PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{}); err != nil {
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

func cleanUpController(t *testing.T) {
	t.Log("Tearing down Wait Custom Task Controller...")
	cmd := exec.Command("ko", "delete", "-f", "./config/controller.yaml")
	cmd.Dir = waitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to tear down Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func TestWaitCustomTask_Run(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(supportedFeatureGates))
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
		timeout:             &metav1.Duration{time.Second},
		conditionAccessorFn: Failed,
		wantCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "TimedOut",
		},
	}, {
		name:                "Wait Task Retries on Timed Out",
		duration:            "2s",
		timeout:             &metav1.Duration{time.Second},
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

			if _, err := c.RunClient.Create(ctx, run, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun %q: %v", runName, err)
			}

			// Wait for the Run
			if err := WaitForRunState(ctx, c, runName, time.Minute, tc.conditionAccessorFn(runName), tc.wantCondition.Reason); err != nil {
				t.Fatalf("Waiting for Run to finish running: %v", err)
			}

			// Get the actual Run
			gotRun, err := c.RunClient.Get(ctx, runName, metav1.GetOptions{})
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
				t.Errorf("-got +want: %v", d)
			}
		})
	}
}

func TestWaitCustomTask_PipelineRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(supportedFeatureGates))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a custom task controller
	applyController(t)
	// Cleanup the controller after finishing the test
	defer cleanUpController(t)

	embeddedStatusValue := GetEmbeddedStatus(ctx, t, c.KubeClient)

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
		prTimeout:             &metav1.Duration{time.Second * 60},
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
		prTimeout:             &metav1.Duration{time.Second * 5},
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
		prTimeout:             &metav1.Duration{time.Second},
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
		runTimeout:            &metav1.Duration{time.Second},
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
		runTimeout:            &metav1.Duration{time.Second},
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
				tc.prTimeout = &metav1.Duration{time.Second * 60}
			}
			pipeline := &v1beta1.Pipeline{
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
						Name: pipeline.Name,
					},
					Timeout: tc.prTimeout,
				},
			}
			if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline %q: %v", pipeline.Name, err)
			}
			if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Wait for the PipelineRun to the desired state
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, tc.prConditionAccessorFn(pipelineRun.Name), string(tc.wantPrCondition.Type)); err != nil {
				t.Fatalf("Error waiting for PipelineRun %q to be running: %s", pipelineRun.Name, err)
			}

			// Get actual pipelineRun
			gotPipelineRun, err := c.PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Start to compose expected PipelineRun
			wantPipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
					Labels: map[string]string{
						"tekton.dev/pipeline": pipeline.Name,
					},
				},
				Spec: v1beta1.PipelineRunSpec{
					ServiceAccountName: "default",
					PipelineRef:        &v1beta1.PipelineRef{Name: pipeline.Name},
					Timeout:            tc.prTimeout,
				},
				Status: v1beta1.PipelineRunStatus{
					Status: duckv1beta1.Status{
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

			// Compose wantRunStatus and wantChildStatusReferences.
			// If config.BothEmbeddedStatus is set, both runStatus and childStatusReferences should exist in PipelineRunStatus
			// If config.MinimalEmbeddedStatus is set, only childStatusReferences should be included.
			// If config.FullEmbeddedStatus is set, only runStatus should be included.
			wantRunName := ""
			if embeddedStatusValue != config.MinimalEmbeddedStatus {
				wantRunsStatus := &v1beta1.PipelineRunRunStatus{}
				if len(gotPipelineRun.Status.Runs) != 1 {
					t.Fatalf("PipelineRun had unexpected .status.runs; got %d, want 1", len(gotPipelineRun.Status.Runs))
				}
				for rn := range gotPipelineRun.Status.Runs {
					wantRunName = rn
					wantRunsStatus.PipelineTaskName = "wait"
					wantRunsStatus.Status = &tc.wantRunStatus
					if tc.wantRetriesStatus != nil {
						wantRunsStatus.Status.RetriesStatus = tc.wantRetriesStatus
					}
				}
				if tc.wantRetriesStatus != nil {
					wantRunsStatus.Status.RetriesStatus = tc.wantRetriesStatus
				}
				wantPipelineRun.Status.PipelineRunStatusFields.Runs = map[string]*v1beta1.PipelineRunRunStatus{
					wantRunName: wantRunsStatus,
				}
			}
			if embeddedStatusValue != config.FullEmbeddedStatus {
				if len(gotPipelineRun.Status.ChildReferences) != 1 {
					t.Fatalf("PipelineRun had unexpected .status.childReferences; got %d, want 1", len(gotPipelineRun.Status.ChildReferences))
				}
				wantRunName = gotPipelineRun.Status.ChildReferences[0].Name
				wantPipelineRun.Status.PipelineRunStatusFields.ChildReferences = []v1beta1.ChildStatusReference{{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "tekton.dev/v1alpha1",
						Kind:       "Run",
					},
					Name:             wantRunName,
					PipelineTaskName: "wait",
				}}
			}

			// Get the Run.
			_, err = c.RunClient.Get(ctx, wantRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get Run %q: %v", wantRunName, err)
			}

			if d := cmp.Diff(wantPipelineRun, gotPipelineRun,
				filterTypeMeta,
				filterObjectMeta,
				filterCondition,
				filterRunStatus,
				filterPipelineRunStatus,
			); d != "" {
				t.Errorf("-got, +want: %v", d)
			}
		})
	}
}
