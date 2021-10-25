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
package sidecars

import (
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/pod"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	sidecarPrefix = "sidecar-"
)

// Ready returns true if all of the Pod's sidecars are Ready or
// Terminated.
func Ready(podStatus corev1.PodStatus) bool {
	if podStatus.Phase != corev1.PodRunning {
		return false
	}
	for _, s := range podStatus.ContainerStatuses {
		// If the step indicates that it's a step, skip it.
		// An injected sidecar might not have the "sidecar-" prefix, so
		// we can't just look for that prefix, we need to look at any
		// non-step container.
		if pod.IsContainerStep(s.Name) {
			continue
		}
		if s.State.Running != nil && s.Ready {
			continue
		}
		if s.State.Terminated != nil {
			continue
		}
		return false
	}
	return true
}

// Stop updates sidecar containers in the Pod to a nop image, which
// exits successfully immediately.
func Stop(ctx context.Context, nopImage string, kubeclient kubernetes.Interface, namespace, name string) (*corev1.Pod, error) {
	newPod, err := kubeclient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		// return NotFound as-is, since the K8s error checks don't handle wrapping.
		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting Pod %q when stopping sidecars: %w", name, err)
	}

	updated := false
	if newPod.Status.Phase == corev1.PodRunning {
		for _, s := range newPod.Status.ContainerStatuses {
			// Stop any running container that isn't a step.
			// An injected sidecar container might not have the
			// "sidecar-" prefix, so we can't just look for that
			// prefix.
			if !pod.IsContainerStep(s.Name) && s.State.Running != nil {
				for j, c := range newPod.Spec.Containers {
					if c.Name == s.Name && c.Image != nopImage {
						updated = true
						newPod.Spec.Containers[j].Image = nopImage
					}
				}
			}
		}
	}
	if updated {
		if newPod, err = kubeclient.CoreV1().Pods(newPod.Namespace).Update(ctx, newPod, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("error stopping sidecars of Pod %q: %w", name, err)
		}
	}
	return newPod, nil
}

// IsRunning determines if any SidecarStatus on a TaskRun
// is still running.
func IsRunning(tr *v1beta1.TaskRun) bool {
	for _, sidecar := range tr.Status.Sidecars {
		if sidecar.Terminated == nil {
			return true
		}
	}

	return false
}

// IsContainerSidecar returns true if the container name indicates that it
// represents a sidecar.
func IsContainerSidecar(name string) bool { return strings.HasPrefix(name, sidecarPrefix) }

// TrimPrefix returns the container name, stripped of its sidecar
// prefix.
func TrimPrefix(name string) string { return strings.TrimPrefix(name, sidecarPrefix) }
