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

package computeresources

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/internal/computeresources/compare"
	"github.com/tektoncd/pipeline/pkg/internal/computeresources/limitrange"
	"github.com/tektoncd/pipeline/pkg/pod"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var resourceNames = []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage}

// NewTransformer returns a pod.Transformer that will modify limits if needed
func NewTransformer(ctx context.Context, namespace string, lister corev1listers.LimitRangeLister) pod.Transformer {
	return func(p *corev1.Pod) (*corev1.Pod, error) {
		limitRange, err := limitrange.GetVirtualLimitRange(namespace, lister)
		if err != nil {
			return p, err
		}
		return transformPodComputeResources(ctx, p, limitRange), nil
	}
}

func transformPodComputeResources(ctx context.Context, p *corev1.Pod, limitRange *corev1.LimitRange) *corev1.Pod {
	if limitRange != nil {
		p = transformPodBasedOnLimitRange(p, *limitRange)
	}
	return setInitContainerResources(ctx, p)
}

// transformPodBasedOnLimitRange modifies the pod's containers' resource requirements to meet the constraints of the LimitRange.
// The only supported type of LimitRange is "Container".
// For any container:
// - If the container has requests, they are set to the max of (requests, limitRange minimum).
// - If the container doesn't have requests, they are set to the max of (limitRange minimum, "default"),
// where "default" is the LimitRange defaultRequest (for init containers) or the LimitRange defaultRequest / # of app containers
// (for app containers).
// - If the container has limits, they are set to the min of (limits, limitRange maximum).
// - If the container doesn't have limits, they are set to the min of (limitRange maximum, limitRange default).
func transformPodBasedOnLimitRange(p *corev1.Pod, limitRange corev1.LimitRange) *corev1.Pod {
	// The assumption here is that the min, max, default, ratio have already been
	// computed if there is multiple LimitRange to satisfy the most (if we can).
	// Count the number of step containers in the Pod.
	// This should help us find the smallest request to apply to containers
	nbStepContainers := 0
	for _, c := range p.Spec.Containers {
		if pod.IsContainerStep(c.Name) {
			nbStepContainers++
		}
	}

	// FIXME(#4230) maxLimitRequestRatio to support later
	defaultContainerLimits := getDefaultLimits(limitRange)
	defaultContainerRequests := getDefaultContainerRequest(limitRange)
	defaultStepContainerRequests := getDefaultStepContainerRequest(limitRange, nbStepContainers)

	for i := range p.Spec.InitContainers {
		// We are trying to set the smallest requests possible
		if p.Spec.InitContainers[i].Resources.Requests == nil {
			p.Spec.InitContainers[i].Resources.Requests = defaultContainerRequests
		} else {
			for _, name := range resourceNames {
				setRequestsOrLimits(name, p.Spec.InitContainers[i].Resources.Requests, defaultContainerRequests)
			}
		}
		// We are trying to set the highest limits possible
		if p.Spec.InitContainers[i].Resources.Limits == nil {
			p.Spec.InitContainers[i].Resources.Limits = defaultContainerLimits
		} else {
			for _, name := range resourceNames {
				setRequestsOrLimits(name, p.Spec.InitContainers[i].Resources.Limits, defaultContainerLimits)
			}
		}
	}

	for i, c := range p.Spec.Containers {
		var defaultRequests = defaultContainerRequests
		if pod.IsContainerStep(c.Name) {
			defaultRequests = defaultStepContainerRequests
		}

		if p.Spec.Containers[i].Resources.Requests == nil {
			p.Spec.Containers[i].Resources.Requests = defaultRequests
		} else {
			for _, name := range resourceNames {
				setRequestsOrLimits(name, p.Spec.Containers[i].Resources.Requests, defaultRequests)
			}
		}
		if p.Spec.Containers[i].Resources.Limits == nil {
			p.Spec.Containers[i].Resources.Limits = defaultContainerLimits
		} else {
			for _, name := range resourceNames {
				setRequestsOrLimits(name, p.Spec.Containers[i].Resources.Limits, defaultContainerLimits)
			}
		}
	}
	return p
}

// setInitContainerResources sets the resource requests of the pod's init containers to be the maximum resource
// requests of any app container, and the resource limits of the pod's init containers to be the maximum resource
// limits of any app container. The maximum limit is no limit.
// These changes apply only if the "enable-init-container-resources" flag is set to "true".
func setInitContainerResources(ctx context.Context, p *corev1.Pod) *corev1.Pod {
	enableInitContainerResources := config.FromContextOrDefaults(ctx).FeatureFlags.EnableInitContainerResources
	if !enableInitContainerResources {
		return p
	}
	requests := getMaxContainerRequests(p)
	limits := getMaxContainerLimits(p)
	for i := range p.Spec.InitContainers {
		if p.Spec.InitContainers[i].Resources.Requests == nil {
			p.Spec.InitContainers[i].Resources.Requests = requests
		} else {
			for _, name := range resourceNames {
				overrideRequestsOrLimits(name, p.Spec.InitContainers[i].Resources.Requests, requests)
			}
		}
		if p.Spec.InitContainers[i].Resources.Limits == nil {
			p.Spec.InitContainers[i].Resources.Limits = limits
		} else {
			for _, name := range resourceNames {
				overrideRequestsOrLimits(name, p.Spec.InitContainers[i].Resources.Limits, limits)
			}
		}
	}
	return p
}

func setRequestsOrLimits(name corev1.ResourceName, dst, src corev1.ResourceList) {
	if compare.IsZero(dst[name]) && !compare.IsZero(src[name]) {
		dst[name] = src[name]
	}
}

func overrideRequestsOrLimits(name corev1.ResourceName, dst, src corev1.ResourceList) {
	if !compare.IsZero(src[name]) {
		dst[name] = src[name]
	}
}

// Returns the default requests to use for each step container, determined by dividing the LimitRange default requests
// among the step containers, and applying the LimitRange minimum if necessary
func getDefaultStepContainerRequest(limitRange corev1.LimitRange, nbContainers int) corev1.ResourceList {
	// Support only Type Container to start with
	var r corev1.ResourceList = map[corev1.ResourceName]resource.Quantity{}
	for _, item := range limitRange.Spec.Limits {
		// Only support LimitTypeContainer
		if item.Type == corev1.LimitTypeContainer {
			for _, name := range resourceNames {
				var defaultRequest resource.Quantity
				var min resource.Quantity
				request := r[name]
				if item.DefaultRequest != nil {
					defaultRequest = item.DefaultRequest[name]
				}
				if item.Min != nil {
					min = item.Min[name]
				}

				var result resource.Quantity
				if name == corev1.ResourceMemory || name == corev1.ResourceEphemeralStorage {
					result = compare.MaxRequest(request, *resource.NewQuantity(defaultRequest.Value()/int64(nbContainers), defaultRequest.Format), min)
				} else {
					result = compare.MaxRequest(request, *resource.NewMilliQuantity(defaultRequest.MilliValue()/int64(nbContainers), defaultRequest.Format), min)
				}
				// only set non-zero request values
				if !compare.IsZero(result) {
					r[name] = result
				}
			}
		}
	}
	// return nil if the resource list is empty to avoid setting an empty defaultrequest
	if len(r) == 0 {
		return nil
	}
	return r
}

// Returns the default requests to use for each init container, determined by the LimitRange default requests and minimums
func getDefaultContainerRequest(limitRange corev1.LimitRange) corev1.ResourceList {
	// Support only Type Container to start with
	var r corev1.ResourceList
	for _, item := range limitRange.Spec.Limits {
		// Only support LimitTypeContainer
		if item.Type == corev1.LimitTypeContainer {
			if item.DefaultRequest != nil {
				r = item.DefaultRequest
			} else if item.Min != nil {
				r = item.Min
			}
		}
	}
	return r
}

func getDefaultLimits(limitRange corev1.LimitRange) corev1.ResourceList {
	// Support only Type Container to start with
	var l corev1.ResourceList
	for _, item := range limitRange.Spec.Limits {
		if item.Type == corev1.LimitTypeContainer {
			if item.Default != nil {
				l = item.Default
			} else if item.Max != nil {
				l = item.Max
			}
		}
	}
	return l
}

// getMaxContainerRequests returns a map of resource name to the maximum quantity of that resource in any of
// the pod's containers' resource requests.
func getMaxContainerRequests(p *corev1.Pod) corev1.ResourceList {
	var out corev1.ResourceList = map[corev1.ResourceName]resource.Quantity{}
	for _, c := range p.Spec.Containers {
		for name, quantity := range c.Resources.Requests {
			max, ok := out[name]
			if !ok {
				out[name] = quantity
			} else if quantity.Cmp(max) > 0 {
				out[name] = quantity
			}
		}
	}
	return out
}

// getMaxContainerLimits returns a map of resource name to the maximum quantity of that resource in any of
// the pod's containers' resource limits. The absence of limits is treated as the highest limit.
func getMaxContainerLimits(p *corev1.Pod) corev1.ResourceList {
	// Build map of resource name to all limits for that resource
	allLimits := make(map[corev1.ResourceName][]resource.Quantity)
	for _, name := range resourceNames {
		allLimits[name] = make([]resource.Quantity, 0)
	}
	for _, c := range p.Spec.Containers {
		for name, quantity := range c.Resources.Limits {
			limits, ok := allLimits[name]
			if !ok {
				allLimits[name] = []resource.Quantity{quantity}
			} else {
				limits = append(limits, quantity)
				allLimits[name] = limits
			}
		}
	}

	var out corev1.ResourceList = map[corev1.ResourceName]resource.Quantity{}
	for name, limits := range allLimits {
		if len(limits) < len(p.Spec.Containers) {
			// Not all containers have resource limits; therefore, the maximum limit is no limit.
			continue
		}
		max := resource.Quantity{}
		for _, limit := range limits {
			if limit.Cmp(max) > 0 {
				max = limit
			}
		}
		out[name] = max
	}
	return out
}
