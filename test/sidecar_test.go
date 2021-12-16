//go:build e2e
// +build e2e

/*
Copyright 2019 The Tekton Authors

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

	"github.com/tektoncd/pipeline/test/parse"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

const (
	sidecarTaskName      = "sidecar-test-task"
	sidecarTaskRunName   = "sidecar-test-task-run"
	sidecarContainerName = "sidecar-container"
	primaryContainerName = "primary"
)

// TestSidecarTaskSupport checks whether support for sidecars is working
// as expected by running a Task with a Sidecar defined and confirming
// that both the primary and sidecar containers terminate.
func TestSidecarTaskSupport(t *testing.T) {
	tests := []struct {
		desc           string
		stepCommand    []string
		sidecarCommand []string
	}{{
		desc:        "A sidecar that runs forever is terminated when Steps complete",
		stepCommand: []string{"echo", "\"hello world\""},
		// trapping the exit signals lets us stop the sidecar more
		// rapidly than waiting for a force stop.
		sidecarCommand: []string{"sh", "-c", "trap exit SIGTERM SIGINT; while [[ true ]] ; do echo \"hello from sidecar\"; sleep 3 ; done"},
	}, {
		desc:           "A sidecar that terminates early does not cause problems running Steps",
		stepCommand:    []string{"echo", "\"hello world\""},
		sidecarCommand: []string{"echo", "\"hello from sidecar\""},
	}}

	ctx := context.Background()
	t.Parallel()

	for i, test := range tests {
		i := i
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			clients, namespace := setup(ctx, t)
			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, clients, namespace) }, t.Logf)
			defer tearDown(ctx, t, clients, namespace)

			sidecarTaskName := fmt.Sprintf("%s-%d", sidecarTaskName, i)
			sidecarTaskRunName := fmt.Sprintf("%s-%d", sidecarTaskRunName, i)
			task := parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: %s
    image: busybox
    command: [%s]
  sidecars:
  - name: %s
    image: busybox
    command: [%s]
`, sidecarTaskName, namespace, primaryContainerName, stringSliceToYAMLArray(test.stepCommand), sidecarContainerName, stringSliceToYAMLArray(test.sidecarCommand)))

			taskRun := parse.MustParseTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
  timeout: 1m
`, sidecarTaskRunName, namespace, sidecarTaskName))

			t.Logf("Creating Task %q", sidecarTaskName)
			if _, err := clients.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Task %q: %v", sidecarTaskName, err)
			}

			t.Logf("Creating TaskRun %q", sidecarTaskRunName)
			if _, err := clients.TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun %q: %v", sidecarTaskRunName, err)
			}

			if err := WaitForTaskRunState(ctx, clients, sidecarTaskRunName, Succeed(sidecarTaskRunName), "TaskRunSucceed"); err != nil {
				t.Fatalf("Error waiting for TaskRun %q to finish: %v", sidecarTaskRunName, err)
			}

			tr, err := clients.TaskRunClient.Get(ctx, sidecarTaskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Error getting Taskrun: %v", err)
			}
			podName := tr.Status.PodName

			if err := WaitForPodState(ctx, clients, podName, namespace, func(pod *corev1.Pod) (bool, error) {
				terminatedCount := 0
				for _, c := range pod.Status.ContainerStatuses {
					if c.State.Terminated != nil {
						terminatedCount++
					}
				}
				return terminatedCount == 2, nil
			}, "PodContainersTerminated"); err != nil {
				t.Fatalf("Error waiting for Pod %q to terminate both the primary and sidecar containers: %v", podName, err)
			}

			pod, err := clients.KubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Error getting TaskRun pod: %v", err)
			}

			primaryTerminated := false
			sidecarTerminated := false

			for _, c := range pod.Status.ContainerStatuses {
				if c.Name == fmt.Sprintf("step-%s", primaryContainerName) {
					if c.State.Terminated == nil || c.State.Terminated.Reason != "Completed" {
						t.Errorf("Primary container has nil Terminated state or did not complete successfully. Actual Terminated state: %v", c.State.Terminated)
					} else {
						primaryTerminated = true
					}
				}
				if c.Name == fmt.Sprintf("sidecar-%s", sidecarContainerName) {
					if c.State.Terminated == nil {
						t.Errorf("Sidecar container has a nil Terminated status but non-nil is expected.")
					} else {
						sidecarTerminated = true
					}
				}
			}

			if !primaryTerminated || !sidecarTerminated {
				t.Errorf("Either the primary or sidecar containers did not terminate")
			}

			trCheckSidecarStatus, err := clients.TaskRunClient.Get(ctx, sidecarTaskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Error getting TaskRun: %v", err)
			}

			sidecarFromStatus := trCheckSidecarStatus.Status.Sidecars[0]

			// Check if Sidecar ContainerName is present for SidecarStatus
			if sidecarFromStatus.ContainerName != fmt.Sprintf("sidecar-%s", sidecarContainerName) {
				t.Errorf("Sidecar ContainerName should be: %s", sidecarContainerName)
			}

			// Check if Terminated status is present for SidecarStatus
			if trCheckSidecarStatus.Name == "sidecar-test-task-run-1" && sidecarFromStatus.Terminated == nil {
				t.Errorf("TaskRunStatus: Sidecar container has a nil Terminated status but non-nil is expected.")
			} else if trCheckSidecarStatus.Name == "sidecar-test-task-run-1" && sidecarFromStatus.Terminated.Reason != "Completed" {
				t.Errorf("TaskRunStatus: Sidecar container has a nil Terminated reason of %s but should be Completed", sidecarFromStatus.Terminated.Reason)
			}
		})
	}
}

func stringSliceToYAMLArray(stringSlice []string) string {
	var quoted []string
	for _, s := range stringSlice {
		quoted = append(quoted, fmt.Sprintf("'%s'", s))
	}
	return strings.Join(quoted, ", ")
}
