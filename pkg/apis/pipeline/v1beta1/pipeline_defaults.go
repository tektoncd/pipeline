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

	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*Pipeline)(nil)

// SetDefaults sets default values on the Pipeline's Spec
func (p *Pipeline) SetDefaults(ctx context.Context) {
	p.Spec.SetDefaults(ctx)
}

// SetDefaults sets default values for the PipelineSpec's Params, Tasks, and Finally
func (ps *PipelineSpec) SetDefaults(ctx context.Context) {
	for i := range ps.Params {
		ps.Params[i].SetDefaults(ctx)
	}
	if GetImplicitParamsEnabled(ctx) {
		ctx = addContextParamSpec(ctx, ps.Params)
		ps.Params = getContextParamSpecs(ctx)
	}
	for i, pt := range ps.Tasks {
		ctx := ctx // Ensure local scoping per Task

		if pt.TaskRef != nil {
			if pt.TaskRef.Kind == "" {
				pt.TaskRef.Kind = NamespacedTaskKind
			}
		}
		if pt.TaskSpec != nil {
			// Only propagate param context to the spec - ref params should
			// still be explicitly set.
			if GetImplicitParamsEnabled(ctx) {
				ctx = addContextParams(ctx, pt.Params)
				ps.Tasks[i].Params = getContextParams(ctx, pt.Params...)
			}
			pt.TaskSpec.SetDefaults(ctx)
		}
	}

	for i, ft := range ps.Finally {
		ctx := ctx // Ensure local scoping per Task
		if ft.TaskRef != nil {
			if ft.TaskRef.Kind == "" {
				ft.TaskRef.Kind = NamespacedTaskKind
			}
		}
		if ft.TaskSpec != nil {
			if GetImplicitParamsEnabled(ctx) {
				ctx = addContextParams(ctx, ft.Params)
				ps.Finally[i].Params = getContextParams(ctx, ft.Params...)
			}
			ft.TaskSpec.SetDefaults(ctx)
		}
	}
}
