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

var _ apis.Convertible = (*TaskRun)(nil)

// ConvertTo implements api.Convertible
func (tr *TaskRun) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.TaskRun:
		sink.ObjectMeta = tr.ObjectMeta
		if err := tr.Spec.ConvertTo(ctx, &sink.Spec); err != nil {
			return err
		}
		sink.Status = tr.Status
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements api.Convertible
func (trs *TaskRunSpec) ConvertTo(ctx context.Context, sink *v1beta1.TaskRunSpec) error {
	sink.ServiceAccountName = trs.ServiceAccountName
	sink.TaskRef = trs.TaskRef
	if trs.TaskSpec != nil {
		sink.TaskSpec = &v1beta1.TaskSpec{}
		if err := trs.TaskSpec.ConvertTo(ctx, sink.TaskSpec); err != nil {
			return err
		}
	}
	sink.Status = trs.Status
	sink.Timeout = trs.Timeout
	sink.PodTemplate = trs.PodTemplate
	sink.Workspaces = trs.Workspaces
	sink.Params = trs.Params
	sink.Resources = trs.Resources
	// Deprecated fields
	if trs.Inputs != nil {
		if len(trs.Inputs.Params) > 0 && len(trs.Params) > 0 {
			// This shouldn't happen as it shouldn't pass validation
			return apis.ErrMultipleOneOf("inputs.params", "params")
		}
		if len(trs.Inputs.Params) > 0 {
			sink.Params = make([]v1beta1.Param, len(trs.Inputs.Params))
			for i, param := range trs.Inputs.Params {
				sink.Params[i] = *param.DeepCopy()
			}
		}
		if len(trs.Inputs.Resources) > 0 {
			if sink.Resources == nil {
				sink.Resources = &v1beta1.TaskRunResources{}
			}
			if len(trs.Inputs.Resources) > 0 && trs.Resources != nil && len(trs.Resources.Inputs) > 0 {
				// This shouldn't happen as it shouldn't pass validation but just in case
				return apis.ErrMultipleOneOf("inputs.resources", "resources.inputs")
			}
			sink.Resources.Inputs = make([]v1beta1.TaskResourceBinding, len(trs.Inputs.Resources))
			for i, resource := range trs.Inputs.Resources {
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

	if trs.Outputs != nil && len(trs.Outputs.Resources) > 0 {
		if sink.Resources == nil {
			sink.Resources = &v1beta1.TaskRunResources{}
		}
		if len(trs.Outputs.Resources) > 0 && trs.Resources != nil && len(trs.Resources.Outputs) > 0 {
			// This shouldn't happen as it shouldn't pass validation but just in case
			return apis.ErrMultipleOneOf("outputs.resources", "resources.outputs")
		}
		sink.Resources.Outputs = make([]v1beta1.TaskResourceBinding, len(trs.Outputs.Resources))
		for i, resource := range trs.Outputs.Resources {
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
func (tr *TaskRun) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.TaskRun:
		tr.ObjectMeta = source.ObjectMeta
		if err := tr.Spec.ConvertFrom(ctx, &source.Spec); err != nil {
			return err
		}
		tr.Status = source.Status
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", tr)
	}
}

// ConvertFrom implements api.Convertible
func (trs *TaskRunSpec) ConvertFrom(ctx context.Context, source *v1beta1.TaskRunSpec) error {
	trs.ServiceAccountName = source.ServiceAccountName
	trs.TaskRef = source.TaskRef
	if source.TaskSpec != nil {
		trs.TaskSpec = &TaskSpec{}
		if err := trs.TaskSpec.ConvertFrom(ctx, source.TaskSpec); err != nil {
			return err
		}
	}
	trs.Status = source.Status
	trs.Timeout = source.Timeout
	trs.PodTemplate = source.PodTemplate
	trs.Workspaces = source.Workspaces
	trs.Params = source.Params
	trs.Resources = source.Resources
	return nil
}
