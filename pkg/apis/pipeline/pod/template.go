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

// +listType=atomic
type Volumes []corev1.Volume

// Template holds pod specific configuration
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type Template struct {
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// List of environment variables that can be provided to the containers belonging to the pod.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar `json:"env,omitempty" patchMergeKey:"name" patchStrategy:"merge" protobuf:"bytes,7,rep,name=env"`

	// If specified, the pod's tolerations.
	// +optional
	// +listType=atomic
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// If specified, the pod's scheduling constraints.
	// See Pod.spec.affinity (API version: v1)
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// SecurityContext holds pod-level security attributes and common container settings.
	// Optional: Defaults to empty.  See type description for default values of each field.
	// See Pod.spec.securityContext (API version: v1)
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// List of volumes that can be mounted by containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// See Pod.spec.volumes (API version: v1)
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Volumes Volumes `json:"volumes,omitempty" patchMergeKey:"name" patchStrategy:"merge,retainKeys" protobuf:"bytes,1,rep,name=volumes"`

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
	PriorityClassName *string `json:"priorityClassName,omitempty" protobuf:"bytes,8,opt,name=priorityClassName"`
	// SchedulerName specifies the scheduler to be used to dispatch the Pod
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`

	// ImagePullSecrets gives the name of the secret used by the pod to pull the image if specified
	// +optional
	// +listType=atomic
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
	// file if specified. This is only valid for non-hostNetwork pods.
	// +optional
	// +listType=atomic
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`

	// HostNetwork specifies whether the pod may use the node network namespace
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`

	// HostUsers indicates whether the pod will use the host's user namespace.
	// Optional: Default to true.
	// If set to true or not present, the pod will be run in the host user namespace, useful
	// for when the pod needs a feature only available to the host user namespace, such as
	// loading a kernel module with CAP_SYS_MODULE.
	// When set to false, a new user namespace is created for the pod. Setting false
	// is useful to mitigating container breakout vulnerabilities such as allowing
	// containers to run as root without their user having root privileges on the host.
	// This field depends on the kubernetes feature gate UserNamespacesSupport being enabled.
	// +optional
	HostUsers *bool `json:"hostUsers,omitempty"`

	// TopologySpreadConstraints controls how Pods are spread across your cluster among
	// failure-domains such as regions, zones, nodes, and other user-defined topology domains.
	// +optional
	// +listType=atomic
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
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

// ToAffinityAssistantTemplate converts to a affinity assistant pod Template
func (tpl *Template) ToAffinityAssistantTemplate() *AffinityAssistantTemplate {
	if tpl == nil {
		return nil
	}

	return &AffinityAssistantTemplate{
		NodeSelector:      tpl.NodeSelector,
		Tolerations:       tpl.Tolerations,
		ImagePullSecrets:  tpl.ImagePullSecrets,
		SecurityContext:   tpl.SecurityContext,
		PriorityClassName: tpl.PriorityClassName,
	}
}

// PodTemplate holds pod specific configuration
//
//nolint:revive
type PodTemplate = Template

// MergePodTemplateWithDefault merges 2 PodTemplates together. If the same
// field is set on both templates, the value from tpl will overwrite the value
// from defaultTpl.
func MergePodTemplateWithDefault(tpl, defaultTpl *PodTemplate) *PodTemplate {
	switch {
	case defaultTpl == nil:
		// No configured default, just return the template
		return tpl
	case tpl == nil:
		// No template, just return the default template
		return defaultTpl
	default:
		// Otherwise, merge fields
		if tpl.NodeSelector == nil {
			tpl.NodeSelector = defaultTpl.NodeSelector
		}
		tpl.Env = mergeByName(defaultTpl.Env, tpl.Env)
		if tpl.Tolerations == nil {
			tpl.Tolerations = defaultTpl.Tolerations
		}
		if tpl.Affinity == nil {
			tpl.Affinity = defaultTpl.Affinity
		}
		if tpl.SecurityContext == nil {
			tpl.SecurityContext = defaultTpl.SecurityContext
		}
		tpl.Volumes = mergeByName(defaultTpl.Volumes, tpl.Volumes)
		if tpl.RuntimeClassName == nil {
			tpl.RuntimeClassName = defaultTpl.RuntimeClassName
		}
		if tpl.AutomountServiceAccountToken == nil {
			tpl.AutomountServiceAccountToken = defaultTpl.AutomountServiceAccountToken
		}
		if tpl.DNSPolicy == nil {
			tpl.DNSPolicy = defaultTpl.DNSPolicy
		}
		if tpl.DNSConfig == nil {
			tpl.DNSConfig = defaultTpl.DNSConfig
		}
		if tpl.EnableServiceLinks == nil {
			tpl.EnableServiceLinks = defaultTpl.EnableServiceLinks
		}
		if tpl.PriorityClassName == nil {
			tpl.PriorityClassName = defaultTpl.PriorityClassName
		}
		if tpl.SchedulerName == "" {
			tpl.SchedulerName = defaultTpl.SchedulerName
		}
		if tpl.ImagePullSecrets == nil {
			tpl.ImagePullSecrets = defaultTpl.ImagePullSecrets
		}
		if tpl.HostAliases == nil {
			tpl.HostAliases = defaultTpl.HostAliases
		}
		if !tpl.HostNetwork && defaultTpl.HostNetwork {
			tpl.HostNetwork = true
		}
		if tpl.HostUsers == nil {
			tpl.HostUsers = defaultTpl.HostUsers
		}
		if tpl.TopologySpreadConstraints == nil {
			tpl.TopologySpreadConstraints = defaultTpl.TopologySpreadConstraints
		}
		return tpl
	}
}

// AAPodTemplate holds pod specific configuration for the affinity-assistant
type AAPodTemplate = AffinityAssistantTemplate

// MergeAAPodTemplateWithDefault is the same as MergePodTemplateWithDefault but
// for AffinityAssistantPodTemplates.
func MergeAAPodTemplateWithDefault(tpl, defaultTpl *AAPodTemplate) *AAPodTemplate {
	switch {
	case defaultTpl == nil:
		// No configured default, just return the template
		return tpl
	case tpl == nil:
		// No template, just return the default template
		return defaultTpl
	default:
		// Otherwise, merge fields
		if tpl.NodeSelector == nil {
			tpl.NodeSelector = defaultTpl.NodeSelector
		}
		if tpl.Tolerations == nil {
			tpl.Tolerations = defaultTpl.Tolerations
		}
		if tpl.ImagePullSecrets == nil {
			tpl.ImagePullSecrets = defaultTpl.ImagePullSecrets
		}
		if tpl.SecurityContext == nil {
			tpl.SecurityContext = defaultTpl.SecurityContext
		}
		if tpl.PriorityClassName == nil {
			tpl.PriorityClassName = defaultTpl.PriorityClassName
		}
		if tpl.ServiceAccountName == "" {
			tpl.ServiceAccountName = defaultTpl.ServiceAccountName
		}

		return tpl
	}
}

// mergeByName merges two slices of items with names based on the getName
// function, giving priority to the items in the override slice.
func mergeByName[T any](base, overrides []T) []T {
	if len(overrides) == 0 {
		return base
	}

	// create a map to store the exist names in the override slice
	exists := make(map[string]struct{})
	merged := make([]T, 0, len(base)+len(overrides))

	// append the items in the override slice
	for _, item := range overrides {
		name := getName(item)
		if name != "" { // name should not be empty, if empty, ignore
			merged = append(merged, item)
			exists[name] = struct{}{}
		}
	}

	// append the items in the base slice if they have a different name
	for _, item := range base {
		name := getName(item)
		if name != "" { // name should not be empty, if empty, ignore
			if _, found := exists[name]; !found {
				merged = append(merged, item)
			}
		}
	}

	return merged
}

// getName returns the name of the given item, or an empty string if the item
// is not a supported type.
func getName(item interface{}) string {
	switch item := item.(type) {
	case corev1.EnvVar:
		return item.Name
	case corev1.Volume:
		return item.Name
	default:
		return ""
	}
}
