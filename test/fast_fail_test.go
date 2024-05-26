//go:build e2e
// +build e2e

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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
)

func TestFastFail(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Setting up test resources for fail fast test in namespace %s", namespace)
	pipelineRun := getFastFailPipelineRun(t, namespace)

	prName := pipelineRun.Name
	_, err := c.V1beta1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunFailed(prName), "PipelineRunFailed", v1beta1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
	cl, _ := c.V1beta1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
	if !cl.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun to fail but found condition: %s", cl.Status.GetCondition(apis.ConditionSucceeded))
	}
	expectedMessage := "Tasks Completed: 3 (Failed: 1, Cancelled 2), Skipped: 0"
	if cl.Status.GetCondition(apis.ConditionSucceeded).Message != expectedMessage {
		t.Errorf("Expected PipelineRun to fail with condition message: %s but got: %s", expectedMessage, cl.Status.GetCondition(apis.ConditionSucceeded).Message)
	}
}

func getFastFailPipelineRun(t *testing.T, namespace string) *v1beta1.PipelineRun {
	t.Helper()
	pipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: fast-fail-pipeline-run
  namespace: %s
spec:
  failFast: true
  pipelineSpec:
    tasks:
    - name: fail-task
      taskSpec:
        steps:
          - name: fail-task
            image: busybox
            command: ["/bin/sh", "-c"]
            args:
              - exit 1
    - name: success1
      taskSpec:
        steps:
          - name: success1
            image: busybox
            command: ["/bin/sh", "-c"]
            args:
              - sleep 360
    - name: success2
      taskSpec:
        steps:
          - name: success2
            image: busybox
            command: ["/bin/sh", "-c"]
            args:
              - sleep 360
`, namespace))

	return pipelineRun
}
