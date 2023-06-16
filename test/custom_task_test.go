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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	kind            = "Wait"
	betaAPIVersion  = "wait.testing.tekton.dev/v1beta1"
	betaWaitTaskDir = "./custom-task-ctrls/wait-task-beta"
)

var (
	filterTypeMeta          = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	filterObjectMeta        = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields")
	filterCondition         = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time", "Message")
	filterCustomRunStatus   = cmpopts.IgnoreFields(v1beta1.CustomRunStatusFields{}, "StartTime", "CompletionTime")
	filterPipelineRunStatus = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")
)

func TestCustomTask(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	customTaskRawSpec := []byte(`{"field1":123,"field2":"value"}`)
	metadataLabel := map[string]string{"test-label": "test"}
	// Create a PipelineRun that runs a Custom Task.
	pipelineRunName := helpers.ObjectNameForTest(t)
	if _, err := c.V1PipelineRunClient.Create(
		ctx,
		parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
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
`, pipelineRunName, betaAPIVersion, kind, betaAPIVersion, kind, customTaskRawSpec)),
		metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRunName, err)
	}

	// Wait for the PipelineRun to start.
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, time.Minute, Running(pipelineRunName), "PipelineRunRunning", v1Version); err != nil {
		t.Fatalf("Waiting for PipelineRun to start running: %v", err)
	}

	// Get the status of the PipelineRun.
	pr, err := c.V1PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRunName, err)
	}

	// Get the Run name.
	var customRunNames []string
	for _, cr := range pr.Status.ChildReferences {
		if cr.Kind == pipeline.CustomRunControllerName {
			customRunNames = append(customRunNames, cr.Name)
		}
	}
	if len(customRunNames) != 2 {
		t.Fatalf("PipelineRun had unexpected number of CustomRuns in .status.childReferences; got %d, want 2", len(customRunNames))
	}
	for _, customRunName := range customRunNames {
		// Get the CustomRun.
		cr, err := c.V1beta1CustomRunClient.Get(ctx, customRunName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get CustomRun %q: %v", customRunName, err)
		}
		if cr.IsDone() {
			t.Fatalf("CustomRun unexpectedly done: %v", cr.Status.GetCondition(apis.ConditionSucceeded))
		}

		// Simulate a Custom Task controller updating the CustomRun to done/successful.
		cr.Status = v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				Results: []v1beta1.CustomRunResult{{
					Name:  "runResult",
					Value: "aResultValue",
				}},
			},
		}

		if _, err := c.V1beta1CustomRunClient.UpdateStatus(ctx, cr, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Failed to update CustomRun to successful: %v", err)
		}

		// Get the CustomRun.
		cr, err = c.V1beta1CustomRunClient.Get(ctx, customRunName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get CustomRun %q: %v", customRunName, err)
		}

		if strings.Contains(customRunName, "custom-task-spec") {
			if d := cmp.Diff(customTaskRawSpec, cr.Spec.CustomSpec.Spec.Raw); d != "" {
				t.Fatalf("Unexpected value of Spec.Raw: %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(metadataLabel, cr.Spec.CustomSpec.Metadata.Labels); d != "" {
				t.Fatalf("Unexpected value of Metadata.Labels: %s", diff.PrintWantGot(d))
			}
		}
		if !cr.IsDone() {
			t.Fatalf("Run unexpectedly not done after update (UpdateStatus didn't work): %v", cr.Status)
		}
	}
	// Wait for the PipelineRun to become done/successful.
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, time.Minute, PipelineRunSucceed(pipelineRunName), "PipelineRunCompleted", v1Version); err != nil {
		t.Fatalf("Waiting for PipelineRun to complete successfully: %v", err)
	}

	// Get the updated status of the PipelineRun.
	pr, err = c.V1PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
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
	taskRun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get TaskRun %q: %v", taskRunName, err)
	}

	// Validate the task's result reference to the custom task's result was resolved.
	expectedTaskRunParams := v1.Params{{
		Name: "input-result-from-custom-task-ref", Value: *v1.NewStructuredValues("aResultValue"),
	}, {
		Name: "input-result-from-custom-task-spec", Value: *v1.NewStructuredValues("aResultValue"),
	}}

	if d := cmp.Diff(expectedTaskRunParams, taskRun.Spec.Params); d != "" {
		t.Fatalf("Unexpected TaskRun Params: %s", diff.PrintWantGot(d))
	}

	// Validate that the pipeline's result reference to the custom task's result was resolved.

	expectedPipelineResults := []v1.PipelineRunResult{{
		Name:  "prResult-ref",
		Value: *v1.NewStructuredValues("aResultValue"),
	}, {
		Name:  "prResult-spec",
		Value: *v1.NewStructuredValues("aResultValue"),
	}}

	if len(pr.Status.Results) != 2 {
		t.Fatalf("Expected 2 PipelineResults but there are %d.", len(pr.Status.Results))
	}
	if d := cmp.Diff(expectedPipelineResults, pr.Status.Results); d != "" {
		t.Fatalf("Unexpected PipelineResults: %s", diff.PrintWantGot(d))
	}
}

// WaitForCustomRunSpecCancelled polls the spec.status of the Run until it is
// "RunCancelled", returns an error on timeout. desc will be used to name
// the metric that is emitted to track how long it took.
func WaitForCustomRunSpecCancelled(ctx context.Context, c *clients, name string, desc string) error {
	metricName := fmt.Sprintf("WaitForRunSpecCancelled/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return pollImmediateWithContext(ctx, func() (bool, error) {
		cr, err := c.V1beta1CustomRunClient.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return cr.Spec.Status == v1beta1.CustomRunSpecStatusCancelled, nil
	})
}

// TestPipelineRunCustomTaskTimeout is an integration test that will
// verify that pipelinerun timeout works and leads to the correct Run Spec.status
func TestPipelineRunCustomTaskTimeout(t *testing.T) {
	// cancel the context after we have waited a suitable buffer beyond the given deadline.
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Minute)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: custom-task-ref
    taskRef:
      apiVersion: %s
      kind: %s
`, helpers.ObjectNameForTest(t), namespace, betaAPIVersion, kind))
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  timeouts:
    pipeline: 5s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, Running(pipelineRun.Name), "PipelineRunRunning", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be running: %s", pipelineRun.Name, err)
	}

	pr, err := c.V1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
	}

	// Get the CustomRun name.
	customRunName := ""

	if len(pr.Status.ChildReferences) != 1 {
		t.Fatalf("PipelineRun had unexpected .status.childReferences; got %d, want 1", len(pr.Status.ChildReferences))
	}
	customRunName = pr.Status.ChildReferences[0].Name

	// Get the Run.
	cr, err := c.V1beta1CustomRunClient.Get(ctx, customRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get Run %q: %v", customRunName, err)
	}
	if cr.IsDone() {
		t.Fatalf("Run unexpectedly done: %v", cr.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Simulate a Custom Task controller updating the Run to be started/running,
	// because, a Run that has not started cannot timeout.
	cr.Status = v1beta1.CustomRunStatus{
		CustomRunStatusFields: v1beta1.CustomRunStatusFields{
			StartTime: &metav1.Time{Time: time.Now()},
		},
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionUnknown,
			}},
		},
	}
	if _, err := c.V1beta1CustomRunClient.UpdateStatus(ctx, cr, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to update CustomRun to successful: %v", err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be timed out", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1.PipelineRunReasonTimedOut.String(), pipelineRun.Name), "PipelineRunTimedOut", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	customRunList, err := c.V1beta1CustomRunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing Runs for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	t.Logf("Runs from PipelineRun %s in namespace %s must be cancelled", pipelineRun.Name, namespace)
	var wg sync.WaitGroup
	for _, crItem := range customRunList.Items {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			err := WaitForCustomRunSpecCancelled(ctx, c, name, "RunCancelled")
			if err != nil {
				t.Errorf("Error waiting for Run %s to cancel: %s", name, err)
			}
		}(crItem.Name)
	}
	wg.Wait()

	if _, err := c.V1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRun.Name, err)
	}
}

func applyV1Beta1Controller(t *testing.T) {
	t.Helper()
	t.Log("Creating Wait v1beta1.CustomRun Custom Task Controller...")
	cmd := exec.Command("ko", "apply", "--platform", "linux/amd64,linux/s390x,linux/ppc64le", "-f", "./config/controller.yaml")
	cmd.Dir = betaWaitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func cleanUpV1beta1Controller(t *testing.T) {
	t.Helper()
	t.Log("Tearing down Wait v1beta1.CustomRun Custom Task Controller...")
	cmd := exec.Command("ko", "delete", "-f", "./config/controller.yaml")
	cmd.Dir = betaWaitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to tear down Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func TestWaitCustomTask_V1_PipelineRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a custom task controller
	applyV1Beta1Controller(t)
	// Cleanup the controller after finishing the test
	defer cleanUpV1beta1Controller(t)

	featureFlags := getFeatureFlagsBaseOnAPIFlag(t)

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
			Status: duckv1.Status{
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
			Status: duckv1.Status{
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
			Status: duckv1.Status{
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
			Status: duckv1.Status{
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
			Status: duckv1.Status{
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
				Status: duckv1.Status{
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
			p := &v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helpers.ObjectNameForTest(t),
					Namespace: namespace,
				},
				Spec: v1.PipelineSpec{
					Tasks: []v1.PipelineTask{{
						Name:    "wait",
						Timeout: tc.customRunTimeout,
						Retries: tc.customRunRetries,
						TaskRef: &v1.TaskRef{
							APIVersion: betaAPIVersion,
							Kind:       kind,
						},
						Params: v1.Params{{Name: "duration", Value: v1.ParamValue{Type: "string", StringVal: tc.customRunDuration}}},
					}},
				},
			}
			pipelineRun := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helpers.ObjectNameForTest(t),
					Namespace: namespace,
				},
				Spec: v1.PipelineRunSpec{
					PipelineRef: &v1.PipelineRef{
						Name: p.Name,
					},
					Timeouts: &v1.TimeoutFields{
						Pipeline: tc.prTimeout,
					},
				},
			}
			if _, err := c.V1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline %q: %v", p.Name, err)
			}
			if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Wait for the PipelineRun to the desired state
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, tc.prConditionAccessorFn(pipelineRun.Name), string(tc.wantPrCondition.Type), v1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %q completion to be %s: %s", pipelineRun.Name, string(tc.wantPrCondition.Type), err)
			}

			// Get actual pipelineRun
			gotPipelineRun, err := c.V1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Start to compose expected PipelineRun
			wantPipelineRun := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
					Labels: map[string]string{
						"tekton.dev/pipeline": p.Name,
					},
				},
				Spec: v1.PipelineRunSpec{
					TaskRunTemplate: v1.PipelineTaskRunTemplate{
						ServiceAccountName: "default",
					},
					PipelineRef: &v1.PipelineRef{Name: p.Name},
					Timeouts: &v1.TimeoutFields{
						Pipeline: tc.prTimeout,
					},
				},
				Status: v1.PipelineRunStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							tc.wantPrCondition,
						},
					},
					PipelineRunStatusFields: v1.PipelineRunStatusFields{
						PipelineSpec: &v1.PipelineSpec{
							Tasks: []v1.PipelineTask{
								{
									Name:    "wait",
									Timeout: tc.customRunTimeout,
									Retries: tc.customRunRetries,
									TaskRef: &v1.TaskRef{
										APIVersion: betaAPIVersion,
										Kind:       kind,
									},
									Params: v1.Params{{
										Name:  "duration",
										Value: v1.ParamValue{Type: "string", StringVal: tc.customRunDuration},
									}},
								},
							},
						},
						Provenance: &v1.Provenance{
							FeatureFlags: featureFlags,
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
			wantPipelineRun.Status.PipelineRunStatusFields.ChildReferences = []v1.ChildStatusReference{{
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

// updateConfigMap updates the config map for specified @name with values. We can't use the one from knativetest because
// it assumes that Data is already a non-nil map, and by default, it isn't!
func updateConfigMap(ctx context.Context, client kubernetes.Interface, name string, configName string, values map[string]string) error {
	configMap, err := client.CoreV1().ConfigMaps(name).Get(ctx, configName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	for key, value := range values {
		configMap.Data[key] = value
	}

	_, err = client.CoreV1().ConfigMaps(name).Update(ctx, configMap, metav1.UpdateOptions{})
	return err
}

func resetConfigMap(ctx context.Context, t *testing.T, c *clients, namespace, configName string, values map[string]string) {
	t.Helper()
	if err := updateConfigMap(ctx, c.KubeClient, namespace, configName, values); err != nil {
		t.Log(err)
	}
}

func getFeatureFlagsBaseOnAPIFlag(t *testing.T) *config.FeatureFlags {
	t.Helper()
	alphaFeatureFlags, err := config.NewFeatureFlagsFromMap(map[string]string{
		"enable-api-fields":         "alpha",
		"results-from":              "sidecar-logs",
		"enable-tekton-oci-bundles": "true",
	})
	if err != nil {
		t.Fatalf("error creating alpha feature flags configmap: %v", err)
	}
	betaFeatureFlags, err := config.NewFeatureFlagsFromMap(map[string]string{
		"enable-api-fields": "beta",
	})
	if err != nil {
		t.Fatalf("error creating beta feature flags configmap: %v", err)
	}
	stableFeatureFlags, err := config.NewFeatureFlagsFromMap(map[string]string{
		"enable-api-fields": "stable",
	})
	if err != nil {
		t.Fatalf("error creating stable feature flags configmap: %v", err)
	}
	enabledFeatureGate, err := getAPIFeatureGate()
	if err != nil {
		t.Fatalf("error reading enabled feature gate: %v", err)
	}
	switch enabledFeatureGate {
	case "alpha":
		return alphaFeatureFlags
	case "beta":
		return betaFeatureFlags
	default:
		return stableFeatureFlags
	}
}

// getAPIFeatureGate queries the tekton pipelines namespace for the
// current value of the "enable-api-fields" feature gate.
func getAPIFeatureGate() (string, error) {
	ns := os.Getenv("SYSTEM_NAMESPACE")
	if ns == "" {
		ns = "tekton-pipelines"
	}

	cmd := exec.Command("kubectl", "get", "configmap", "feature-flags", "-n", ns, "-o", `jsonpath="{.data['enable-api-fields']}"`)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting feature-flags configmap: %w", err)
	}
	output = bytes.TrimSpace(output)
	output = bytes.Trim(output, "\"")
	if len(output) == 0 {
		output = []byte("stable")
	}
	return string(output), nil
}
