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

// Template holds pod specific configuration
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type Template struct {
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// If specified, the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// SecurityContext holds pod-level security attributes and common container settings.
	// Optional: Defaults to empty.  See type description for default values of each field.
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// List of volumes that can be mounted by containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	Volumes []corev1.Volume `json:"volumes,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name" protobuf:"bytes,1,rep,name=volumes"`

	// RuntimeClassName refers to a RuntimeClass object in the node.k8s.io
	// group, which should be used to run this pod. If no RuntimeClass resource
	// matches the named class, the pod will not be run. If unset or empty, the
	// "legacy" RuntimeClass will be used, which is an implicit class with an
	// empty definition that uses the default runtime handler.
	// More info: https://git.k8s.io/enhancements/keps/sig-node/runtime-class.md
	// This is a beta feature as of Kubernetes v1.14.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty" protobuf:"bytes,2,opt,name=runtimeClassName"`

	// AutomountServiceAccountToken indicates whether pods running as this
	// service account should have an API token automatically mounted.
	// +optional
	AutomountServiceAccountToken *bool `json:"automountServiceAccountToken,omitempty" protobuf:"varint,3,opt,name=automountServiceAccountToken"`

	// Set DNS policy for the pod. Defaults to "ClusterFirst". Valid values are
	// 'ClusterFirst', 'Default' or 'None'. DNS parameters given in DNSConfig
	// will be merged with the policy selected with DNSPolicy.
	// +optional
	DNSPolicy *corev1.DNSPolicy `json:"dnsPolicy,omitempty" protobuf:"bytes,4,opt,name=dnsPolicy,casttype=k8s.io/api/core/v1.DNSPolicy"`

	// Specifies the DNS parameters of a pod.
	// Parameters specified here will be merged to the generated DNS
	// configuration based on DNSPolicy.
	// +optional
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty" protobuf:"bytes,5,opt,name=dnsConfig"`

	// EnableServiceLinks indicates whether information about services should be injected into pod's
	// environment variables, matching the syntax of Docker links.
	// Optional: Defaults to true.
	// +optional
	EnableServiceLinks *bool `json:"enableServiceLinks,omitempty" protobuf:"varint,6,opt,name=enableServiceLinks"`

	// If specified, indicates the pod's priority. "system-node-critical" and
	// "system-cluster-critical" are two special keywords which indicate the
	// highest priorities with the former being the highest priority. Any other
	// name must be defined by creating a PriorityClass object with that name.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty" protobuf:"bytes,7,opt,name=priorityClassName"`
	// SchedulerName specifies the scheduler to be used to dispatch the Pod
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`

	// ImagePullSecrets gives the name of the secret used by the pod to pull the image if specified
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
	// file if specified. This is only valid for non-hostNetwork pods.
	// +optional
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`

	// HostNetwork specifies whether the pod may use the node network namespace
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`
}

// Equals checks if this Template is identical to the given Template.
func (tpl *Template) Equals(other *Template) bool {
	if tpl == nil && other == nil {
		return true
	}

	if tpl == nil || other == nil {
		return false
	}

	return reflect.DeepEqual(tpl, other)
}
