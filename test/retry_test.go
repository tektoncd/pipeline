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
	"testing"
	"time"

	"github.com/tektoncd/pipeline/test/parse"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	knativetest "knative.dev/pkg/test"
)

// TestTaskRunRetry tests that retries behave as expected, by creating multiple
// Pods for the same TaskRun each time it fails, up to the configured max.
func TestTaskRunRetry(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a PipelineRun with a single TaskRun that can only fail,
	// configured to retry 5 times.
	pipelineRunName := "retry-pipeline"
	numRetries := 5
	if _, err := c.PipelineRunClient.Create(ctx, parse.MustParsePipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineSpec:
    tasks:
    - name: retry-me
      retries: %d
      taskSpec:
        steps:
        - image: busybox
          script: exit 1
`, pipelineRunName, numRetries)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRunName, err)
	}

	// Wait for the PipelineRun to fail, when retries are exhausted.
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, 5*time.Minute, PipelineRunFailed(pipelineRunName), "PipelineRunFailed"); err != nil {
		t.Fatalf("Waiting for PipelineRun to fail: %v", err)
	}

	// Get the status of the PipelineRun.
	pr, err := c.PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRunName, err)
	}

	// PipelineRunStatus should have 1 TaskRun status, and it should be failed.
	if len(pr.Status.TaskRuns) != 1 {
		t.Errorf("Got %d TaskRun statuses, wanted %d", len(pr.Status.TaskRuns), numRetries)
	}
	for taskRunName, trs := range pr.Status.TaskRuns {
		if !isFailed(t, taskRunName, trs.Status.Conditions) {
			t.Errorf("TaskRun status %q is not failed", taskRunName)
		}
	}

	// There should only be one TaskRun created.
	trs, err := c.TaskRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list TaskRuns: %v", err)
	} else if len(trs.Items) != 1 {
		t.Fatalf("Found %d TaskRuns, want 1", len(trs.Items))
	}

	// The TaskRun status should have N retriesStatuses, all failures.
	tr := trs.Items[0]
	podNames := map[string]struct{}{}
	for idx, r := range tr.Status.RetriesStatus {
		if !isFailed(t, tr.Name, r.Conditions) {
			t.Errorf("TaskRun %q retry status %d is not failed", tr.Name, idx)
		}
		podNames[r.PodName] = struct{}{}
	}
	podNames[tr.Status.PodName] = struct{}{}
	if len(tr.Status.RetriesStatus) != numRetries {
		t.Errorf("TaskRun %q had %d retriesStatuses, want %d", tr.Name, len(tr.Status.RetriesStatus), numRetries)
	}

	// There should be N Pods created, all failed, all owned by the TaskRun.
	pods, err := c.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	// We expect N+1 Pods total, one for each failed and retried attempt, and one for the final attempt.
	wantPods := numRetries + 1

	if err != nil {
		t.Fatalf("Failed to list Pods: %v", err)
	} else if len(pods.Items) != wantPods {
		t.Errorf("BUG: Found %d Pods, want %d", len(pods.Items), wantPods)
	}
	for _, p := range pods.Items {
		if _, found := podNames[p.Name]; !found {
			t.Errorf("BUG: TaskRunStatus.RetriesStatus did not report pod name %q", p.Name)
		}
		if p.Status.Phase != corev1.PodFailed {
			t.Errorf("BUG: Pod %q is not failed: %v", p.Name, p.Status.Phase)
		}
	}
}

// This method is necessary because PipelineRunTaskRunStatus and TaskRunStatus
// don't have an IsFailed method.
func isFailed(t *testing.T, taskRunName string, conds duckv1beta1.Conditions) bool {
	for _, c := range conds {
		if c.Type == apis.ConditionSucceeded {
			if c.Status != corev1.ConditionFalse {
				t.Errorf("TaskRun status %q is not failed, got %q", taskRunName, c.Status)
			}
			return true
		}
	}
	t.Errorf("TaskRun status %q had no Succeeded condition", taskRunName)
	return false
}
