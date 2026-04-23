//go:build e2e

/*
Copyright 2026 The Tekton Authors

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

	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// TestComputeResourcesParamSubstitution verifies that $(params.*) references
// in computeResources fields are correctly substituted at runtime.
func TestComputeResourcesParamSubstitution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	taskRunName := helpers.ObjectNameForTest(t)

	t.Logf("Creating Task %q with parameterized computeResources", taskName)
	if _, err := c.V1TaskClient.Create(ctx, parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: MEMORY_REQUEST
      type: string
      default: "64Mi"
    - name: MEMORY_LIMIT
      type: string
      default: "128Mi"
  steps:
    - name: check-resources
      image: busybox
      computeResources:
        requests:
          memory: $(params.MEMORY_REQUEST)
        limits:
          memory: $(params.MEMORY_LIMIT)
      script: |
        #!/bin/sh
        echo "SUCCESS: Task ran with parameterized compute resources"
`, taskName, namespace)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %v", err)
	}

	t.Logf("Creating TaskRun %q with param overrides", taskRunName)
	if _, err := c.V1TaskRunClient.Create(ctx, parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
  params:
    - name: MEMORY_REQUEST
      value: "32Mi"
    - name: MEMORY_LIMIT
      value: "64Mi"
`, taskRunName, namespace, taskName)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %v", err)
	}

	t.Logf("Waiting for TaskRun %q to succeed", taskRunName)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceeded", v1Version); err != nil {
		t.Fatalf("TaskRun %q failed: %v", taskRunName, err)
	}

	// Verify the TaskRun succeeded and the pod had the right resources
	tr, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get TaskRun: %v", err)
	}

	if !tr.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
		t.Fatalf("TaskRun did not succeed: %v", tr.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Verify pod resources
	pod, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, tr.Status.PodName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get pod: %v", err)
	}

	for _, container := range pod.Spec.Containers {
		if container.Name == "step-check-resources" {
			memReq := container.Resources.Requests.Memory()
			memLim := container.Resources.Limits.Memory()
			if memReq.String() != "32Mi" {
				t.Errorf("Expected memory request 32Mi, got %s", memReq.String())
			}
			if memLim.String() != "64Mi" {
				t.Errorf("Expected memory limit 64Mi, got %s", memLim.String())
			}
			t.Logf("Pod container resources: requests=%s limits=%s", memReq, memLim)
			return
		}
	}
	t.Fatal("step-check-resources container not found in pod")
}
