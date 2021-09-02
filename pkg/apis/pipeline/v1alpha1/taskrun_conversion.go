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

// nolint: revive
package v1alpha1

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*TaskRun)(nil)

// ConvertTo implements api.Convertible
func (source *TaskRun) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.TaskRun:
		sink.ObjectMeta = source.ObjectMeta
		if err := source.Spec.ConvertTo(ctx, &sink.Spec); err != nil {
			return err
		}
		sink.Status = source.Status
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (source *TaskRunSpec) ConvertTo(ctx context.Context, sink *v1beta1.TaskRunSpec) error {
	sink.ServiceAccountName = source.ServiceAccountName
	sink.TaskRef = source.TaskRef
	if source.TaskSpec != nil {
		sink.TaskSpec = &v1beta1.TaskSpec{}
		if err := source.TaskSpec.ConvertTo(ctx, sink.TaskSpec); err != nil {
			return err
		}
	}
	sink.Status = source.Status
	sink.Timeout = source.Timeout
	sink.PodTemplate = source.PodTemplate
	sink.Workspaces = source.Workspaces
	sink.Params = source.Params
	sink.Resources = source.Resources
	// Deprecated fields
	if source.Inputs != nil {
		if len(source.Inputs.Params) > 0 && len(source.Params) > 0 {
			// This shouldn't happen as it shouldn't pass validation
			return apis.ErrMultipleOneOf("inputs.params", "params")
		}
		if len(source.Inputs.Params) > 0 {
			sink.Params = make([]v1beta1.Param, len(source.Inputs.Params))
			for i, param := range source.Inputs.Params {
				sink.Params[i] = *param.DeepCopy()
			}
		}
		if len(source.Inputs.Resources) > 0 {
			if sink.Resources == nil {
				sink.Resources = &v1beta1.TaskRunResources{}
			}
			if len(source.Inputs.Resources) > 0 && source.Resources != nil && len(source.Resources.Inputs) > 0 {
				// This shouldn't happen as it shouldn't pass validation but just in case
				return apis.ErrMultipleOneOf("inputs.resources", "resources.inputs")
			}
			sink.Resources.Inputs = make([]v1beta1.TaskResourceBinding, len(source.Inputs.Resources))
			for i, resource := range source.Inputs.Resources {
				sink.Resources.Inputs[i] = v1beta1.TaskResourceBinding{
					PipelineResourceBinding: v1beta1.PipelineResourceBinding{
						Name:         resource.Name,
						ResourceRef:  resource.ResourceRef,
						ResourceSpec: resource.ResourceSpec,
					},
					Paths: resource.Paths,
				}
			}
		}
	}

	if source.Outputs != nil && len(source.Outputs.Resources) > 0 {
		if sink.Resources == nil {
			sink.Resources = &v1beta1.TaskRunResources{}
		}
		if len(source.Outputs.Resources) > 0 && source.Resources != nil && len(source.Resources.Outputs) > 0 {
			// This shouldn't happen as it shouldn't pass validation but just in case
			return apis.ErrMultipleOneOf("outputs.resources", "resources.outputs")
		}
		sink.Resources.Outputs = make([]v1beta1.TaskResourceBinding, len(source.Outputs.Resources))
		for i, resource := range source.Outputs.Resources {
			sink.Resources.Outputs[i] = v1beta1.TaskResourceBinding{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					Name:         resource.Name,
					ResourceRef:  resource.ResourceRef,
					ResourceSpec: resource.ResourceSpec,
				},
				Paths: resource.Paths,
			}
		}
	}
	return nil
}

// ConvertFrom implements api.Convertible
func (sink *TaskRun) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.TaskRun:
		sink.ObjectMeta = source.ObjectMeta
		if err := sink.Spec.ConvertFrom(ctx, &source.Spec); err != nil {
			return err
		}
		sink.Status = source.Status
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (sink *TaskRunSpec) ConvertFrom(ctx context.Context, source *v1beta1.TaskRunSpec) error {
	sink.ServiceAccountName = source.ServiceAccountName
	sink.TaskRef = source.TaskRef
	if source.TaskSpec != nil {
		sink.TaskSpec = &TaskSpec{}
		if err := sink.TaskSpec.ConvertFrom(ctx, source.TaskSpec); err != nil {
			return err
		}
	}
	sink.Status = source.Status
	sink.Timeout = source.Timeout
	sink.PodTemplate = source.PodTemplate
	sink.Workspaces = source.Workspaces
	sink.Params = source.Params
	sink.Resources = source.Resources
	return nil
}
