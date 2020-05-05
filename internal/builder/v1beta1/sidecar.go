/*
Copyright 2020 The Tekton Authors
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

package builder

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// SidecarStateName sets the name of the Sidecar for the SidecarState.
func SidecarStateName(name string) SidecarStateOp {
	return func(s *v1beta1.SidecarState) {
		s.Name = name
	}
}

// SidecarStateImageID sets ImageID of Sidecar for SidecarState.
func SidecarStateImageID(imageID string) SidecarStateOp {
	return func(s *v1beta1.SidecarState) {
		s.ImageID = imageID
	}
}

// SidecarStateContainerName sets ContainerName of Sidecar for SidecarState.
func SidecarStateContainerName(containerName string) SidecarStateOp {
	return func(s *v1beta1.SidecarState) {
		s.ContainerName = containerName
	}
}

// SetSidecarStateTerminated sets Terminated state of a Sidecar.
func SetSidecarStateTerminated(terminated corev1.ContainerStateTerminated) SidecarStateOp {
	return func(s *v1beta1.SidecarState) {
		s.ContainerState = corev1.ContainerState{
			Terminated: &terminated,
		}
	}
}

// SetSidecarStateRunning sets Running state of a Sidecar.
func SetSidecarStateRunning(running corev1.ContainerStateRunning) SidecarStateOp {
	return func(s *v1beta1.SidecarState) {
		s.ContainerState = corev1.ContainerState{
			Running: &running,
		}
	}
}

// SetSidecarStateWaiting sets Waiting state of a Sidecar.
func SetSidecarStateWaiting(waiting corev1.ContainerStateWaiting) SidecarStateOp {
	return func(s *v1beta1.SidecarState) {
		s.ContainerState = corev1.ContainerState{
			Waiting: &waiting,
		}
	}
}
