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

	"github.com/tektoncd/pipeline/pkg/pod"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var resourceNames = []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage}

func isZero(q resource.Quantity) bool {
	return (&q).IsZero()
}

func NewTransformer(ctx context.Context, namespace string, lister corev1listers.LimitRangeLister) pod.Transformer {
	return func(p *corev1.Pod) (*corev1.Pod, error) {
		limitRange, err := getVirtualLimitRange(ctx, namespace, lister)
		if err != nil {
			return p, err
		}
		// No LimitRange defined, nothing to transform, bail early we don't have anything to transform.
		if limitRange == nil {
			return p, nil
		}

		// The assumption here is that the min, max, default, ratio have already been
		// computed if there is multiple LimitRange to satisfy the most (if we can).
		// Count the number of containers (that we know) in the Pod.
		// This should help us find the smallest request to apply to containers
		// We are adding +1 to the number of container to take into account the init containers.
		// The reason to use +1 only is that, k8s treats request on all init container as one (getting the max of all)
		nbContainers := len(p.Spec.Containers) + 1
		// FIXME(vdemeester) maxLimitRequestRatio to support later
		defaultRequests := getDefaultRequest(limitRange, nbContainers)
		defaultLimits := getDefaultLimits(limitRange)

		for i := range p.Spec.InitContainers {
			// We are trying to set the smallest requests possible
			if p.Spec.InitContainers[i].Resources.Requests == nil {
				p.Spec.InitContainers[i].Resources.Requests = defaultRequests
			} else {
				for _, name := range resourceNames {
					setRequestsOrLimits(name, p.Spec.InitContainers[i].Resources.Requests, defaultRequests)
				}
			}
			// We are trying to set the highest limits possible
			if p.Spec.InitContainers[i].Resources.Limits == nil {
				p.Spec.InitContainers[i].Resources.Limits = defaultLimits
			} else {
				for _, name := range resourceNames {
					setRequestsOrLimits(name, p.Spec.InitContainers[i].Resources.Limits, defaultLimits)
				}
			}
		}

		for i := range p.Spec.Containers {
			if p.Spec.Containers[i].Resources.Requests == nil {
				p.Spec.Containers[i].Resources.Requests = defaultRequests
			} else {
				for _, name := range resourceNames {
					setRequestsOrLimits(name, p.Spec.Containers[i].Resources.Requests, defaultRequests)
				}
			}
			if p.Spec.Containers[i].Resources.Limits == nil {
				p.Spec.Containers[i].Resources.Limits = defaultLimits
			} else {
				for _, name := range resourceNames {
					setRequestsOrLimits(name, p.Spec.Containers[i].Resources.Limits, defaultLimits)
				}
			}
		}
		return p, nil
	}
}

func setRequestsOrLimits(name corev1.ResourceName, dst, src corev1.ResourceList) {
	if isZero(dst[name]) && !isZero(src[name]) {
		dst[name] = src[name]
	}
}

func getDefaultRequest(limitRange *corev1.LimitRange, nbContainers int) corev1.ResourceList {
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
				r[name] = takeTheMax(request, *resource.NewMilliQuantity(defaultRequest.MilliValue()/int64(nbContainers), defaultRequest.Format), min)
			}
		}
	}
	return r
}

func takeTheMax(requestQ, defaultQ, maxQ resource.Quantity) resource.Quantity {
	var q resource.Quantity = requestQ
	if defaultQ.Cmp(q) > 0 {
		q = defaultQ
	}
	if maxQ.Cmp(q) > 0 {
		q = maxQ
	}
	return q
}

func getDefaultLimits(limitRange *corev1.LimitRange) corev1.ResourceList {
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
