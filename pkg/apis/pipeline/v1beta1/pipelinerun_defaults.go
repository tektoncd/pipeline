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
	"regexp"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmap"
)

var (
	_                              apis.Defaultable = (*PipelineRun)(nil)
	filterReservedAnnotationRegexp                  = regexp.MustCompile(pipeline.TektonReservedAnnotationExpr)
)

// SetDefaults implements apis.Defaultable
func (pr *PipelineRun) SetDefaults(ctx context.Context) {
	pr.Spec.SetDefaults(ctx)

	// Silently filtering out Tekton Reserved annotations at creation
	if apis.IsInCreate(ctx) {
		pr.ObjectMeta.Annotations = kmap.Filter(pr.ObjectMeta.Annotations, func(s string) bool {
			return filterReservedAnnotationRegexp.MatchString(s)
		})
	}
}

// SetDefaults implements apis.Defaultable
func (prs *PipelineRunSpec) SetDefaults(ctx context.Context) {
	cfg := config.FromContextOrDefaults(ctx)
	if prs.PipelineRef != nil && prs.PipelineRef.Name == "" && prs.PipelineRef.Resolver == "" {
		prs.PipelineRef.Resolver = ResolverName(cfg.Defaults.DefaultResolverType)
	}

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
	prs.PodTemplate = pod.MergePodTemplateWithDefault(prs.PodTemplate, defaultPodTemplate)

	if prs.PipelineSpec != nil {
		prs.PipelineSpec.SetDefaults(ctx)
	}
}
