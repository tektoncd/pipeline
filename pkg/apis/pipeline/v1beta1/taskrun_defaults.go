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

import (
	"context"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*TaskRun)(nil)

const ManagedByLabelKey = "app.kubernetes.io/managed-by"

func (tr *TaskRun) SetDefaults(ctx context.Context) {
	ctx = apis.WithinParent(ctx, tr.ObjectMeta)
	tr.Spec.SetDefaults(ctx)

	// If the TaskRun doesn't have a managed-by label, apply the default
	// specified in the config.
	cfg := config.FromContextOrDefaults(ctx)
	if tr.ObjectMeta.Labels == nil {
		tr.ObjectMeta.Labels = map[string]string{}
	}
	if _, found := tr.ObjectMeta.Labels[ManagedByLabelKey]; !found {
		tr.ObjectMeta.Labels[ManagedByLabelKey] = cfg.Defaults.DefaultManagedByLabelValue
	}
}

func (trs *TaskRunSpec) SetDefaults(ctx context.Context) {
	cfg := config.FromContextOrDefaults(ctx)
	if trs.TaskRef != nil && trs.TaskRef.Kind == "" {
		trs.TaskRef.Kind = NamespacedTaskKind
	}

	if trs.Timeout == nil {
		trs.Timeout = &metav1.Duration{Duration: time.Duration(cfg.Defaults.DefaultTimeoutMinutes) * time.Minute}
	}

	defaultSA := cfg.Defaults.DefaultServiceAccount
	if trs.ServiceAccountName == "" && defaultSA != "" {
		trs.ServiceAccountName = defaultSA
	}

	defaultPodTemplate := cfg.Defaults.DefaultPodTemplate
	trs.PodTemplate = mergePodTemplateWithDefault(trs.PodTemplate, defaultPodTemplate)

	// If this taskrun has an embedded task, apply the usual task defaults
	if trs.TaskSpec != nil {
		if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == "alpha" {
			ctx = AddContextParams(ctx, trs.Params)
		}
		trs.TaskSpec.SetDefaults(ctx)
	}
}

func mergePodTemplateWithDefault(tpl, defaultTpl *PodTemplate) *PodTemplate {
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
