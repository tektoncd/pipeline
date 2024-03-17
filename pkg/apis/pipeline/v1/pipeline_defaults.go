/*
Copyright 2022 The Tekton Authors

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

package v1

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
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

	for _, pt := range ps.Tasks {
		pt.SetDefaults(ctx)
	}

	for _, ft := range ps.Finally {
		ctx := ctx // Ensure local scoping per Task
		ft.SetDefaults(ctx)
	}
}

// SetDefaults sets default values for a PipelineTask
func (pt *PipelineTask) SetDefaults(ctx context.Context) {
	cfg := config.FromContextOrDefaults(ctx)
	if pt.TaskRef != nil {
		if pt.TaskRef.Name == "" && pt.TaskRef.Resolver == "" {
			pt.TaskRef.Resolver = ResolverName(cfg.Defaults.DefaultResolverType)
		}
		if pt.TaskRef.Kind == "" && pt.TaskRef.Resolver == "" {
			pt.TaskRef.Kind = NamespacedTaskKind
		}
	}
	if pt.TaskSpec != nil {
		pt.TaskSpec.SetDefaults(ctx)
	}
}
