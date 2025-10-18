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
	"time"

	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// TestTaskRunPreemption tests that Taskrun can run again
// after its pod has been preempted before completion.
func TestTaskRunPreemption(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	if _, err := c.V1TaskClient.Create(ctx, parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  params:
    - name: MESSAGE
      type: string
      default: "Hello World"
  stepTemplate:
      computeResources:
        limits:
          cpu: 500
          memory: 5000Gi
        requests:
          cpu: 500
          memory: 5000Gi
  steps:
    - name: echo
      image: mirror.gcr.io/ubuntu
      script: |
        #!/usr/bin/env bash
        echo "Good Morning!" > $(workspaces.task-ws.path)
      args:
        - "$(params.MESSAGE)"
  workspaces:
    - name: task-ws
`, taskName)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create task %q: %v", taskName, err)
	}

	pipelineRunName := helpers.ObjectNameForTest(t)
	if _, err := c.V1PipelineRunClient.Create(ctx, parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineSpec:
    params:
      - name: MORNING_GREETINGS
        type: string
        default: Good Morning!
    tasks:
      - name: test-pod-preemption
        workspaces:
          - name: task-ws
            workspace: ws
        taskRef:
          name: %s
        params:
          - name: MESSAGE
            value: $(params.MORNING_GREETINGS)
    workspaces:
      - name: ws
  params:
    - name: MORNING_GREETINGS
      value: Good Morning, Bob!
  workspaces:
    - name: ws
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 32Mi
`, pipelineRunName, taskName)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRunName, err)
	}

	t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRunName, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, timeout, Running(pipelineRunName), "PipelineRunRunning", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be running: %s", pipelineRunName, err)
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRunName})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRunName, err)
	}

	tr := &taskrunList.Items[0]

	t.Logf("Waiting for TaskRun from PipelineRun %s in namespace %s to be in ExceededNodeResources", pipelineRunName, namespace)
	err = WaitForTaskRunState(ctx, c, tr.Name, exceededNodeResources(tr.Name), "TaskRunRunning", v1Version)
	if err != nil {
		t.Errorf("Error waiting for TaskRun %s to be ExceededNodeResources: %v", tr.Name, err)
	}

	tr, err = c.V1TaskRunClient.Get(ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error Getting TaskRun %s: %v", tr.Name, err)
	}

	t.Logf("Deleting Pod %s for TaskRun %s in namespace %s", tr.Status.PodName, tr.Name, namespace)
	deletePolicy := metav1.DeletePropagationForeground
	err = c.KubeClient.CoreV1().Pods(namespace).Delete(ctx, tr.Status.PodName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		t.Errorf("Error Deleting Pod: %s of TaskRun %s: %v", tr.Status.PodName, tr.Name, err)
	}

	// Wait for pod deletion event to be handled by TaskrunReconciler
	time.Sleep(5 * time.Second)
	// Get the status of the Taskrun again.
	err = WaitForTaskRunState(ctx, c, tr.Name, exceededNodeResources(tr.Name), "TaskRunRunning", v1Version)
	if err != nil {
		t.Errorf("Error waiting for TaskRun %s to be ExceededNodeResources: %v", tr.Name, err)
	}
}

// exceededNodeResources provides a poll condition function that checks if the ConditionAccessor
// resource is currently exceededNodeResources.
func exceededNodeResources(name string) ConditionAccessorFn {
	return func(ca apis.ConditionAccessor) (bool, error) {
		c := ca.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf(`%q already finished`, name)
			} else if c.Status == corev1.ConditionUnknown && (c.Reason == "ExceededNodeResources") {
				return true, nil
			}
		}
		return false, nil
	}
}
