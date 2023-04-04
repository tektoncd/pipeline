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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestCustomTaskField(t *testing.T) {
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
        customTask: %s
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
`, pipelineRunName, kind, apiVersion, kind, customTaskRawSpec)),
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
		if cr.Kind == pipeline.CustomRunControllerName {
			runNames = append(runNames, cr.Name)
		}
	}
	if len(runNames) != 2 {
		t.Fatalf("PipelineRun had unexpected number of Runs in .status.childReferences; got %d, want 2", len(runNames))
	}
	for _, runName := range runNames {
		// Get the Run.
		cr, err := c.V1beta1CustomRunClient.Get(ctx, runName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get Run %q: %v", runName, err)
		}
		if cr.IsDone() {
			t.Fatalf("Run unexpectedly done: %v", cr.Status.GetCondition(apis.ConditionSucceeded))
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
			t.Fatalf("Failed to update Run to successful: %v", err)
		}

		// Get the Run.
		cr, err = c.V1beta1CustomRunClient.Get(ctx, runName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get Run %q: %v", runName, err)
		}

		if strings.Contains(runName, "custom-task-spec") {
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
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, time.Minute, PipelineRunSucceed(pipelineRunName), "PipelineRunCompleted", v1beta1Version); err != nil {
		t.Fatalf("Waiting for PipelineRun to complete successfully: %v", err)
	}

	// Get the updated status of the PipelineRun.
	pr, err = c.V1beta1PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q after it completed: %v", pipelineRunName, err)
	}

	// check if the "custom-task-ref"'s taskRef
	expectedTaskRef := v1beta1.TaskRef{Kind: kind, APIVersion: v1beta1.CloudBuildApiVersion, CustomTask: kind}
	for _, pt := range pr.Status.PipelineSpec.Tasks {
		if pt.Name == "custom-task-ref" {
			if d := cmp.Diff(expectedTaskRef, *pt.TaskRef); d != "" {
				t.Fatalf("Unexpected custom-task-ref: %s", diff.PrintWantGot(d))
			}
		}
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
	expectedTaskRunParams := v1beta1.Params{{
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
