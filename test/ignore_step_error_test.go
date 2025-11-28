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
	"strings"
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

var (
	onErrorContinue        = "continue"
	onErrorContinueAndFail = "continueAndFail"
)

func successfulStep(name string, onError *string) string {
	step := fmt.Sprintf(`
      - name: %s
        image: mirror.gcr.io/busybox
        script: 'echo hello'`, name)

	if onError != nil {
		step += "\n" + `
        onError: ` + *onError
	}

	return step
}

func failingStep(name string, onError *string) string {
	step := fmt.Sprintf(`
      - name: %s
        image: mirror.gcr.io/busybox
        script: 'echo -n 123 | tee $(results.result1.path); exit 1; echo -n 456 | tee $(results.result2.path)'`, name)

	if onError != nil {
		step += "\n" + `
        onError: ` + *onError
	}

	return step
}

func TestFailingStepOnContinue(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	tests := []struct {
		description       string
		steps             []string
		shouldSucceed     bool
		expectedStatusMsg string
		state             string
		conditionFn       func(name string) ConditionAccessorFn
	}{
		{
			description: "failing step with onError continue and successful follow-up step",
			steps: []string{
				failingStep("failing-step", &onErrorContinue),
				successfulStep("successful-step", nil),
			},
			shouldSucceed:     true,
			expectedStatusMsg: "All Steps have completed executing",
			state:             "TaskRunSucceeded",
			conditionFn:       TaskRunSucceed,
		},
		{
			description: "failing step with onError continueAndFail and successful follow-up step",
			steps: []string{
				failingStep("failing-step", &onErrorContinueAndFail),
				successfulStep("successful-step", nil),
			},
			shouldSucceed:     false,
			expectedStatusMsg: `"step-failing-step" exited with code 1`,
			state:             "TaskRunFailed",
			conditionFn:       TaskRunFailed,
		},
		{
			description: "failing step with onError continue and failing follow-up step",
			steps: []string{
				failingStep("failing-step-1", &onErrorContinue),
				failingStep("failing-step-2", nil),
			},
			shouldSucceed:     false,
			expectedStatusMsg: `"step-failing-step-2" exited with code 1`,
			state:             "TaskRunFailed",
			conditionFn:       TaskRunFailed,
		},
		{
			description: "failing step with onError continueAndFail and failing follow-up step",
			steps: []string{
				failingStep("failing-step-1", &onErrorContinueAndFail),
				failingStep("failing-step-2", nil),
			},
			shouldSucceed:     false,
			expectedStatusMsg: `"step-failing-step-1" exited with code 1`,
			state:             "TaskRunFailed",
			conditionFn:       TaskRunFailed,
		},
		{
			description: "failing step with onError continue and failing follow-up step with onError continueAndFail",
			steps: []string{
				failingStep("failing-step-1", &onErrorContinue),
				failingStep("failing-step-2", &onErrorContinueAndFail),
			},
			shouldSucceed:     false,
			expectedStatusMsg: `"step-failing-step-2" exited with code 1`,
			state:             "TaskRunFailed",
			conditionFn:       TaskRunFailed,
		},
		{
			description: "failing step with onError continueAndFail and failing follow-up step with onError continueAndFail",
			steps: []string{
				failingStep("failing-step-1", &onErrorContinueAndFail),
				failingStep("failing-step-2", &onErrorContinueAndFail),
			},
			shouldSucceed:     false,
			expectedStatusMsg: `"step-failing-step-1" exited with code 1`,
			state:             "TaskRunFailed",
			conditionFn:       TaskRunFailed,
		},
		{
			description: "failing step with onError continue and failing follow-up step with onError continue",
			steps: []string{
				failingStep("failing-step-1", &onErrorContinue),
				failingStep("failing-step-2", &onErrorContinue),
			},
			shouldSucceed:     true,
			expectedStatusMsg: "All Steps have completed executing",
			state:             "TaskRunSucceeded",
			conditionFn:       TaskRunSucceed,
		},
		{
			description: "failing step with onError continueAndFail and failing follow-up step with onError continue",
			steps: []string{
				failingStep("failing-step-1", &onErrorContinueAndFail),
				failingStep("failing-step-2", &onErrorContinue),
			},
			shouldSucceed:     false,
			expectedStatusMsg: `"step-failing-step-1" exited with code 1`,
			state:             "TaskRunFailed",
			conditionFn:       TaskRunFailed,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			taskRunName := helpers.ObjectNameForTest(t)

			stepYaml := strings.Join(test.steps, "\n")
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
%s
`, taskRunName, namespace, stepYaml))

			if _, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}

			t.Logf("Waiting for TaskRun in namespace %s to finish", namespace)
			if err := WaitForTaskRunState(ctx, c, taskRunName, test.conditionFn(taskRunName), test.state, v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
			}

			taskRun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
			}

			if taskRun.Status.GetCondition(apis.ConditionSucceeded).IsTrue() != test.shouldSucceed {
				t.Fatalf("Expected TaskRun %s ConditionSucceeded to be %t, but Status is %v", taskRunName, test.shouldSucceed, taskRun.Status)
			}

			if taskRun.Status.GetCondition(apis.ConditionSucceeded).Message != test.expectedStatusMsg {
				t.Fatalf("Expected TaskRun Status Message %s, but got %s", test.expectedStatusMsg, taskRun.Status.GetCondition(apis.ConditionSucceeded).Message)
			}

			expectedResults := []v1.TaskRunResult{{
				Name:  "result1",
				Type:  "string",
				Value: *v1.NewStructuredValues("123"),
			}}

			if d := cmp.Diff(expectedResults, taskRun.Status.Results); d != "" {
				t.Errorf("Got unexpected results %s", diff.PrintWantGot(d))
			}
		})
	}
}
