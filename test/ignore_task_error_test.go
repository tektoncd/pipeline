//go:build e2e

/*
Copyright 2023 The Tekton Authors

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
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

// @test:execution=parallel
func TestFailingPipelineTaskOnContinue(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{"enable-api-fields": "beta"}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	prName := "my-pipelinerun"
	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineSpec:
    tasks:
    - name: failed-ignored-task
      onError: continue
      taskSpec:
        results:
        - name: result1
          type: string
        steps:
        - name: failing-step
          image: mirror.gcr.io/busybox
          script: 'exit 1; echo -n 123 | tee $(results.result1.path)'
    - name: order-dep-task
      runAfter: ["failed-ignored-task"]
      taskSpec:
        steps:
        - name: foo
          image: mirror.gcr.io/busybox
          script: 'echo hello'
    - name: resource-dep-task
      onError: continue
      params:
      - name: param1
        value: $(tasks.failed-ignored-task.results.result1)
      taskSpec:
        params:
        - name: param1
          type: string
        steps:
        - name: foo
          image: mirror.gcr.io/busybox
          script: 'echo $(params.param1)'
`, prName, namespace))

	if _, err := c.V1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	// wait for PipelineRun to finish
	t.Logf("Waiting for PipelineRun in namespace %s to finish", namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSucceeded", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish: %s", err)
	}

	// validate pipelinerun success, with right TaskRun counts
	pr, err := c.V1PipelineRunClient.Get(ctx, "my-pipelinerun", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun my-pipelinerun: %s", err)
	}
	cond := pr.Status.Conditions[0]
	if cond.Status != corev1.ConditionTrue {
		t.Fatalf("Expect my-pipelinerun to success but got: %s", cond)
	}
	expectErrMsg := "Tasks Completed: 2 (Failed: 1 (Ignored: 1), Cancelled 0), Skipped: 1"
	if d := cmp.Diff(expectErrMsg, cond.Message); d != "" {
		t.Errorf("Got unexpected error message %s", diff.PrintWantGot(d))
	}

	// validate first TaskRun to fail but ignored
	failedTaskRun, err := c.V1TaskRunClient.Get(ctx, "my-pipelinerun-failed-ignored-task", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun my-pipelinerun-failed-ignored-task: %s", err)
	}
	cond = failedTaskRun.Status.Conditions[0]
	if cond.Status != corev1.ConditionFalse || cond.Reason != string(v1.TaskRunReasonFailureIgnored) {
		t.Errorf("Expect failed-ignored-task Task in Failed status with reason %s, but got %s status with reason: %s", v1.TaskRunReasonFailureIgnored, cond.Status, cond.Reason)
	}

	// validate second TaskRun succeeded
	orderDepTaskRun, err := c.V1TaskRunClient.Get(ctx, "my-pipelinerun-order-dep-task", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun my-pipelinerun-order-dep-task: %s", err)
	}
	cond = orderDepTaskRun.Status.Conditions[0]
	if cond.Status != corev1.ConditionTrue {
		t.Errorf("Expect order-dep-task Task to success but got: %s", cond)
	}
}
