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

// NewTransformer returns a pod.Transformer that will modify limits if needed
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
		// Count the number of "step" containers in the Pod.
		// This should help us find the smallest request to apply to containers
		stepContainers := []*corev1.Container{}
		for i, c := range p.Spec.Containers {
			if pod.IsContainerStep(c.Name) {
				stepContainers = append(stepContainers, &p.Spec.Containers[i])
			}
		}
		// No step containers, bail early we don't have anything to transform.
		if len(stepContainers) == 0 {
			return p, nil
		}

		defaultStepRequests := getDefaultStepContainerRequest(limitRange, len(stepContainers))

		// Empty LimitRange Requests, bail early we don't have anything to transform.
		if len(defaultStepRequests) == 0 {
			return p, nil
		}

		for _, c := range stepContainers {
			if c.Resources.Requests == nil {
				c.Resources.Requests = defaultStepRequests
			} else {
				for _, name := range resourceNames {
					setRequests(name, c.Resources.Requests, defaultStepRequests)
				}
			}
		}
		return p, nil
	}
}

func setRequests(name corev1.ResourceName, dst, src corev1.ResourceList) {
	if isZero(dst[name]) && !isZero(src[name]) {
		dst[name] = src[name]
	}
}

// Returns the default requests to use for each step container, determined by dividing the LimitRange default requests
// among the step containers, and applying the LimitRange minimum if necessary
// FIXME(#4230) maxLimitRequestRatio to support later
func getDefaultStepContainerRequest(limitRange *corev1.LimitRange, nbContainers int) corev1.ResourceList {
	// Support only Type Container to start with
	var r corev1.ResourceList = map[corev1.ResourceName]resource.Quantity{}
	for _, item := range limitRange.Spec.Limits {
		// Only support LimitTypeContainer
		if item.Type == corev1.LimitTypeContainer {
			for _, name := range resourceNames {
				// get previous request value if set
				request := r[name]

				var defaultRequest resource.Quantity
				if item.DefaultRequest != nil {
					defaultRequest = item.DefaultRequest[name]
				}

				var min resource.Quantity
				if item.Min != nil {
					min = item.Min[name]
				}

				var result resource.Quantity
				if name == corev1.ResourceMemory || name == corev1.ResourceEphemeralStorage {
					result = takeTheMax(request, *resource.NewQuantity(defaultRequest.Value()/int64(nbContainers), defaultRequest.Format), min)
				} else {
					result = takeTheMax(request, *resource.NewMilliQuantity(defaultRequest.MilliValue()/int64(nbContainers), defaultRequest.Format), min)
				}
				// only set non-zero request values
				if !isZero(result) {
					r[name] = result
				}
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
