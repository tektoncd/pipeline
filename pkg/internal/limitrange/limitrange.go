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

package limitrange

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func getVirtualLimitRange(ctx context.Context, namespace string, lister corev1listers.LimitRangeLister) (*corev1.LimitRange, error) {
	limitRanges, err := lister.LimitRanges(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var limitRange *corev1.LimitRange
	switch {
	case len(limitRanges) == 0:
		// No LimitRange defined
		break
	case len(limitRanges) == 1:
		// One LimitRange defined
		limitRange = limitRanges[0]
	default:
		// Several LimitRange defined
		// Create a virtual LimitRange with
		// - Maximum of min values
		// - Minimum of max values
		// - Default that "fits" into min/max taken above
		// - Default request that "fits" into min/max taken above
		// - Smallest ratio (aka the most restrictive one)
		limitRange = &corev1.LimitRange{}
		m := map[corev1.LimitType]corev1.LimitRangeItem{}
		for _, lr := range limitRanges {
			for _, item := range lr.Spec.Limits {
				_, exists := m[item.Type]
				if !exists {
					m[item.Type] = corev1.LimitRangeItem{
						Type:                 item.Type,
						Min:                  corev1.ResourceList{},
						Max:                  corev1.ResourceList{},
						Default:              corev1.ResourceList{},
						DefaultRequest:       corev1.ResourceList{},
						MaxLimitRequestRatio: corev1.ResourceList{},
					}
				}
				// Min
				m[item.Type].Min[corev1.ResourceCPU] = maxOf(m[item.Type].Min[corev1.ResourceCPU], item.Min[corev1.ResourceCPU])
				m[item.Type].Min[corev1.ResourceMemory] = maxOf(m[item.Type].Min[corev1.ResourceMemory], item.Min[corev1.ResourceMemory])
				m[item.Type].Min[corev1.ResourceEphemeralStorage] = maxOf(m[item.Type].Min[corev1.ResourceEphemeralStorage], item.Min[corev1.ResourceEphemeralStorage])
				// Max
				m[item.Type].Max[corev1.ResourceCPU] = minOf(m[item.Type].Max[corev1.ResourceCPU], item.Max[corev1.ResourceCPU])
				m[item.Type].Max[corev1.ResourceMemory] = minOf(m[item.Type].Max[corev1.ResourceMemory], item.Max[corev1.ResourceMemory])
				m[item.Type].Max[corev1.ResourceEphemeralStorage] = minOf(m[item.Type].Max[corev1.ResourceEphemeralStorage], item.Max[corev1.ResourceEphemeralStorage])
				// MaxLimitRequestRatio
				m[item.Type].MaxLimitRequestRatio[corev1.ResourceCPU] = minOf(m[item.Type].MaxLimitRequestRatio[corev1.ResourceCPU], item.MaxLimitRequestRatio[corev1.ResourceCPU])
				m[item.Type].MaxLimitRequestRatio[corev1.ResourceMemory] = minOf(m[item.Type].MaxLimitRequestRatio[corev1.ResourceMemory], item.MaxLimitRequestRatio[corev1.ResourceMemory])
				m[item.Type].MaxLimitRequestRatio[corev1.ResourceEphemeralStorage] = minOf(m[item.Type].MaxLimitRequestRatio[corev1.ResourceEphemeralStorage], item.MaxLimitRequestRatio[corev1.ResourceEphemeralStorage])
			}
		}
		// Handle Default and DefaultRequest
		for _, lr := range limitRanges {
			for _, item := range lr.Spec.Limits {
				// Default
				m[item.Type].Default[corev1.ResourceCPU] = minOfBetween(m[item.Type].Default[corev1.ResourceCPU], item.Default[corev1.ResourceCPU], m[item.Type].Min[corev1.ResourceCPU], m[item.Type].Max[corev1.ResourceCPU])
				m[item.Type].Default[corev1.ResourceMemory] = minOfBetween(m[item.Type].Default[corev1.ResourceMemory], item.Default[corev1.ResourceMemory], m[item.Type].Min[corev1.ResourceMemory], m[item.Type].Max[corev1.ResourceMemory])
				m[item.Type].Default[corev1.ResourceEphemeralStorage] = minOfBetween(m[item.Type].Default[corev1.ResourceEphemeralStorage], item.Default[corev1.ResourceEphemeralStorage], m[item.Type].Min[corev1.ResourceEphemeralStorage], m[item.Type].Max[corev1.ResourceEphemeralStorage])
				// DefaultRequest
				m[item.Type].DefaultRequest[corev1.ResourceCPU] = minOfBetween(m[item.Type].DefaultRequest[corev1.ResourceCPU], item.DefaultRequest[corev1.ResourceCPU], m[item.Type].Min[corev1.ResourceCPU], m[item.Type].Max[corev1.ResourceCPU])
				m[item.Type].DefaultRequest[corev1.ResourceMemory] = minOfBetween(m[item.Type].DefaultRequest[corev1.ResourceMemory], item.DefaultRequest[corev1.ResourceMemory], m[item.Type].Min[corev1.ResourceMemory], m[item.Type].Max[corev1.ResourceMemory])
				m[item.Type].DefaultRequest[corev1.ResourceEphemeralStorage] = minOfBetween(m[item.Type].DefaultRequest[corev1.ResourceEphemeralStorage], item.DefaultRequest[corev1.ResourceEphemeralStorage], m[item.Type].Min[corev1.ResourceCPU], m[item.Type].Max[corev1.ResourceCPU])
			}
		}
		for _, v := range m {
			limitRange.Spec.Limits = append(limitRange.Spec.Limits, v)
		}
	}
	return limitRange, nil
}

func maxOf(a, b resource.Quantity) resource.Quantity {
	if (&a).Cmp(b) > 0 {
		return a
	}
	return b
}

func minOf(a, b resource.Quantity) resource.Quantity {
	if isZero(a) || (&a).Cmp(b) > 0 {
		return b
	}
	return a
}

func minOfBetween(a, b, min, max resource.Quantity) resource.Quantity {
	if isZero(a) || (&a).Cmp(b) > 0 {
		return b
	}
	return a
}
