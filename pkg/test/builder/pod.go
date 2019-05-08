// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package builder

import (
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

type PodStatusOp func(*corev1.PodStatus)

// PodStatus creates a Status with default values.
// Any number of PodStatus modifiers can be passed to transform it.
func PodStatus(ops ...PodStatusOp) tb.PodOp {
	return func(pod *corev1.Pod) {
		podStatus := &pod.Status
		for _, op := range ops {
			op(podStatus)
		}
		pod.Status = *podStatus
	}
}

// PodInitContainerStatus creates new ContainerStatus
func PodInitContainerStatus(name, image string) PodStatusOp {
	return func(status *corev1.PodStatus) {
		cs := corev1.ContainerStatus{
			Name:  name,
			Image: image,
		}
		status.InitContainerStatuses = append(status.InitContainerStatuses, cs)
	}
}
