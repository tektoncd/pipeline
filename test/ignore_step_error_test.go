//go:build e2e

/*
Copyright 2021 The Tekton Authors

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// @test:execution=parallel
func TestFailingStepOnContinue(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)
	taskRunName := "mytaskrun"
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    results:
      - name: result1
      - name: result2
    steps:
      - name: failing-step
        onError: continue
        image: mirror.gcr.io/busybox
        script: 'echo -n 123 | tee $(results.result1.path); exit 1; echo -n 456 | tee $(results.result2.path)'
`, taskRunName, namespace))

	if _, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceeded", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskRun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	if !taskRun.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
		t.Fatalf("Expected TaskRun %s to have succeeded but Status is %v", taskRunName, taskRun.Status)
	}

	expectedResults := []v1.TaskRunResult{{
		Name:  "result1",
		Type:  "string",
		Value: *v1.NewStructuredValues("123"),
	}}

	if d := cmp.Diff(expectedResults, taskRun.Status.Results); d != "" {
		t.Errorf("Got unexpected results %s", diff.PrintWantGot(d))
	}
}

// @test:execution=parallel
func TestFailingStepOnContinueAndFail(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	for _, tc := range []struct {
		name              string
		secondStepOnError string
		secondStepExit    string
	}{
		{
			name:           "successful following step",
			secondStepExit: "exit 0",
		},
		{
			name:           "failing following step",
			secondStepExit: "exit 1",
		},
		{
			name:              "following continueAndFail step",
			secondStepOnError: "onError: continueAndFail",
			secondStepExit:    "exit 1",
		},
		{
			name:              "following continue step",
			secondStepOnError: "onError: continue",
			secondStepExit:    "exit 1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			taskRunName := helpers.ObjectNameForTest(t)
			tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    results:
      - name: followed
    steps:
      - name: first-failure
        onError: continueAndFail
        image: mirror.gcr.io/busybox
        script: exit 1
      - name: following-step
        %s
        image: mirror.gcr.io/busybox
        script: echo -n ran > $(results.followed.path); %s
`, taskRunName, namespace, tc.secondStepOnError, tc.secondStepExit))

			if _, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}
			if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed", v1Version); err != nil {
				t.Fatalf("Error waiting for TaskRun to fail: %s", err)
			}

			taskRun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
			}
			condition := taskRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition.IsTrue() {
				t.Fatalf("Expected TaskRun %s to fail but Status is %v", taskRunName, taskRun.Status)
			}
			if want := `"step-first-failure" exited with code 1: Error`; condition.Message != want {
				t.Errorf("TaskRun failure message = %q, want %q", condition.Message, want)
			}

			wantResults := []v1.TaskRunResult{{
				Name:  "followed",
				Type:  "string",
				Value: *v1.NewStructuredValues("ran"),
			}}
			if d := cmp.Diff(wantResults, taskRun.Status.Results); d != "" {
				t.Errorf("Following step did not produce the expected result: %s", diff.PrintWantGot(d))
			}
		})
	}
}
