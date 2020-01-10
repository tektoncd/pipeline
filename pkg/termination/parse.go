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
package termination

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TerminationDetails stores information collected from a Pod status'
// termination messages.
type TerminationDetails struct {
	// StartTimes is all the start times of containers in the Pod, in
	// order, as reported by the container statuses' termination messages.
	StartTimes []metav1.Time
	// ResourceResults is all the resource results reported by containers
	// in the Pod, in order, as reported by the container statuses'
	// termination messages.
	ResourceResults []v1alpha1.PipelineResourceResult
}

// ParseMessages parses all of a PodStatus' termination messages to produce a
// list of PipelineResourceResults, and the start times for each step.
//
// For ResourceResults, if more than one item has the same key for all
// container statuses, only the last is preserved. Items are sorted by their
// key.
func ParseMessages(s corev1.PodStatus) (*TerminationDetails, error) {
	var d TerminationDetails
	for idx, cs := range s.ContainerStatuses {
		if cs.State.Terminated != nil {
			msg := cs.State.Terminated.Message
			if msg == "" {
				continue
			}
			var this []v1alpha1.PipelineResourceResult
			if err := json.Unmarshal([]byte(msg), &this); err != nil {
				return nil, fmt.Errorf("parsing message json for status %d: %v", idx, err)
			}

			for ridx, r := range this {
				if r.Key == "StartedAt" {
					t, err := time.Parse(time.RFC3339, r.Value)
					if err != nil {
						return nil, fmt.Errorf("parsing start time for status %d: %v", idx, err)
					}
					d.StartTimes = append(d.StartTimes, metav1.NewTime(t))
					this = append(this[:ridx], this[ridx+1:]...) // Trim out this result.
				}
			}

			d.ResourceResults = append(d.ResourceResults, this...)
		}
	}

	// Remove duplicates (last one wins) and sort by key.
	dedupe := map[string]v1alpha1.PipelineResourceResult{}
	for _, r := range d.ResourceResults {
		dedupe[r.Key] = r
	}
	var sorted []v1alpha1.PipelineResourceResult
	for _, v := range dedupe {
		sorted = append(sorted, v)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })
	d.ResourceResults = sorted
	return &d, nil
}
