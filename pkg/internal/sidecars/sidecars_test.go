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
package sidecars_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/internal/sidecars"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

const (
	stepPrefix    = "step-"    // FIXME(vdemeester) deduplicate this
	sidecarPrefix = "sidecar-" // FIXME(vdemeester) deduplicate this
	nopImage      = "nop-image"
)

// TestStopSidecars tests stopping sidecars by updating their images to a nop
// image.
func TestStopSidecars(t *testing.T) {
	stepContainer := corev1.Container{
		Name:  stepPrefix + "my-step",
		Image: "foo",
	}
	sidecarContainer := corev1.Container{
		Name:  sidecarPrefix + "my-sidecar",
		Image: "original-image",
	}
	stoppedSidecarContainer := corev1.Container{
		Name:  sidecarContainer.Name,
		Image: nopImage,
	}

	// This is a container that doesn't start with the "sidecar-" prefix,
	// which indicates it was injected into the Pod by a Mutating Webhook
	// Admission Controller. Injected sidecars should also be stopped if
	// they're running.
	injectedSidecar := corev1.Container{
		Name:  "injected",
		Image: "original-injected-image",
	}
	stoppedInjectedSidecar := corev1.Container{
		Name:  injectedSidecar.Name,
		Image: nopImage,
	}

	for _, c := range []struct {
		desc           string
		pod            corev1.Pod
		wantContainers []corev1.Container
	}{{
		desc: "Running sidecars (incl injected) should be stopped",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{stepContainer, sidecarContainer, injectedSidecar},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					// Step state doesn't matter.
				}, {
					Name: sidecarContainer.Name,
					// Sidecar is running.
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())}},
				}, {
					Name: injectedSidecar.Name,
					// Injected sidecar is running.
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())}},
				}},
			},
		},
		wantContainers: []corev1.Container{stepContainer, stoppedSidecarContainer, stoppedInjectedSidecar},
	}, {
		desc: "Pending Pod should not be updated",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{stepContainer, sidecarContainer},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				// Container states don't matter.
			},
		},
		wantContainers: []corev1.Container{stepContainer, sidecarContainer},
	}, {
		desc: "Non-Running sidecars should not be stopped",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{stepContainer, sidecarContainer, injectedSidecar},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					// Step state doesn't matter.
				}, {
					Name: sidecarContainer.Name,
					// Sidecar is waiting.
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}},
				}, {
					Name: injectedSidecar.Name,
					// Injected sidecar is waiting.
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}},
				}},
			},
		},
		wantContainers: []corev1.Container{stepContainer, sidecarContainer, injectedSidecar},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			kubeclient := fakek8s.NewSimpleClientset(&c.pod)
			if got, err := sidecars.Stop(ctx, nopImage, kubeclient, c.pod.Namespace, c.pod.Name); err != nil {
				t.Errorf("error stopping sidecar: %v", err)
			} else if d := cmp.Diff(c.wantContainers, got.Spec.Containers); d != "" {
				t.Errorf("Containers Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
