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

package pod

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var zeroQty = resource.MustParse("0")

func allZeroQty() corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:              zeroQty,
		corev1.ResourceMemory:           zeroQty,
		corev1.ResourceEphemeralStorage: zeroQty,
	}
}

func resolveResourceRequests(containers []corev1.Container) []corev1.Container {
	max := allZeroQty()
	for _, c := range containers {
		for k, v := range c.Resources.Requests {
			if v.Cmp(max[k]) > 0 {
				max[k] = v
			}
		}
	}

	// Set resource requests for all steps but the last container to
	// zero.
	for i := range containers[:len(containers)-1] {
		containers[i].Resources.Requests = allZeroQty()
	}
	// Set the last container's request to the max of all resources.
	containers[len(containers)-1].Resources.Requests = max
	return containers
}
