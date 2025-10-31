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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
)

// TestAffinityAssistant_PerWorkspace tests the taskrun pod scheduling and the PVC lifecycle status
func TestAffinityAssistant_PerWorkspace(t *testing.T) {
	ctx := t.Context()
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
        - image: mirror.gcr.io/busybox
          script: echo hello foo
    - name: bar
      workspaces:
      - name: my-workspace
      taskSpec:
        steps:
        - image: mirror.gcr.io/busybox
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

	// wait for PipelineRun to finish
	t.Logf("Waiting for PipelineRun in namespace %s to finish", namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSucceeded", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish: %s", err)
	}

	// get PVC
	pvcList, err := c.KubeClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PVCs: %s", err)
	}
	if len(pvcList.Items) != 1 {
		t.Fatalf("Unexpected PVC created, expecting 1 but got: %v", len(pvcList.Items))
	}
	pvcName := pvcList.Items[0].Name

	// validate PipelineRun pods sharing the same PVC are scheduled to the same node
	podNames := []string{fmt.Sprintf("%v-foo-pod", prName), fmt.Sprintf("%v-bar-pod", prName)}
	validatePodAffinity(t, ctx, podNames, namespace, c.KubeClient)

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

// TestAffinityAssistant_PerPipelineRun tests that mounting multiple PVC based workspaces to a pipeline task is allowed and
// all the pods are scheduled to the same node in AffinityAssistantPerPipelineRuns mode
func TestAffinityAssistant_PerPipelineRun(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer resetFeatureFlagAndCleanup(ctx, t, c, namespace)

	// update feature flag
	configMapData := map[string]string{
		"coschedule": config.CoschedulePipelineRuns,
	}
	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}

	prName := "my-pipelinerun"
	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineSpec:
    workspaces:
    - name: my-workspace
    - name: my-workspace2
    tasks:
    - name: foo
      workspaces:
      - name: my-workspace
      taskSpec:
        steps:
        - image: mirror.gcr.io/busybox
          script: echo hello foo
    - name: bar
      workspaces:
      - name: my-workspace2
      taskSpec:
        steps:
        - image: mirror.gcr.io/busybox
          script: echo hello bar
    - name: double-ws
      workspaces:
      - name: my-workspace
      - name: my-workspace2
      taskSpec:
        steps:
        - image: mirror.gcr.io/busybox
          script: echo double-ws
    - name: no-ws
      taskSpec:
        steps:
        - image: mirror.gcr.io/busybox
        script: echo no-ws
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
  - name: my-workspace2
    volumeClaimTemplate:
      metadata:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 16Mi
`, prName, namespace))

	// create PipelineRun
	if _, err := c.V1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	// wait for PipelineRun to finish
	t.Logf("Waiting for PipelineRun in namespace %s to finish", namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSucceeded", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish: %s", err)
	}

	// wait and check PVCs are deleted
	t.Logf("Waiting for PVC in namespace %s to delete", namespace)
	if err := WaitForPVCIsDeleted(ctx, c, timeout, prName, namespace, "PVCDeleted"); err != nil {
		t.Errorf("Error waiting for PVC to be deleted: %s", err)
	}

	// validate PipelineRun pods sharing the same PVC are scheduled to the same node
	podNames := []string{fmt.Sprintf("%v-foo-pod", prName), fmt.Sprintf("%v-bar-pod", prName), fmt.Sprintf("%v-double-ws-pod", prName), fmt.Sprintf("%v-no-ws-pod", prName)}
	validatePodAffinity(t, ctx, podNames, namespace, c.KubeClient)
}

// validatePodAffinity checks if all the given pods are scheduled to the same node
func validatePodAffinity(t *testing.T, ctx context.Context, podNames []string, namespace string, client kubernetes.Interface) {
	t.Helper()
	nodeName := ""
	for _, podName := range podNames {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get pod: %v, err: %v", podName, err)
		}

		if nodeName == "" {
			nodeName = pod.Spec.NodeName
		} else if pod.Spec.NodeName != nodeName {
			t.Errorf("pods are not scheduled to the same node as expected %v vs %v", nodeName, pod.Spec.NodeName)
		}
	}
}

func resetFeatureFlagAndCleanup(ctx context.Context, t *testing.T, c *clients, namespace string) {
	t.Helper()
	configMapData := map[string]string{
		"coschedule": config.DefaultCoschedule,
	}
	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
	tearDown(ctx, t, c, namespace)
}
