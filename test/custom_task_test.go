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
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	knativetest "knative.dev/pkg/test"
)

const (
	apiVersion = "example.dev/v0"
	kind       = "Example"
)

func TestCustomTask(t *testing.T) {
	c, namespace := setup(t)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	// Create a PipelineRun that runs a Custom Task.
	pipelineRunName := "custom-task-pipeline"
	if _, err := c.PipelineRunClient.Create(&v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineRunName},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "example",
					TaskRef: &v1beta1.TaskRef{
						APIVersion: apiVersion,
						Kind:       kind,
					},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRunName, err)
	}

	// Wait for the PipelineRun to start.
	if err := WaitForPipelineRunState(c, pipelineRunName, time.Minute, Running(pipelineRunName), "PipelineRunRunning"); err != nil {
		t.Fatalf("Waiting for PipelineRun to start running: %v", err)
	}

	// Get the status of the PipelineRun.
	pr, err := c.PipelineRunClient.Get(pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRunName, err)
	}

	// Get the Run name.
	if len(pr.Status.Runs) != 1 {
		t.Fatalf("PipelineRun had unexpected .status.runs; got %d, want 1", len(pr.Status.Runs))
	}
	var runName string
	for k := range pr.Status.Runs {
		runName = k
		break
	}

	// Get the Run.
	r, err := c.RunClient.Get(runName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get Run %q: %v", runName, err)
	}
	if r.IsDone() {
		t.Fatalf("Run unexpectedly done: %v", r.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Simulate a Custom Task controller updating the Run to
	// done/successful.
	r.Status = v1alpha1.RunStatus{Status: duckv1.Status{
		Conditions: duckv1.Conditions{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		}},
	}}
	if _, err := c.RunClient.UpdateStatus(r); err != nil {
		t.Fatalf("Failed to update Run to successful: %v", err)
	}

	// Get the Run.
	r, err = c.RunClient.Get(runName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get Run %q: %v", runName, err)
	}
	if !r.IsDone() {
		t.Fatalf("Run unexpectedly not done after update (UpdateStatus didn't work): %v", r.Status)
	}

	// Wait for the PipelineRun to become done/successful.
	if err := WaitForPipelineRunState(c, pipelineRunName, time.Minute, PipelineRunSucceed(pipelineRunName), "PipelineRunCompleted"); err != nil {
		t.Fatalf("Waiting for PipelineRun to complete successfully: %v", err)
	}
}
