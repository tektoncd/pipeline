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

package v1beta1

import "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"

// PodTemplate holds pod specific configuration
type PodTemplate = pod.Template

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
		if tpl.Tolerations == nil {
			tpl.Tolerations = defaultTpl.Tolerations
		}
		if tpl.Affinity == nil {
			tpl.Affinity = defaultTpl.Affinity
		}
		if tpl.SecurityContext == nil {
			tpl.SecurityContext = defaultTpl.SecurityContext
		}
		if tpl.Volumes == nil {
			tpl.Volumes = defaultTpl.Volumes
		}
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
		if tpl.HostNetwork == false && defaultTpl.HostNetwork == true {
			tpl.HostNetwork = true
		}
		return tpl
	}
}

// AAPodTemplate holds pod specific configuration for the affinity-assistant
type AAPodTemplate = pod.AffinityAssistantTemplate

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
		return tpl
	}
}
