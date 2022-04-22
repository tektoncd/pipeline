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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/hashicorp/go-multierror"
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

func validateParams(ctx context.Context, paramSpecs []v1beta1.ParamSpec, params []v1beta1.Param, matrix []v1beta1.Param) error {
	neededParamsNames, neededParamsTypes := neededParamsNamesAndTypes(paramSpecs)
	providedParamsNames := providedParamsNames(append(params, matrix...))
	if missingParamsNames := missingParamsNames(neededParamsNames, providedParamsNames, paramSpecs); len(missingParamsNames) != 0 {
		return fmt.Errorf("missing values for these params which have no default values: %s", missingParamsNames)
	}
	if extraParamsNames := extraParamsNames(ctx, neededParamsNames, providedParamsNames); len(extraParamsNames) != 0 {
		return fmt.Errorf("didn't need these params but they were provided anyway: %s", extraParamsNames)
	}
	if wrongTypeParamNames := wrongTypeParamsNames(params, matrix, neededParamsTypes); len(wrongTypeParamNames) != 0 {
		return fmt.Errorf("param types don't match the user-specified type: %s", wrongTypeParamNames)
	}
	if missingKeysObjectParamNames := missingKeysObjectParamNames(paramSpecs, params); len(missingKeysObjectParamNames) != 0 {
		return fmt.Errorf("missing keys for these params which are required in ParamSpec's properties %v", missingKeysObjectParamNames)
	}

	return nil
}

func neededParamsNamesAndTypes(paramSpecs []v1beta1.ParamSpec) ([]string, map[string]v1beta1.ParamType) {
	var neededParamsNames []string
	neededParamsTypes := make(map[string]v1beta1.ParamType)
	neededParamsNames = make([]string, 0, len(paramSpecs))
	for _, inputResourceParam := range paramSpecs {
		neededParamsNames = append(neededParamsNames, inputResourceParam.Name)
		neededParamsTypes[inputResourceParam.Name] = inputResourceParam.Type
	}
	return neededParamsNames, neededParamsTypes
}

func providedParamsNames(params []v1beta1.Param) []string {
	providedParamsNames := make([]string, 0, len(params))
	for _, param := range params {
		providedParamsNames = append(providedParamsNames, param.Name)
	}
	return providedParamsNames
}

func missingParamsNames(neededParams []string, providedParams []string, paramSpecs []v1beta1.ParamSpec) []string {
	missingParamsNames := list.DiffLeft(neededParams, providedParams)
	var missingParamsNamesWithNoDefaults []string
	for _, param := range missingParamsNames {
		for _, inputResourceParam := range paramSpecs {
			if inputResourceParam.Name == param && inputResourceParam.Default == nil {
				missingParamsNamesWithNoDefaults = append(missingParamsNamesWithNoDefaults, param)
			}
		}
	}
	return missingParamsNamesWithNoDefaults
}

func extraParamsNames(ctx context.Context, neededParams []string, providedParams []string) []string {
	// If alpha features are enabled, disable checking for extra params.
	// Extra params are needed to support
	// https://github.com/tektoncd/community/blob/main/teps/0023-implicit-mapping.md
	// So that parent params can be passed down to subresources (even if they
	// are not explicitly used).
	if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields != "alpha" {
		return list.DiffLeft(providedParams, neededParams)
	}
	return nil
}

func wrongTypeParamsNames(params []v1beta1.Param, matrix []v1beta1.Param, neededParamsTypes map[string]v1beta1.ParamType) []string {
	var wrongTypeParamNames []string
	for _, param := range params {
		if _, ok := neededParamsTypes[param.Name]; !ok {
			// Ignore any missing params - this happens when extra params were
			// passed to the task that aren't being used.
			continue
		}
		if param.Value.Type != neededParamsTypes[param.Name] {
			wrongTypeParamNames = append(wrongTypeParamNames, param.Name)
		}
	}
	for _, param := range matrix {
		if _, ok := neededParamsTypes[param.Name]; !ok {
			// Ignore any missing params - this happens when extra params were
			// passed to the task that aren't being used.
			continue
		}
		if neededParamsTypes[param.Name] != v1beta1.ParamTypeString {
			wrongTypeParamNames = append(wrongTypeParamNames, param.Name)
		}
	}
	return wrongTypeParamNames
}

// missingKeysObjectParamNames checks if all required keys of object type params are provided in taskrun params.
// TODO (@chuangw6): This might be refactored out to support single pair validation.
func missingKeysObjectParamNames(paramSpecs []v1beta1.ParamSpec, params []v1beta1.Param) []string {
	neededKeys := make(map[string][]string)
	providedKeys := make(map[string][]string)

	// collect needed keys for object parameters
	for _, spec := range paramSpecs {
		if spec.Type == v1beta1.ParamTypeObject {
			for key := range spec.Properties {
				neededKeys[spec.Name] = append(neededKeys[spec.Name], key)
			}
		}
	}

	// collect provided keys for object parameters
	for _, p := range params {
		if p.Value.Type == v1beta1.ParamTypeObject {
			for key := range p.Value.ObjectVal {
				providedKeys[p.Name] = append(providedKeys[p.Name], key)
			}
		}
	}

	return validateObjectKeys(neededKeys, providedKeys)
}

// validateObjectKeys checks if objects have missing keys in its provider (either taskrun value or result value)
func validateObjectKeys(neededObjectKeys, providedObjectKeys map[string][]string) []string {
	missings := []string{}
	for p, keys := range providedObjectKeys {
		if _, ok := neededObjectKeys[p]; !ok {
			// Ignore any missing objects - this happens when extra objects were
			// passed that aren't being used.
			continue
		}
		missedKeys := list.DiffLeft(neededObjectKeys[p], keys)
		if len(missedKeys) != 0 {
			missings = append(missings, p)
		}
	}

	return missings
}

// ValidateResolvedTaskResources validates task inputs, params and output matches taskrun
func ValidateResolvedTaskResources(ctx context.Context, params []v1beta1.Param, matrix []v1beta1.Param, rtr *resources.ResolvedTaskResources) error {
	if err := validateParams(ctx, rtr.TaskSpec.Params, params, matrix); err != nil {
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

// validateOverrides validates that all stepOverrides map to valid steps, and likewise for sidecarOverrides
func validateOverrides(ctx context.Context, ts *v1beta1.TaskSpec, trs *v1beta1.TaskRunSpec) error {
	stepErr := validateStepOverrides(ctx, ts, trs)
	sidecarErr := validateSidecarOverrides(ctx, ts, trs)
	return multierror.Append(stepErr, sidecarErr).ErrorOrNil()
}

func validateStepOverrides(ctx context.Context, ts *v1beta1.TaskSpec, trs *v1beta1.TaskRunSpec) error {
	var err error
	stepNames := sets.NewString()
	for _, step := range ts.Steps {
		stepNames.Insert(step.Name)
	}
	for _, stepOverride := range trs.StepOverrides {
		if !stepNames.Has(stepOverride.Name) {
			err = multierror.Append(err, fmt.Errorf("invalid StepOverride: No Step named %s", stepOverride.Name))
		}
	}
	return err
}

func validateSidecarOverrides(ctx context.Context, ts *v1beta1.TaskSpec, trs *v1beta1.TaskRunSpec) error {
	var err error
	sidecarNames := sets.NewString()
	for _, sidecar := range ts.Sidecars {
		sidecarNames.Insert(sidecar.Name)
	}
	for _, sidecarOverride := range trs.SidecarOverrides {
		if !sidecarNames.Has(sidecarOverride.Name) {
			err = multierror.Append(err, fmt.Errorf("invalid SidecarOverride: No Sidecar named %s", sidecarOverride.Name))
		}
	}
	return err
}
