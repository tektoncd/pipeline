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

package v1beta1

import (
	"context"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*PipelineRun)(nil)

// SetDefaults implements apis.Defaultable
func (pr *PipelineRun) SetDefaults(ctx context.Context) {
	pr.Spec.SetDefaults(ctx)
}

// SetDefaults implements apis.Defaultable
func (prs *PipelineRunSpec) SetDefaults(ctx context.Context) {
	cfg := config.FromContextOrDefaults(ctx)
	if prs.Timeout == nil && prs.Timeouts == nil {
		prs.Timeout = &metav1.Duration{Duration: time.Duration(cfg.Defaults.DefaultTimeoutMinutes) * time.Minute}
	}

	if prs.Timeouts != nil && prs.Timeouts.Pipeline == nil {
		prs.Timeouts.Pipeline = &metav1.Duration{Duration: time.Duration(cfg.Defaults.DefaultTimeoutMinutes) * time.Minute}
	}

	defaultSA := cfg.Defaults.DefaultServiceAccount
	if prs.ServiceAccountName == "" && defaultSA != "" {
		prs.ServiceAccountName = defaultSA
	}

	defaultPodTemplate := cfg.Defaults.DefaultPodTemplate
	prs.PodTemplate = mergePodTemplateWithDefault(prs.PodTemplate, defaultPodTemplate)

	if prs.PipelineSpec != nil {
		prs.PipelineSpec.SetDefaults(ctx)
		if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == "alpha" {
			prs.PipelineSpec.applyImplicitParams(addContextParams(ctx, prs.Params))
		}
	}
}
