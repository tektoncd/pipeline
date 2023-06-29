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

	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	"github.com/tektoncd/pipeline/test/parse"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

// TestAffinityAssistant_PerWorkspace tests the lifecycle status of Affinity Assistant and PVCs
// from the start to the deletion of a PipelineRun
func TestAffinityAssistant_PerWorkspace(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)
	prName := "my-pipelinerun"
	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineSpec:
    workspaces:
    - name: my-workspace
    tasks:
    - name: foo
      workspaces:
      - name: my-workspace
      taskSpec:
        steps:
        - image: busybox
          script: echo hello foo
    - name: bar
      workspaces:
      - name: my-workspace
      taskSpec:
        steps:
        - image: busybox
          script: echo hello bar
  workspaces:
  - name: my-workspace
    volumeClaimTemplate:
      metadata:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 16Mi
`, prName, namespace))

	if _, err := c.V1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}
	t.Logf("Waiting for PipelineRun in namespace %s to run", namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, Running(prName), "PipelineRun Running", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to run: %s", err)
	}

	// validate SS and PVC are created
	ssName := pipelinerun.GetAffinityAssistantName("my-workspace", pr.Name)
	ss, err := c.KubeClient.AppsV1().StatefulSets(namespace).Get(ctx, ssName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected Affinity Assistant StatefulSet %s, err: %s", ss, err)
	}
	pvcList, err := c.KubeClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PVCs: %s", err)
	}
	if len(pvcList.Items) != 1 {
		t.Fatalf("Unexpected PVC created, expecting 1 but got: %v", len(pvcList.Items))
	}
	pvcName := pvcList.Items[0].Name

	// wait for PipelineRun to finish
	t.Logf("Waiting for PipelineRun in namespace %s to finish", namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSucceeded", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish: %s", err)
	}

	// validate Affinity Assistant is cleaned up
	ss, err = c.KubeClient.AppsV1().StatefulSets(namespace).Get(ctx, ssName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Failed to cleanup Affinity Assistant StatefulSet: %v, err: %v", ssName, err)
	}

	// validate PipelineRun pods sharing the same PVC are scheduled to the same node
	podFoo, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, fmt.Sprintf("%v-foo-pod", prName), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get pod: %v-foo-pod, err: %v", prName, err)
	}
	podBar, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, fmt.Sprintf("%v-bar-pod", prName), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get pod: %v-bar-pod, err: %v", prName, err)
	}
	if podFoo.Spec.NodeName != podBar.Spec.NodeName {
		t.Errorf("pods are not scheduled to same node: %v and %v", podFoo.Spec.NodeName, podBar.Spec.NodeName)
	}

	// delete PipelineRun
	if err = c.V1PipelineRunClient.Delete(ctx, prName, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("failed to delete PipelineRun: %s", err)
	}

	// validate PVC is in bounded state
	pvc, err := c.KubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get PVC %v after PipelineRun is deleted, err: %v", pvcName, err)
	}
	if pvc.Status.Phase != "Bound" {
		t.Fatalf("expect PVC %s to be in bounded state but got %v", pvcName, pvc.Status.Phase)
	}
}
