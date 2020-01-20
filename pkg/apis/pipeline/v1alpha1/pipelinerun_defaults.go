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

package v1alpha1

import (
	"context"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/contexts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*PipelineRun)(nil)

func (pr *PipelineRun) SetDefaults(ctx context.Context) {
	pr.Spec.SetDefaults(ctx)
}

func (prs *PipelineRunSpec) SetDefaults(ctx context.Context) {
	cfg := config.FromContextOrDefaults(ctx)
	if prs.Timeout == nil {
		var timeout *metav1.Duration
		if contexts.IsUpgradeViaDefaulting(ctx) {
			// This case is for preexisting `TaskRun` before 0.5.0, so let's
			// add the old default timeout.
			// Most likely those TaskRun passing here are already done and/or already running
			timeout = &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute}
		} else {
			timeout = &metav1.Duration{Duration: time.Duration(cfg.Defaults.DefaultTimeoutMinutes) * time.Minute}
		}
		prs.Timeout = timeout
	}

	defaultSA := cfg.Defaults.DefaultServiceAccount
	if prs.ServiceAccountName == "" && defaultSA != "" {
		prs.ServiceAccountName = defaultSA
	}

	defaultPodTemplate := cfg.Defaults.DefaultPodTemplate
	if prs.PodTemplate == nil {
		prs.PodTemplate = defaultPodTemplate
	}
}
