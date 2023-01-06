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
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
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

func validateParams(ctx context.Context, paramSpecs []v1.ParamSpec, params []v1.Param, matrix *v1.Matrix) error {
	neededParamsNames, neededParamsTypes := neededParamsNamesAndTypes(paramSpecs)
	var matrixParams []v1.Param
	if matrix != nil {
		matrixParams = matrix.Params
	}
	providedParamsNames := providedParamsNames(append(params, matrixParams...))
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

func neededParamsNamesAndTypes(paramSpecs []v1.ParamSpec) ([]string, map[string]v1.ParamType) {
	var neededParamsNames []string
	neededParamsTypes := make(map[string]v1.ParamType)
	neededParamsNames = make([]string, 0, len(paramSpecs))
	for _, inputResourceParam := range paramSpecs {
		neededParamsNames = append(neededParamsNames, inputResourceParam.Name)
		neededParamsTypes[inputResourceParam.Name] = inputResourceParam.Type
	}
	return neededParamsNames, neededParamsTypes
}

func providedParamsNames(params []v1.Param) []string {
	providedParamsNames := make([]string, 0, len(params))
	for _, param := range params {
		providedParamsNames = append(providedParamsNames, param.Name)
	}
	return providedParamsNames
}

func missingParamsNames(neededParams []string, providedParams []string, paramSpecs []v1.ParamSpec) []string {
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

func wrongTypeParamsNames(params []v1.Param, matrix []v1.Param, neededParamsTypes map[string]v1.ParamType) []string {
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
		if neededParamsTypes[param.Name] != v1.ParamTypeString {
			wrongTypeParamNames = append(wrongTypeParamNames, param.Name)
		}
	}
	return wrongTypeParamNames
}

// MissingKeysObjectParamNames checks if all required keys of object type params are provided in taskrun params or taskSpec's default.
func MissingKeysObjectParamNames(paramSpecs []v1.ParamSpec, params []v1.Param) map[string][]string {
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

// ValidateResolvedTaskResources validates task inputs, params and output matches taskrun
func ValidateResolvedTaskResources(ctx context.Context, params []v1.Param, matrix *v1.Matrix, rtr *resources.ResolvedTaskResources) error {
	if err := validateParams(ctx, rtr.TaskSpec.Params, params, matrix); err != nil {
		return fmt.Errorf("invalid input params for task %s: %w", rtr.TaskName, err)
	}
	inputs := []v1beta1.TaskResource{}
	outputs := []v1beta1.TaskResource{}
	//TODO: add a field in rtr?
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
		return fmt.Errorf("missmatched Types for these results, %v", missmatchedTypes)
	}

	// When get the results, for object value need to check if they have missing keys.
	if missingKeysObjectNames := missingKeysofObjectResults(tr, specResults); len(missingKeysObjectNames) != 0 {
		return fmt.Errorf("missing keys for these results which are required in TaskResult's properties %v", missingKeysObjectNames)
	}
	return nil
}

// mismatchedTypesResults checks and returns all the mismatched types of emitted results against specified results.
func mismatchedTypesResults(tr *v1.TaskRun, specResults []v1.TaskResult) map[string][]string {
	neededTypes := make(map[string][]string)
	providedTypes := make(map[string][]string)
	// collect needed keys for object results
	for _, r := range specResults {
		neededTypes[r.Name] = append(neededTypes[r.Name], string(r.Type))
	}

	// collect provided keys for object results
	for _, trr := range tr.Status.TaskRunResults {
		providedTypes[trr.Name] = append(providedTypes[trr.Name], string(trr.Type))
	}
	return findMissingKeys(neededTypes, providedTypes)
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
	for _, trr := range tr.Status.TaskRunResults {
		if trr.Value.Type == v1.ParamTypeObject {
			for key := range trr.Value.ObjectVal {
				providedKeys[trr.Name] = append(providedKeys[trr.Name], key)
			}
		}
	}
	return findMissingKeys(neededKeys, providedKeys)
}

func validateParamArrayIndex(ctx context.Context, params []v1.Param, spec *v1.TaskSpec) error {
	cfg := config.FromContextOrDefaults(ctx)
	if cfg.FeatureFlags.EnableAPIFields != config.AlphaAPIFields {
		return nil
	}

	var defaults []v1.ParamSpec
	if len(spec.Params) > 0 {
		defaults = append(defaults, spec.Params...)
	}
	// Collect all array params
	arrayParams := make(map[string]int)

	patterns := []string{
		"$(params.%s)",
		"$(params[%q])",
		"$(params['%s'])",
	}

	// Collect array params lengths from defaults
	for _, p := range defaults {
		if p.Default != nil {
			if p.Default.Type == v1.ParamTypeArray {
				for _, pattern := range patterns {
					for i := 0; i < len(p.Default.ArrayVal); i++ {
						arrayParams[fmt.Sprintf(pattern, p.Name)] = len(p.Default.ArrayVal)
					}
				}
			}
		}
	}

	// Collect array params lengths from pipeline
	for _, p := range params {
		if p.Value.Type == v1.ParamTypeArray {
			for _, pattern := range patterns {
				for i := 0; i < len(p.Value.ArrayVal); i++ {
					arrayParams[fmt.Sprintf(pattern, p.Name)] = len(p.Value.ArrayVal)
				}
			}
		}
	}

	outofBoundParams := sets.String{}

	// Validate array param in steps fields.
	validateStepsParamArrayIndexing(spec.Steps, arrayParams, &outofBoundParams)

	// Validate array param in StepTemplate fields.
	validateStepsTemplateParamArrayIndexing(spec.StepTemplate, arrayParams, &outofBoundParams)

	// Validate array param in build's volumes
	validateVolumesParamArrayIndexing(spec.Volumes, arrayParams, &outofBoundParams)

	for _, v := range spec.Workspaces {
		extractParamIndex(v.MountPath, arrayParams, &outofBoundParams)
	}

	validateSidecarsParamArrayIndexing(spec.Sidecars, arrayParams, &outofBoundParams)

	if outofBoundParams.Len() > 0 {
		return fmt.Errorf("non-existent param references:%v", outofBoundParams.List())
	}

	return nil
}

func extractParamIndex(paramReference string, arrayParams map[string]int, outofBoundParams *sets.String) {
	list := substitution.ExtractParamsExpressions(paramReference)
	for _, val := range list {
		indexString := substitution.ExtractIndexString(paramReference)
		idx, _ := substitution.ExtractIndex(indexString)
		v := substitution.TrimArrayIndex(val)
		if paramLength, ok := arrayParams[v]; ok {
			if idx >= paramLength {
				outofBoundParams.Insert(val)
			}
		}
	}
}

func validateStepsParamArrayIndexing(steps []v1.Step, arrayParams map[string]int, outofBoundParams *sets.String) {
	for _, step := range steps {
		extractParamIndex(step.Script, arrayParams, outofBoundParams)
		container := step.ToK8sContainer()
		validateContainerParamArrayIndexing(container, arrayParams, outofBoundParams)
	}
}

func validateStepsTemplateParamArrayIndexing(stepTemplate *v1.StepTemplate, arrayParams map[string]int, outofBoundParams *sets.String) {
	if stepTemplate == nil {
		return
	}
	container := stepTemplate.ToK8sContainer()
	validateContainerParamArrayIndexing(container, arrayParams, outofBoundParams)
}

func validateSidecarsParamArrayIndexing(sidecars []v1.Sidecar, arrayParams map[string]int, outofBoundParams *sets.String) {
	for _, s := range sidecars {
		extractParamIndex(s.Script, arrayParams, outofBoundParams)
		container := s.ToK8sContainer()
		validateContainerParamArrayIndexing(container, arrayParams, outofBoundParams)
	}
}

func validateVolumesParamArrayIndexing(volumes []corev1.Volume, arrayParams map[string]int, outofBoundParams *sets.String) {
	for i, v := range volumes {
		extractParamIndex(v.Name, arrayParams, outofBoundParams)
		if v.VolumeSource.ConfigMap != nil {
			extractParamIndex(v.ConfigMap.Name, arrayParams, outofBoundParams)
			for _, item := range v.ConfigMap.Items {
				extractParamIndex(item.Key, arrayParams, outofBoundParams)
				extractParamIndex(item.Path, arrayParams, outofBoundParams)
			}
		}
		if v.VolumeSource.Secret != nil {
			extractParamIndex(v.Secret.SecretName, arrayParams, outofBoundParams)
			for _, item := range v.Secret.Items {
				extractParamIndex(item.Key, arrayParams, outofBoundParams)
				extractParamIndex(item.Path, arrayParams, outofBoundParams)
			}
		}
		if v.PersistentVolumeClaim != nil {
			extractParamIndex(v.PersistentVolumeClaim.ClaimName, arrayParams, outofBoundParams)
		}
		if v.Projected != nil {
			for _, s := range volumes[i].Projected.Sources {
				if s.ConfigMap != nil {
					extractParamIndex(s.ConfigMap.Name, arrayParams, outofBoundParams)
				}
				if s.Secret != nil {
					extractParamIndex(s.Secret.Name, arrayParams, outofBoundParams)
				}
				if s.ServiceAccountToken != nil {
					extractParamIndex(s.ServiceAccountToken.Audience, arrayParams, outofBoundParams)
				}
			}
		}
		if v.CSI != nil {
			if v.CSI.NodePublishSecretRef != nil {
				extractParamIndex(v.CSI.NodePublishSecretRef.Name, arrayParams, outofBoundParams)
			}
			if v.CSI.VolumeAttributes != nil {
				for _, value := range v.CSI.VolumeAttributes {
					extractParamIndex(value, arrayParams, outofBoundParams)
				}
			}
		}
	}
}

func validateContainerParamArrayIndexing(c *corev1.Container, arrayParams map[string]int, outofBoundParams *sets.String) {
	extractParamIndex(c.Name, arrayParams, outofBoundParams)
	extractParamIndex(c.Image, arrayParams, outofBoundParams)
	extractParamIndex(string(c.ImagePullPolicy), arrayParams, outofBoundParams)

	for _, a := range c.Args {
		extractParamIndex(a, arrayParams, outofBoundParams)
	}

	for ie, e := range c.Env {
		extractParamIndex(e.Value, arrayParams, outofBoundParams)
		if c.Env[ie].ValueFrom != nil {
			if e.ValueFrom.SecretKeyRef != nil {
				extractParamIndex(e.ValueFrom.SecretKeyRef.LocalObjectReference.Name, arrayParams, outofBoundParams)
				extractParamIndex(e.ValueFrom.SecretKeyRef.Key, arrayParams, outofBoundParams)
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				extractParamIndex(e.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, arrayParams, outofBoundParams)
				extractParamIndex(e.ValueFrom.ConfigMapKeyRef.Key, arrayParams, outofBoundParams)
			}
		}
	}

	for _, e := range c.EnvFrom {
		extractParamIndex(e.Prefix, arrayParams, outofBoundParams)
		if e.ConfigMapRef != nil {
			extractParamIndex(e.ConfigMapRef.LocalObjectReference.Name, arrayParams, outofBoundParams)
		}
		if e.SecretRef != nil {
			extractParamIndex(e.SecretRef.LocalObjectReference.Name, arrayParams, outofBoundParams)
		}
	}

	extractParamIndex(c.WorkingDir, arrayParams, outofBoundParams)
	for _, cc := range c.Command {
		extractParamIndex(cc, arrayParams, outofBoundParams)
	}

	for _, v := range c.VolumeMounts {
		extractParamIndex(v.Name, arrayParams, outofBoundParams)
		extractParamIndex(v.MountPath, arrayParams, outofBoundParams)
		extractParamIndex(v.SubPath, arrayParams, outofBoundParams)
	}
}
