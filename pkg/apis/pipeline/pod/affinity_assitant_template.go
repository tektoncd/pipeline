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
	"reflect"

	corev1 "k8s.io/api/core/v1"
)

// AffinityAssistantTemplate holds pod specific configuration and is a subset
// of the generic pod Template
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type AffinityAssistantTemplate struct {
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// If specified, the pod's tolerations.
	// +optional
	// +listType=atomic
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// ImagePullSecrets gives the name of the secret used by the pod to pull the image if specified
	// +optional
	// +listType=atomic
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// SecurityContext sets the security context for the pod
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// If specified, indicates the pod's priority. "system-node-critical" and
	// "system-cluster-critical" are two special keywords which indicate the
	// highest priorities with the former being the highest priority. Any other
	// name must be defined by creating a PriorityClass object with that name.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`
}

// Equals checks if this Template is identical to the given Template.
func (tpl *AffinityAssistantTemplate) Equals(other *AffinityAssistantTemplate) bool {
	if tpl == nil && other == nil {
		return true
	}

	if tpl == nil || other == nil {
		return false
	}

	return reflect.DeepEqual(tpl, other)
}
