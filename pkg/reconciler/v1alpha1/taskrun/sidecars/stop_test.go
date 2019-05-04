package sidecars

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestStop exercises the Stop() method of the sidecars package.
// A sidecar is killed by having its container image changed to that of the nop image.
// This test therefore runs through a series of pod and sidecar configurations,
// checking whether the resulting image in the sidecar's container matches that expected.
func TestStop(t *testing.T) {
	testcases := []struct {
		description      string
		podPhase         corev1.PodPhase
		sidecarContainer corev1.Container
		sidecarState     corev1.ContainerState
		expectedImage    string
	}{{
		description: "a running sidecar container should be stopped",
		podPhase:    corev1.PodRunning,
		sidecarContainer: corev1.Container{
			Name:    "echo-hello",
			Image:   "echo-hello:latest",
			Command: []string{"echo"},
			Args:    []string{"hello"},
		},
		sidecarState: corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())},
		},
		expectedImage: *nopImage,
	}, {
		description: "a pending pod should not have its sidecars stopped",
		podPhase:    corev1.PodPending,
		sidecarContainer: corev1.Container{
			Name:    "echo-hello",
			Image:   "echo-hello:latest",
			Command: []string{"echo"},
			Args:    []string{"hello"},
		},
		sidecarState: corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())},
		},
		expectedImage: "echo-hello:latest",
	}, {
		description: "a sidecar container that is not in a running state should not be stopped",
		podPhase:    corev1.PodRunning,
		sidecarContainer: corev1.Container{
			Name:    "echo-hello",
			Image:   "echo-hello:latest",
			Command: []string{"echo"},
			Args:    []string{"hello"},
		},
		sidecarState: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{},
		},
		expectedImage: "echo-hello:latest",
	}}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test_pod",
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: nil,
					Containers: []corev1.Container{
						{
							Name:    "test-task-step",
							Image:   "test-task-step:latest",
							Command: []string{"test"},
						},
						tc.sidecarContainer,
					},
				},
				Status: corev1.PodStatus{
					Phase: tc.podPhase,
					ContainerStatuses: []corev1.ContainerStatus{
						corev1.ContainerStatus{},
						corev1.ContainerStatus{
							Name:  tc.sidecarContainer.Name,
							State: tc.sidecarState,
						},
					},
				},
			}
			updatePod := func(p *corev1.Pod) (*corev1.Pod, error) { return nil, nil }
			if err := Stop(pod, updatePod); err != nil {
				t.Errorf("error stopping sidecar: %v", err)
			}
			sidecarIdx := len(pod.Spec.Containers) - 1
			if pod.Spec.Containers[sidecarIdx].Image != tc.expectedImage {
				t.Errorf("expected sidecar image %q, actual: %q", tc.expectedImage, pod.Spec.Containers[sidecarIdx].Image)
			}
		})
	}
}
