//go:build e2e
// +build e2e

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

func TestFailingStepOnContinue(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	tests := []struct {
		description   string
		onError       string
		shouldSucceed bool
		state         string
		conditionFn   func(name string) ConditionAccessorFn
	}{
		{
			description:   "continue and succeed",
			onError:       "continue",
			shouldSucceed: true,
			state:         "TaskRunSucceeded",
			conditionFn:   TaskRunSucceed,
		},
		{
			description:   "continue and fail",
			onError:       "continueAndFail",
			shouldSucceed: false,
			state:         "TaskRunFailed",
			conditionFn:   TaskRunFailed,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			taskRunName := helpers.ObjectNameForTest(t)

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
        onError: %s
        image: mirror.gcr.io/busybox
        script: 'echo -n 123 | tee $(results.result1.path); exit 1; echo -n 456 | tee $(results.result2.path)'
      - name: successful-step
        image: mirror.gcr.io/busybox
        script: 'echo hello'
`, taskRunName, namespace, test.onError))

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
