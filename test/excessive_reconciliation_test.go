//go:build e2e

/*
Copyright 2025 The Tekton Authors

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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	controllerContainer = "tekton-pipelines-controller"
	pipelineRunTimeout  = 2 * time.Minute
	logTailLines        = 10000
)

// getTektonNamespace returns the Tekton system namespace from the SYSTEM_NAMESPACE
// environment variable, defaulting to "tekton-pipelines" if not set.
func getTektonNamespace() string {
	ns := os.Getenv("SYSTEM_NAMESPACE")
	if ns == "" {
		return "tekton-pipelines"
	}
	return ns
}

// TestPipelineRunExcessiveReconciliation verifies that PipelineRuns and their TaskRuns
// don't get reconciled excessively while in Running state. This is a regression test for issue #8495.
//
// Without the fix, both the PipelineRun and TaskRun would be reconciled hundreds or thousands of times.
// With the fix, reconciliations should stay well below 20 (typically around 10 or less).
//
// This test validates the fix by counting actual reconciliations from controller logs.
//
// @test:execution=parallel
func TestPipelineRunExcessiveReconciliation(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	pipelineRunName := helpers.ObjectNameForTest(t)

	// Create a ConfigMap that will be mounted by the Task
	// This ConfigMap will be created after the PipelineRun starts to trigger volume mount events
	configMapName := helpers.ObjectNameForTest(t)

	// Create a PipelineRun with embedded Task spec that has multiple features to trigger frequent pod events:
	// 1. Two sequential steps - triggers step state transitions
	// 2. Results (termination messages) - triggers status updates
	// 3. Sidecar container with readiness probe (every 1s) - triggers pod condition changes
	// 4. Required ConfigMap volume mount (not optional) - pod waits for ConfigMap, triggers mount events
	t.Logf("Creating PipelineRun with embedded Task spec in namespace %s", namespace)
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  # Set timeout to 0 to disable timeout and trigger the excessive reconciliation bug
  # Without the fix, this causes hundreds or thousands of reconciliations
  timeouts:
    pipeline: "0s"
  pipelineSpec:
    tasks:
    - name: event-generating-task
      taskSpec:
        results:
        - name: output
          description: Task output result
        - name: timestamp
          description: Completion timestamp
        sidecars:
        - name: monitoring-sidecar
          image: mirror.gcr.io/busybox
          command: ['/bin/sh']
          args: ['-c', 'while true; do echo "Sidecar running"; sleep 2; done']
          readinessProbe:
            exec:
              command: ['/bin/sh', '-c', 'true']
            initialDelaySeconds: 1
            periodSeconds: 1
        steps:
        - name: main-step
          image: mirror.gcr.io/busybox
          volumeMounts:
          - name: config-volume
            mountPath: /config
          command: ['/bin/sh']
          args:
          - '-c'
          - |
            echo "Step 1: Starting main task with config from /config"
            # Read config if available
            if [ -f /config/data ]; then
              echo "Config found: $(cat /config/data)"
            fi
            # Run for 10 seconds, writing progress
            for i in $(seq 1 10); do
              echo "Progress: $i/10"
              sleep 1
            done
            echo "Main task completed"
        - name: finalize-step
          image: mirror.gcr.io/busybox
          command: ['/bin/sh']
          args:
          - '-c'
          - |
            echo "Step 2: Finalization - writing results"
            # Write results (triggers termination message updates)
            echo -n "task-completed-successfully" > $(results.output.path)
            echo -n "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)" > $(results.timestamp.path)
            echo "Task completed"
        volumes:
        - name: config-volume
          configMap:
            name: %s
`, pipelineRunName, namespace, configMapName))

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	// Create ConfigMap after a delay to trigger volume mount events and pod status changes
	// This simulates dynamic resource availability that can trigger reconciliation
	go func() {
		time.Sleep(2 * time.Second)
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"data": "configuration-data-for-task",
			},
		}
		// Use test context for creating the ConfigMap
		createCtx := t.Context()
		if _, err := c.KubeClient.CoreV1().ConfigMaps(namespace).Create(createCtx, configMap, metav1.CreateOptions{}); err != nil {
			t.Logf("Warning: Failed to create ConfigMap (non-fatal): %v", err)
		} else {
			t.Logf("Created ConfigMap %s to trigger volume mount events", configMapName)
		}
	}()

	// Wait for PipelineRun to complete
	t.Logf("Waiting for PipelineRun %s to complete...", pipelineRunName)
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, pipelineRunTimeout, PipelineRunSucceed(pipelineRunName), "PipelineRunSuccess", "v1"); err != nil {
		t.Fatalf("Failed waiting for PipelineRun to succeed: %v", err)
	}

	t.Logf("PipelineRun completed - counting reconciliations from controller logs...")

	// Count reconciliations from controller logs - this is the primary validation metric
	prReconcileCount, err := countPipelineRunReconciliationsFromLogs(ctx, c, pipelineRunName)
	if err != nil {
		t.Fatalf("Failed to count PipelineRun reconciliations from logs: %v", err)
	}
	t.Logf("PipelineRun reconciliations: %d", prReconcileCount)

	// Get TaskRun names to count their reconciliations
	taskRuns, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineRun=" + pipelineRunName,
	})
	if err != nil {
		t.Fatalf("Failed to list TaskRuns: %v", err)
	}

	for _, tr := range taskRuns.Items {
		trReconcileCount, err := countTaskRunReconciliationsFromLogs(ctx, c, tr.Name, namespace)
		if err != nil {
			t.Logf("Warning: Failed to count TaskRun %s reconciliations from logs: %v", tr.Name, err)
		} else {
			t.Logf("TaskRun %s reconciliations while Running: %d", tr.Name, trReconcileCount)
		}
	}

	// With the fix for issue #8495, we expect reconciliations to stay well below 20.
	// Without the fix, there would be hundreds or thousands of reconciliations.
	//
	// We use a threshold of 20 to account for legitimate reconciliations:
	// - Pod watch events (pod created, containers starting, running, completed)
	// - Step state transitions
	// - Sidecar readiness probes
	// - ConfigMap mount events
	// - Periodic resyncs
	const maxExpectedReconciliations = 20

	if prReconcileCount > maxExpectedReconciliations {
		t.Errorf("PipelineRun had excessive reconciliations: %d (expected ≤ %d). "+
			"This suggests the fix for issue #8495 is not working correctly. "+
			"Without the fix, there would be hundreds or thousands of reconciliations.",
			prReconcileCount, maxExpectedReconciliations)
	} else {
		t.Logf("✓ PipelineRun reconciliation count is optimal: %d reconciliations (threshold: %d)",
			prReconcileCount, maxExpectedReconciliations)
	}
}

// getControllerLogs retrieves the controller logs stream for parsing.
// It returns an io.ReadCloser that must be closed by the caller.
func getControllerLogs(ctx context.Context, c *clients) (io.ReadCloser, error) {
	tektonNamespace := getTektonNamespace()
	// Find the controller pod
	pods, err := c.KubeClient.CoreV1().Pods(tektonNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=controller",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list controller pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, errors.New("no controller pod found")
	}

	controllerPod := pods.Items[0].Name

	// Get the controller logs
	logOptions := &corev1.PodLogOptions{
		Container: controllerContainer,
		TailLines: pointerToInt64(logTailLines),
	}
	req := c.KubeClient.CoreV1().Pods(tektonNamespace).GetLogs(controllerPod, logOptions)
	logs, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get controller logs: %w", err)
	}
	return logs, nil
}

// countPipelineRunReconciliationsFromLogs counts how many times the controller
// reconciled the PipelineRun by parsing controller logs.
func countPipelineRunReconciliationsFromLogs(ctx context.Context, c *clients, prName string) (int, error) {
	logs, err := getControllerLogs(ctx, c)
	if err != nil {
		return 0, err
	}
	defer logs.Close()

	// Count "status is being set to" messages for this PipelineRun
	count := 0
	scanner := bufio.NewScanner(logs)
	searchString := fmt.Sprintf("PipelineRun %s status is being set to", prName)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, searchString) {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading controller logs: %w", err)
	}

	return count, nil
}

// countTaskRunReconciliationsFromLogs counts how many times the controller
// reconciled the TaskRun while it was in "Running" state by parsing controller logs.
// This is a more reliable metric than watching for status updates, as it directly
// measures reconciliation attempts rather than their side effects.
func countTaskRunReconciliationsFromLogs(ctx context.Context, c *clients, taskRunName, namespace string) (int, error) {
	logs, err := getControllerLogs(ctx, c)
	if err != nil {
		return 0, err
	}
	defer logs.Close()

	// Parse logs and count "Successfully reconciled" messages for this TaskRun
	// while it was in Running state (Reason:\"Running\")
	count := 0
	scanner := bufio.NewScanner(logs)
	searchString := fmt.Sprintf("Successfully reconciled taskrun %s/%s", taskRunName, namespace)

	for scanner.Scan() {
		line := scanner.Text()

		// Look for reconciliation log lines for this specific TaskRun
		if !strings.Contains(line, searchString) {
			continue
		}
		// Count only reconciliations while in Running state
		// In JSON logs, the format is Reason:\"Running\"
		if strings.Contains(line, `Reason:\"Running\"`) {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading controller logs: %w", err)
	}

	return count, nil
}

func pointerToInt64(i int64) *int64 {
	return &i
}
