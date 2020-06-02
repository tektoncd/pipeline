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
	"fmt"
	"testing"
	"time"

	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
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
		desc:           "A sidecar that runs forever is terminated when Steps complete",
		stepCommand:    []string{"echo", "\"hello world\""},
		sidecarCommand: []string{"sh", "-c", "while [[ true ]] ; do echo \"hello from sidecar\" ; done"},
	}, {
		desc:           "A sidecar that terminates early does not cause problems running Steps",
		stepCommand:    []string{"echo", "\"hello world\""},
		sidecarCommand: []string{"echo", "\"hello from sidecar\""},
	}}

	clients, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, clients, namespace) }, t.Logf)
	defer tearDown(t, clients, namespace)

	for i, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			sidecarTaskName := fmt.Sprintf("%s-%d", sidecarTaskName, i)
			sidecarTaskRunName := fmt.Sprintf("%s-%d", sidecarTaskRunName, i)
			task := &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{Name: sidecarTaskName, Namespace: namespace},
				Spec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{
						Container: corev1.Container{Name: primaryContainerName, Image: "busybox", Command: test.stepCommand},
					}},
					Sidecars: []v1beta1.Sidecar{{
						Container: corev1.Container{Name: sidecarContainerName, Image: "busybox", Command: test.sidecarCommand},
					}},
				},
			}

			taskRun := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: sidecarTaskRunName, Namespace: namespace},
				Spec: v1beta1.TaskRunSpec{
					TaskRef: &v1beta1.TaskRef{Name: sidecarTaskName},
					Timeout: &metav1.Duration{Duration: 1 * time.Minute},
				},
			}

			t.Logf("Creating Task %q", sidecarTaskName)
			if _, err := clients.TaskClient.Create(task); err != nil {
				t.Errorf("Failed to create Task %q: %v", sidecarTaskName, err)
			}

			t.Logf("Creating TaskRun %q", sidecarTaskRunName)
			if _, err := clients.TaskRunClient.Create(taskRun); err != nil {
				t.Errorf("Failed to create TaskRun %q: %v", sidecarTaskRunName, err)
			}

			if err := WaitForTaskRunState(clients, sidecarTaskRunName, Succeed(sidecarTaskRunName), "TaskRunSucceed"); err != nil {
				t.Errorf("Error waiting for TaskRun %q to finish: %v", sidecarTaskRunName, err)
			}

			tr, err := clients.TaskRunClient.Get(sidecarTaskRunName, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Error getting Taskrun: %v", err)
			}
			podName := tr.Status.PodName

			if err := WaitForPodState(clients, podName, namespace, func(pod *corev1.Pod) (bool, error) {
				terminatedCount := 0
				for _, c := range pod.Status.ContainerStatuses {
					if c.State.Terminated != nil {
						terminatedCount++
					}
				}
				return terminatedCount == 2, nil
			}, "PodContainersTerminated"); err != nil {
				t.Errorf("Error waiting for Pod %q to terminate both the primary and sidecar containers: %v", podName, err)
			}

			pod, err := clients.KubeClient.Kube.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Error getting TaskRun pod: %v", err)
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

			trCheckSidecarStatus, err := clients.TaskRunClient.Get(sidecarTaskRunName, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Error getting TaskRun: %v", err)
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
