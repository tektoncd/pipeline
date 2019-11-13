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

package taskrun

import (
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
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
	optional := make([]string, 0, len(requiredResources))
	for _, resource := range requiredResources {
		if resource.Optional {
			// create a list of optional resources
			optional = append(optional, resource.Name)
		} else {
			// create a list of required resources
			required = append(required, resource.Name)
		}
	}
	provided := make([]string, 0, len(providedResources))
	for resource := range providedResources {
		provided = append(provided, resource)
	}
	// verify that the list of required resources does exist in the provided resources
	missing := list.DiffLeft(required, provided)
	if len(missing) > 0 {
		return fmt.Errorf("Task's declared required resources are missing from the TaskRun: %s", missing)
	}
	// verify that the list of provided resources does not have any extra resources (outside of required and optional resources combined)
	extra := list.DiffLeft(provided, append(required, optional...))
	if len(extra) > 0 {
		return fmt.Errorf("TaskRun's declared resources didn't match usage in Task: %s", extra)
	}
	for _, resource := range requiredResources {
		r := providedResources[resource.Name]
		if !resource.Optional && r == nil {
			// This case should never be hit due to the check for missing resources at the beginning of the function
			return fmt.Errorf("resource %q is missing", resource.Name)
		}
		if r != nil && resource.Type != r.Spec.Type {
			return fmt.Errorf("resource %q should be type %q but was %q", resource.Name, r.Spec.Type, resource.Type)
		}
	}
	return nil
}

func validateParams(inputs *v1alpha1.Inputs, params []v1alpha1.Param) error {
	var neededParams []string
	paramTypes := make(map[string]v1alpha1.ParamType)
	if inputs != nil {
		neededParams = make([]string, 0, len(inputs.Params))
		for _, inputResourceParam := range inputs.Params {
			neededParams = append(neededParams, inputResourceParam.Name)
			paramTypes[inputResourceParam.Name] = inputResourceParam.Type
		}
	}
	providedParams := make([]string, 0, len(params))
	for _, param := range params {
		providedParams = append(providedParams, param.Name)
	}
	missingParams := list.DiffLeft(neededParams, providedParams)
	var missingParamsNoDefaults []string
	for _, param := range missingParams {
		for _, inputResourceParam := range inputs.Params {
			if inputResourceParam.Name == param && inputResourceParam.Default == nil {
				missingParamsNoDefaults = append(missingParamsNoDefaults, param)
			}
		}
	}
	if len(missingParamsNoDefaults) > 0 {
		return fmt.Errorf("missing values for these params which have no default values: %s", missingParamsNoDefaults)
	}
	extraParams := list.DiffLeft(providedParams, neededParams)
	if len(extraParams) != 0 {
		return fmt.Errorf("didn't need these params but they were provided anyway: %s", extraParams)
	}

	// Now that we have checked against missing/extra params, make sure each param's actual type matches
	// the user-specified type.
	var wrongTypeParamNames []string
	for _, param := range params {
		if param.Value.Type != paramTypes[param.Name] {
			wrongTypeParamNames = append(wrongTypeParamNames, param.Name)
		}
	}
	if len(wrongTypeParamNames) != 0 {
		return fmt.Errorf("param types don't match the user-specified type: %s", wrongTypeParamNames)
	}

	return nil
}

// ValidateResolvedTaskResources validates task inputs, params and output matches taskrun
func ValidateResolvedTaskResources(params []v1alpha1.Param, rtr *resources.ResolvedTaskResources) error {
	if err := validateParams(rtr.TaskSpec.Inputs, params); err != nil {
		return fmt.Errorf("invalid input params: %w", err)
	}
	if err := validateInputResources(rtr.TaskSpec.Inputs, rtr.Inputs); err != nil {
		return fmt.Errorf("invalid input resources: %w", err)
	}
	if err := validateOutputResources(rtr.TaskSpec.Outputs, rtr.Outputs); err != nil {
		return fmt.Errorf("invalid output resources: %w", err)
	}

	return nil
}
