/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either extress or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package taskrun

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"golang.org/x/xerrors"
)

func validateInputResources(inputs *v1alpha1.Inputs, providedResources map[string]*v1alpha1.PipelineResource) error {
	if inputs != nil {
		return validateResources(inputs.Resources, providedResources)
	}
	return validateResources([]v1alpha1.TaskResource{}, providedResources)
}

func validateOutputResources(outputs *v1alpha1.Outputs, providedResources map[string]*v1alpha1.PipelineResource) error {
	if outputs != nil {
		return validateResources(outputs.Resources, providedResources)
	}
	return validateResources([]v1alpha1.TaskResource{}, providedResources)
}

func validateResources(requiredResources []v1alpha1.TaskResource, providedResources map[string]*v1alpha1.PipelineResource) error {
	required := make([]string, 0, len(requiredResources))
	for _, resource := range requiredResources {
		required = append(required, resource.Name)
	}
	provided := make([]string, 0, len(providedResources))
	for resource := range providedResources {
		provided = append(provided, resource)
	}
	err := list.IsSame(required, provided)
	if err != nil {
		return xerrors.Errorf("TaskRun's declared resources didn't match usage in Task: %w", err)
	}
	for _, resource := range requiredResources {
		r := providedResources[resource.Name]
		if r == nil {
			// This case should never be hit due to the check for missing resources at the beginning of the function
			return xerrors.Errorf("resource %q is missing", resource.Name)
		}
		if resource.Type != r.Spec.Type {
			return xerrors.Errorf("resource %q should be type %q but was %q", resource.Name, r.Spec.Type, resource.Type)
		}
	}
	return nil
}

func validateParams(inputs *v1alpha1.Inputs, params []v1alpha1.Param) error {
	neededParams := []string{}
	if inputs != nil {
		neededParams = make([]string, 0, len(inputs.Params))
		for _, inputResourceParam := range inputs.Params {
			neededParams = append(neededParams, inputResourceParam.Name)
		}
	}
	providedParams := make([]string, 0, len(params))
	for _, param := range params {
		providedParams = append(providedParams, param.Name)
	}
	missingParams := list.DiffLeft(neededParams, providedParams)
	missingParamsNoDefaults := []string{}
	for _, param := range missingParams {
		for _, inputResourceParam := range inputs.Params {
			if inputResourceParam.Name == param && inputResourceParam.Default == "" {
				missingParamsNoDefaults = append(missingParamsNoDefaults, param)
			}
		}
	}
	if len(missingParamsNoDefaults) > 0 {
		return xerrors.Errorf("missing values for these params which have no default values: %s", missingParamsNoDefaults)
	}
	extraParams := list.DiffLeft(providedParams, neededParams)
	if len(extraParams) != 0 {
		return xerrors.Errorf("didn't need these params but they were provided anyway: %s", extraParams)
	}
	return nil
}

// ValidateResolvedTaskResources validates task inputs, params and output matches taskrun
func ValidateResolvedTaskResources(params []v1alpha1.Param, rtr *resources.ResolvedTaskResources) error {
	if err := validateParams(rtr.TaskSpec.Inputs, params); err != nil {
		return xerrors.Errorf("invalid input params: %w", err)
	}
	if err := validateInputResources(rtr.TaskSpec.Inputs, rtr.Inputs); err != nil {
		return xerrors.Errorf("invalid input resources: %w", err)
	}
	if err := validateOutputResources(rtr.TaskSpec.Outputs, rtr.Outputs); err != nil {
		return xerrors.Errorf("invalid output resources: %w", err)
	}

	return nil
}
