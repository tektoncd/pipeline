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
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
)

func validateResources(requiredResources []v1beta1.TaskResource, providedResources map[string]*resourcev1alpha1.PipelineResource) error {
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

func validateParams(ctx context.Context, paramSpecs []v1beta1.ParamSpec, params []v1beta1.Param) error {
	var neededParams []string
	paramTypes := make(map[string]v1beta1.ParamType)
	neededParams = make([]string, 0, len(paramSpecs))
	for _, inputResourceParam := range paramSpecs {
		neededParams = append(neededParams, inputResourceParam.Name)
		paramTypes[inputResourceParam.Name] = inputResourceParam.Type
	}
	providedParams := make([]string, 0, len(params))
	for _, param := range params {
		providedParams = append(providedParams, param.Name)
	}
	missingParams := list.DiffLeft(neededParams, providedParams)
	var missingParamsNoDefaults []string
	for _, param := range missingParams {
		for _, inputResourceParam := range paramSpecs {
			if inputResourceParam.Name == param && inputResourceParam.Default == nil {
				missingParamsNoDefaults = append(missingParamsNoDefaults, param)
			}
		}
	}
	if len(missingParamsNoDefaults) > 0 {
		return fmt.Errorf("missing values for these params which have no default values: %s", missingParamsNoDefaults)
	}
	// If alpha features are enabled, disable checking for extra params.
	// Extra params are needed to support
	// https://github.com/tektoncd/community/blob/main/teps/0023-implicit-mapping.md
	// So that parent params can be passed down to subresources (even if they
	// are not explicitly used).
	if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields != "alpha" {
		extraParams := list.DiffLeft(providedParams, neededParams)
		if len(extraParams) != 0 {
			return fmt.Errorf("didn't need these params but they were provided anyway: %s", extraParams)
		}
	}

	// Now that we have checked against missing/extra params, make sure each param's actual type matches
	// the user-specified type.
	var wrongTypeParamNames []string
	for _, param := range params {
		if _, ok := paramTypes[param.Name]; !ok {
			// Ignore any missing params - this happens when extra params were
			// passed to the task that aren't being used.
			continue
		}
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
func ValidateResolvedTaskResources(ctx context.Context, params []v1beta1.Param, rtr *resources.ResolvedTaskResources) error {
	if err := validateParams(ctx, rtr.TaskSpec.Params, params); err != nil {
		return fmt.Errorf("invalid input params for task %s: %w", rtr.TaskName, err)
	}
	inputs := []v1beta1.TaskResource{}
	outputs := []v1beta1.TaskResource{}
	if rtr.TaskSpec.Resources != nil {
		inputs = rtr.TaskSpec.Resources.Inputs
		outputs = rtr.TaskSpec.Resources.Outputs
	}
	if err := validateResources(inputs, rtr.Inputs); err != nil {
		return fmt.Errorf("invalid input resources for task %s: %w", rtr.TaskName, err)
	}
	if err := validateResources(outputs, rtr.Outputs); err != nil {
		return fmt.Errorf("invalid output resources for task %s: %w", rtr.TaskName, err)
	}

	return nil
}

func validateTaskSpecRequestResources(ctx context.Context, taskSpec *v1beta1.TaskSpec) error {
	if taskSpec != nil {
		for _, step := range taskSpec.Steps {
			for k, request := range step.Resources.Requests {
				// First validate the limit in step
				if limit, ok := step.Resources.Limits[k]; ok {
					if (&limit).Cmp(request) == -1 {
						return fmt.Errorf("Invalid request resource value: %v must be less or equal to limit %v", request.String(), limit.String())
					}
				} else if taskSpec.StepTemplate != nil {
					// If step doesn't configure the limit, validate the limit in stepTemplate
					if limit, ok := taskSpec.StepTemplate.Resources.Limits[k]; ok {
						if (&limit).Cmp(request) == -1 {
							return fmt.Errorf("Invalid request resource value: %v must be less or equal to limit %v", request.String(), limit.String())
						}
					}

				}

			}
		}
	}

	return nil
}
