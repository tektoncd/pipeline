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

	// Set resource requests for all steps but the theh last container to
	// zero.
	for i := range containers[:len(containers)-1] {
		containers[i].Resources.Requests = allZeroQty()
	}
	// Set the last container's request to the max of all resources.
	containers[len(containers)-1].Resources.Requests = max
	return containers
}
