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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*Task)(nil)

// ConvertTo implements api.Convertible
func (t *Task) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.Task:
		sink.ObjectMeta = t.ObjectMeta
		return t.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements api.Convertible
func (ts *TaskSpec) ConvertTo(ctx context.Context, sink *v1beta1.TaskSpec) error {
	sink.Steps = ts.Steps
	sink.Volumes = ts.Volumes
	sink.StepTemplate = ts.StepTemplate
	sink.Sidecars = ts.Sidecars
	sink.Workspaces = ts.Workspaces
	sink.Results = ts.Results
	sink.Resources = ts.Resources
	sink.Params = ts.Params
	sink.Description = ts.Description
	if ts.Inputs != nil {
		if len(ts.Inputs.Params) > 0 && len(ts.Params) > 0 {
			// This shouldn't happen as it shouldn't pass validation
			return apis.ErrMultipleOneOf("inputs.params", "params")
		}
		if len(ts.Inputs.Params) > 0 {
			sink.Params = make([]v1beta1.ParamSpec, len(ts.Inputs.Params))
			for i, param := range ts.Inputs.Params {
				sink.Params[i] = *param.DeepCopy()
			}
		}
		if len(ts.Inputs.Resources) > 0 {
			if sink.Resources == nil {
				sink.Resources = &v1beta1.TaskResources{}
			}
			if len(ts.Inputs.Resources) > 0 && ts.Resources != nil && len(ts.Resources.Inputs) > 0 {
				// This shouldn't happen as it shouldn't pass validation but just in case
				return apis.ErrMultipleOneOf("inputs.resources", "resources.inputs")
			}
			sink.Resources.Inputs = make([]v1beta1.TaskResource, len(ts.Inputs.Resources))
			for i, resource := range ts.Inputs.Resources {
				sink.Resources.Inputs[i] = v1beta1.TaskResource{ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name:        resource.Name,
					Type:        resource.Type,
					Description: resource.Description,
					TargetPath:  resource.TargetPath,
					Optional:    resource.Optional,
				}}
			}
		}
	}
	if ts.Outputs != nil && len(ts.Outputs.Resources) > 0 {
		if sink.Resources == nil {
			sink.Resources = &v1beta1.TaskResources{}
		}
		if len(ts.Outputs.Resources) > 0 && ts.Resources != nil && len(ts.Resources.Outputs) > 0 {
			// This shouldn't happen as it shouldn't pass validation but just in case
			return apis.ErrMultipleOneOf("outputs.resources", "resources.outputs")
		}
		sink.Resources.Outputs = make([]v1beta1.TaskResource, len(ts.Outputs.Resources))
		for i, resource := range ts.Outputs.Resources {
			sink.Resources.Outputs[i] = v1beta1.TaskResource{ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:        resource.Name,
				Type:        resource.Type,
				Description: resource.Description,
				TargetPath:  resource.TargetPath,
				Optional:    resource.Optional,
			}}
		}
	}
	return nil
}

// ConvertFrom implements api.Convertible
func (t *Task) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.Task:
		t.ObjectMeta = source.ObjectMeta
		return t.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", t)
	}
}

// ConvertFrom implements api.Convertible
func (ts *TaskSpec) ConvertFrom(ctx context.Context, source *v1beta1.TaskSpec) error {
	ts.Steps = source.Steps
	ts.Volumes = source.Volumes
	ts.StepTemplate = source.StepTemplate
	ts.Sidecars = source.Sidecars
	ts.Workspaces = source.Workspaces
	ts.Results = source.Results
	ts.Params = source.Params
	ts.Resources = source.Resources
	ts.Description = source.Description
	return nil
}
