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
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"

	"k8s.io/apimachinery/pkg/util/sets"
)

// validateParams validates that all Pipeline Task, Matrix.Params and Matrix.Include parameters all have values, match the specified
// type and object params have all the keys required
func validateParams(ctx context.Context, paramSpecs []v1.ParamSpec, params v1.Params, matrixParams v1.Params) error {
	if paramSpecs == nil {
		return nil
	}
	neededParamsNames, neededParamsTypes := neededParamsNamesAndTypes(paramSpecs)
	providedParams := params
	providedParams = append(providedParams, matrixParams...)
	providedParamsNames := providedParams.ExtractNames()
	if missingParamsNames := missingParamsNames(neededParamsNames, providedParamsNames, paramSpecs); len(missingParamsNames) != 0 {
		return fmt.Errorf("missing values for these params which have no default values: %s", missingParamsNames)
	}
	if wrongTypeParamNames := wrongTypeParamsNames(params, matrixParams, neededParamsTypes); len(wrongTypeParamNames) != 0 {
		return fmt.Errorf("param types don't match the user-specified type: %s", wrongTypeParamNames)
	}
	if missingKeysObjectParamNames := MissingKeysObjectParamNames(paramSpecs, params); len(missingKeysObjectParamNames) != 0 {
		return fmt.Errorf("missing keys for these params which are required in ParamSpec's properties %v", missingKeysObjectParamNames)
	}
	return nil
}

// neededParamsNamesAndTypes returns the needed parameter names and types based on the paramSpec
func neededParamsNamesAndTypes(paramSpecs []v1.ParamSpec) (sets.String, map[string]v1.ParamType) {
	neededParamsNames := sets.String{}
	neededParamsTypes := make(map[string]v1.ParamType)
	for _, inputResourceParam := range paramSpecs {
		neededParamsNames.Insert(inputResourceParam.Name)
		neededParamsTypes[inputResourceParam.Name] = inputResourceParam.Type
	}
	return neededParamsNames, neededParamsTypes
}

// missingParamsNames returns a slice of missing parameter names that have not been declared with a default value
// in the paramSpec
func missingParamsNames(neededParams sets.String, providedParams sets.String, paramSpecs []v1.ParamSpec) []string {
	missingParamsNames := neededParams.Difference(providedParams)
	var missingParamsNamesWithNoDefaults []string
	for _, inputResourceParam := range paramSpecs {
		if missingParamsNames.Has(inputResourceParam.Name) && inputResourceParam.Default == nil {
			missingParamsNamesWithNoDefaults = append(missingParamsNamesWithNoDefaults, inputResourceParam.Name)
		}
	}
	return missingParamsNamesWithNoDefaults
}
func wrongTypeParamsNames(params []v1.Param, matrix v1.Params, neededParamsTypes map[string]v1.ParamType) []string {
	// TODO(#4723): validate that $(task.taskname.result.resultname) is invalid for array and object type.
	// It should be used to refer string and need to add [*] to refer to array or object.
	var wrongTypeParamNames []string
	for _, param := range params {
		if _, ok := neededParamsTypes[param.Name]; !ok {
			// Ignore any missing params - this happens when extra params were
			// passed to the task that aren't being used.
			continue
		}
		// This is needed to support array replacements in params. Users want to use $(tasks.taskName.results.resultname[*])
		// to pass array result to array param, yet in yaml format this will be
		// unmarshalled to string for ParamValues. So we need to check and skip this validation.
		// Please refer issue #4879 for more details and examples.
		if param.Value.Type == v1.ParamTypeString && (neededParamsTypes[param.Name] == v1.ParamTypeArray || neededParamsTypes[param.Name] == v1.ParamTypeObject) && v1.VariableSubstitutionRegex.MatchString(param.Value.StringVal) {
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
		// Matrix param replacements must be of type String
		if neededParamsTypes[param.Name] != v1.ParamTypeString {
			wrongTypeParamNames = append(wrongTypeParamNames, param.Name)
		}
	}
	return wrongTypeParamNames
}

// MissingKeysObjectParamNames checks if all required keys of object type param definitions are provided in params or param definitions' defaults.
func MissingKeysObjectParamNames(paramSpecs []v1.ParamSpec, params v1.Params) map[string][]string {
	neededKeys := make(map[string][]string)
	providedKeys := make(map[string][]string)

	for _, spec := range paramSpecs {
		if spec.Type == v1.ParamTypeObject {
			// collect required keys from properties section
			for key := range spec.Properties {
				neededKeys[spec.Name] = append(neededKeys[spec.Name], key)
			}

			// collect provided keys from default
			if spec.Default != nil && spec.Default.ObjectVal != nil {
				for key := range spec.Default.ObjectVal {
					providedKeys[spec.Name] = append(providedKeys[spec.Name], key)
				}
			}
		}
	}

	// collect provided keys from run level value
	for _, p := range params {
		if p.Value.Type == v1.ParamTypeObject {
			for key := range p.Value.ObjectVal {
				providedKeys[p.Name] = append(providedKeys[p.Name], key)
			}
		}
	}

	return findMissingKeys(neededKeys, providedKeys)
}

// findMissingKeys checks if objects have missing keys in its providers (taskrun value and default)
func findMissingKeys(neededKeys, providedKeys map[string][]string) map[string][]string {
	missings := map[string][]string{}
	for p, keys := range providedKeys {
		if _, ok := neededKeys[p]; !ok {
			// Ignore any missing objects - this happens when object param is provided with default
			continue
		}
		missedKeys := list.DiffLeft(neededKeys[p], keys)
		if len(missedKeys) != 0 {
			missings[p] = missedKeys
		}
	}

	return missings
}

// ValidateResolvedTask validates that all parameters declared in the TaskSpec are present in the taskrun
// It also validates that all parameters have values, parameter types match the specified type and
// object params have all the keys required
func ValidateResolvedTask(ctx context.Context, params []v1.Param, matrix *v1.Matrix, rtr *resources.ResolvedTask) error {
	var paramSpecs v1.ParamSpecs
	if rtr != nil {
		paramSpecs = rtr.TaskSpec.Params
	}
	if err := validateParams(ctx, paramSpecs, params, matrix.GetAllParams()); err != nil {
		return fmt.Errorf("invalid input params for task %s: %w", rtr.TaskName, err)
	}
	return nil
}

func validateTaskSpecRequestResources(taskSpec *v1.TaskSpec) error {
	if taskSpec != nil {
		for _, step := range taskSpec.Steps {
			for k, request := range step.ComputeResources.Requests {
				// First validate the limit in step
				if limit, ok := step.ComputeResources.Limits[k]; ok {
					if (&limit).Cmp(request) == -1 {
						return fmt.Errorf("Invalid request resource value: %v must be less or equal to limit %v", request.String(), limit.String())
					}
				} else if taskSpec.StepTemplate != nil {
					// If step doesn't configure the limit, validate the limit in stepTemplate
					if limit, ok := taskSpec.StepTemplate.ComputeResources.Limits[k]; ok {
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
func validateOverrides(ts *v1.TaskSpec, trs *v1.TaskRunSpec) error {
	stepErr := validateStepOverrides(ts, trs)
	sidecarErr := validateSidecarOverrides(ts, trs)
	return multierror.Append(stepErr, sidecarErr).ErrorOrNil()
}

func validateStepOverrides(ts *v1.TaskSpec, trs *v1.TaskRunSpec) error {
	var err error
	stepNames := sets.NewString()
	for _, step := range ts.Steps {
		stepNames.Insert(step.Name)
	}
	for _, stepOverride := range trs.StepSpecs {
		if !stepNames.Has(stepOverride.Name) {
			err = multierror.Append(err, fmt.Errorf("invalid StepOverride: No Step named %s", stepOverride.Name))
		}
	}
	return err
}

func validateSidecarOverrides(ts *v1.TaskSpec, trs *v1.TaskRunSpec) error {
	var err error
	sidecarNames := sets.NewString()
	for _, sidecar := range ts.Sidecars {
		sidecarNames.Insert(sidecar.Name)
	}
	for _, sidecarOverride := range trs.SidecarSpecs {
		if !sidecarNames.Has(sidecarOverride.Name) {
			err = multierror.Append(err, fmt.Errorf("invalid SidecarOverride: No Sidecar named %s", sidecarOverride.Name))
		}
	}
	return err
}

// validateResults checks the emitted results type and object properties against the ones defined in spec.
func validateTaskRunResults(tr *v1.TaskRun, resolvedTaskSpec *v1.TaskSpec) error {
	specResults := []v1.TaskResult{}
	if tr.Spec.TaskSpec != nil {
		specResults = append(specResults, tr.Spec.TaskSpec.Results...)
	}

	if resolvedTaskSpec != nil {
		specResults = append(specResults, resolvedTaskSpec.Results...)
	}

	// When get the results, check if the type of result is the expected one
	if missmatchedTypes := mismatchedTypesResults(tr, specResults); len(missmatchedTypes) != 0 {
		var s []string
		for k, v := range missmatchedTypes {
			s = append(s, fmt.Sprintf(" \"%v\": %v", k, v))
		}
		sort.Strings(s)
		return fmt.Errorf("Provided results don't match declared results; may be invalid JSON or missing result declaration: %v", strings.Join(s, ","))
	}

	// When get the results, for object value need to check if they have missing keys.
	if missingKeysObjectNames := missingKeysofObjectResults(tr, specResults); len(missingKeysObjectNames) != 0 {
		return fmt.Errorf("missing keys for these results which are required in TaskResult's properties %v", missingKeysObjectNames)
	}
	return nil
}

// mismatchedTypesResults checks and returns all the mismatched types of emitted results against specified results.
func mismatchedTypesResults(tr *v1.TaskRun, specResults []v1.TaskResult) map[string]string {
	neededTypes := make(map[string]string)
	mismatchedTypes := make(map[string]string)
	var filteredResults []v1.TaskRunResult
	// collect needed types for results
	for _, r := range specResults {
		neededTypes[r.Name] = string(r.Type)
	}

	// collect mismatched types for results, and correct results in filteredResults
	// TODO(#6097): Validate if the emitted results are defined in taskspec
	for _, trr := range tr.Status.Results {
		needed, ok := neededTypes[trr.Name]
		if ok && needed != string(trr.Type) {
			mismatchedTypes[trr.Name] = fmt.Sprintf("task result is expected to be \"%v\" type but was initialized to a different type \"%v\"", needed, trr.Type)
		} else {
			filteredResults = append(filteredResults, trr)
		}
	}
	// remove the mismatched results
	tr.Status.Results = filteredResults
	return mismatchedTypes
}

// missingKeysofObjectResults checks and returns the missing keys of object results.
func missingKeysofObjectResults(tr *v1.TaskRun, specResults []v1.TaskResult) map[string][]string {
	neededKeys := make(map[string][]string)
	providedKeys := make(map[string][]string)
	// collect needed keys for object results
	for _, r := range specResults {
		if string(r.Type) == string(v1.ParamTypeObject) {
			for key := range r.Properties {
				neededKeys[r.Name] = append(neededKeys[r.Name], key)
			}
		}
	}

	// collect provided keys for object results
	for _, trr := range tr.Status.Results {
		if trr.Value.Type == v1.ParamTypeObject {
			for key := range trr.Value.ObjectVal {
				providedKeys[trr.Name] = append(providedKeys[trr.Name], key)
			}
		}
	}
	return findMissingKeys(neededKeys, providedKeys)
}
