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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*Pipeline)(nil)

func (p *Pipeline) SetDefaults(ctx context.Context) {
	p.Spec.SetDefaults(ctx)
}

func (ps *PipelineSpec) SetDefaults(ctx context.Context) {
	for i := range ps.Params {
		ps.Params[i].SetDefaults(ctx)
	}
	if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == "alpha" {
		ctx = AddContextParamSpec(ctx, ps.Params)
		ps.Params = GetContextParamSpecs(ctx)
	}
	for i, pt := range ps.Tasks {
		ctx := ctx // Ensure local scoping per Task
		if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == "alpha" {
			ctx = AddContextParams(ctx, pt.Params)
			ps.Tasks[i].Params = GetContextParams(ctx, pt.Params...)
		}
		if pt.TaskRef != nil {
			if pt.TaskRef.Kind == "" {
				pt.TaskRef.Kind = NamespacedTaskKind
			}
		}
		if pt.TaskSpec != nil {
			pt.TaskSpec.SetDefaults(ctx)
		}
	}

	for i, ft := range ps.Finally {
		ctx := ctx // Ensure local scoping per Task
		if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == "alpha" {
			ctx = AddContextParams(ctx, ft.Params)
			ps.Finally[i].Params = GetContextParams(ctx, ft.Params...)
		}
		if ft.TaskRef != nil {
			if ft.TaskRef.Kind == "" {
				ft.TaskRef.Kind = NamespacedTaskKind
			}
		}
		if ft.TaskSpec != nil {
			ft.TaskSpec.SetDefaults(ctx)
		}
	}
}
