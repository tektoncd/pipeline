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

package tasklevel

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ApplyTaskLevelComputeResources applies the task-level compute resource requirements to each Step.
func ApplyTaskLevelComputeResources(steps []v1.Step, computeResources *corev1.ResourceRequirements) {
	if computeResources == nil {
		return
	}
	if computeResources.Requests == nil && computeResources.Limits == nil {
		return
	}
	averageRequests := computeAverageRequests(computeResources.Requests, len(steps))
	averageLimits := computeAverageRequests(computeResources.Limits, len(steps))
	for i := range steps {
		// if no requests are specified in step or task level, the limits are used to avoid
		// unnecessary higher requests by Kubernetes default behavior.
		if steps[i].ComputeResources.Requests == nil && computeResources.Requests == nil {
			steps[i].ComputeResources.Requests = averageLimits
		} else {
			steps[i].ComputeResources.Requests = averageRequests
		}
		steps[i].ComputeResources.Limits = computeResources.Limits
	}
}

// computeAverageRequests computes the average of the requests of all the steps.
func computeAverageRequests(requests corev1.ResourceList, steps int) corev1.ResourceList {
	if len(requests) == 0 || steps == 0 {
		return nil
	}
	averageRequests := corev1.ResourceList{}
	for k, v := range requests {
		averageRequests[k] = *resource.NewMilliQuantity(v.MilliValue()/int64(steps), requests[k].Format)
	}
	return averageRequests
}
