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

	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*Pipeline)(nil)

// SetDefaults sets default values on the Pipeline's Spec
func (p *Pipeline) SetDefaults(ctx context.Context) {
	p.Spec.SetDefaults(ctx)
}

// SetDefaults sets default values for the PipelineSpec's Params and Tasks
func (ps *PipelineSpec) SetDefaults(ctx context.Context) {
	for _, pt := range ps.Tasks {
		if pt.TaskRef != nil {
			if pt.TaskRef.Kind == "" {
				pt.TaskRef.Kind = NamespacedTaskKind
			}
		}
		if pt.TaskSpec != nil {
			pt.TaskSpec.SetDefaults(ctx)
		}
	}
	for i := range ps.Params {
		ps.Params[i].SetDefaults(ctx)
	}
}
