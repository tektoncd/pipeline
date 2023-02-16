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

package resources

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidateParamTypesMatching validate that parameters in PipelineRun override corresponding parameters in Pipeline of the same type.
func ValidateParamTypesMatching(p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) error {
	// Build a map of parameter names/types declared in p.
	paramTypes := make(map[string]v1beta1.ParamType)
	for _, param := range p.Params {
		paramTypes[param.Name] = param.Type
	}

	// Build a list of parameter names from pr that have mismatching types with the map created above.
	var wrongTypeParamNames []string
	for _, param := range pr.Spec.Params {
		if paramType, ok := paramTypes[param.Name]; ok {
			if param.Value.Type != paramType {
				wrongTypeParamNames = append(wrongTypeParamNames, param.Name)
			}
		}
	}

	// Return an error with the misconfigured parameters' names, or return nil if there are none.
	if len(wrongTypeParamNames) != 0 {
		return fmt.Errorf("parameters have inconsistent types : %s", wrongTypeParamNames)
	}
	return nil
}

// ValidateRequiredParametersProvided validates that all the parameters expected by the Pipeline are provided by the PipelineRun.
// Extra Parameters are allowed, the Pipeline will use the Parameters it needs and ignore the other Parameters.
func ValidateRequiredParametersProvided(pipelineParameters *[]v1beta1.ParamSpec, pipelineRunParameters *[]v1beta1.Param) error {
	// Build a list of parameter names declared in pr.
	var providedParams []string
	for _, param := range *pipelineRunParameters {
		providedParams = append(providedParams, param.Name)
	}

	var requiredParams []string
	for _, param := range *pipelineParameters {
		if param.Default == nil { // include only parameters that don't have default values specified in the Pipeline
			requiredParams = append(requiredParams, param.Name)
		}
	}

	// Build a list of parameter names in p that are missing from pr.
	missingParams := list.DiffLeft(requiredParams, providedParams)

	// Return an error with the missing parameters' names, or return nil if there are none.
	if len(missingParams) != 0 {
		return fmt.Errorf("PipelineRun missing parameters: %s", missingParams)
	}
	return nil
}

// ValidateObjectParamRequiredKeys validates that the required keys of all the object parameters expected by the Pipeline are provided by the PipelineRun.
func ValidateObjectParamRequiredKeys(pipelineParameters []v1beta1.ParamSpec, pipelineRunParameters []v1beta1.Param) error {
	missings := taskrun.MissingKeysObjectParamNames(pipelineParameters, pipelineRunParameters)
	if len(missings) != 0 {
		return fmt.Errorf("PipelineRun missing object keys for parameters: %v", missings)
	}

	return nil
}

// ValidateParamArrayIndex validate if the array indexing param reference target is existent
func ValidateParamArrayIndex(ctx context.Context, p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) error {
	arrayParams := extractParamIndexes(p.Params, pr.Spec.Params)

	outofBoundParams := sets.String{}
	arrayIndexingParams := []string{}

	// collect all the references
	for i := range p.Tasks {
		extractArrayIndexingReferencesFromParams(ctx, p.Tasks[i].Params, arrayParams, &arrayIndexingParams)
		if p.Tasks[i].IsMatrixed() {
			extractArrayIndexingReferencesFromParams(ctx, p.Tasks[i].Matrix.Params, arrayParams, &arrayIndexingParams)
		}
		for j := range p.Tasks[i].Workspaces {
			taskrun.ExtractArrayIndexingParamReference(ctx, p.Tasks[i].Workspaces[j].SubPath, arrayParams, &arrayIndexingParams)
		}
		for _, wes := range p.Tasks[i].WhenExpressions {
			taskrun.ExtractArrayIndexingParamReference(ctx, wes.Input, arrayParams, &arrayIndexingParams)
			for _, v := range wes.Values {
				taskrun.ExtractArrayIndexingParamReference(ctx, v, arrayParams, &arrayIndexingParams)
			}
		}
	}

	for i := range p.Finally {
		extractArrayIndexingReferencesFromParams(ctx, p.Finally[i].Params, arrayParams, &arrayIndexingParams)
		if p.Finally[i].IsMatrixed() {
			extractArrayIndexingReferencesFromParams(ctx, p.Finally[i].Matrix.Params, arrayParams, &arrayIndexingParams)
		}
		for _, wes := range p.Finally[i].WhenExpressions {
			for _, v := range wes.Values {
				taskrun.ExtractArrayIndexingParamReference(ctx, v, arrayParams, &arrayIndexingParams)
			}
		}
	}

	if len(arrayIndexingParams) > 0 && !config.CheckAlphaOrBetaAPIFields(ctx) {
		return fmt.Errorf(`indexing into array params: %v require "enable-api-fields" feature gate to be "alpha" or "beta"`, arrayIndexingParams)
	}

	for _, val := range arrayIndexingParams {
		indexString := substitution.ExtractIndexString(val)
		idx, _ := substitution.ExtractIndex(indexString)
		v := substitution.TrimArrayIndex(val)
		if paramLength, ok := arrayParams[v]; ok {
			if idx >= paramLength {
				outofBoundParams.Insert(val)
			}
		}
	}

	if outofBoundParams.Len() > 0 {
		return fmt.Errorf("non-existent param references:%v", outofBoundParams.List())
	}
	return nil
}

func extractParamIndexes(defaults []v1beta1.ParamSpec, params []v1beta1.Param) map[string]int {
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
			if p.Default.Type == v1beta1.ParamTypeArray {
				for _, pattern := range patterns {
					for i := 0; i < len(p.Default.ArrayVal); i++ {
						arrayParams[fmt.Sprintf(pattern, p.Name)] = len(p.Default.ArrayVal)
					}
				}
			}
		}
	}

	// Collect array params lengths from pipelinerun or taskrun
	for _, p := range params {
		if p.Value.Type == v1beta1.ParamTypeArray {
			for _, pattern := range patterns {
				for i := 0; i < len(p.Value.ArrayVal); i++ {
					arrayParams[fmt.Sprintf(pattern, p.Name)] = len(p.Value.ArrayVal)
				}
			}
		}
	}
	return arrayParams
}

// extractArrayIndexingReferencesFromParams validates all the params' array indexing references and check if they are valid
func extractArrayIndexingReferencesFromParams(ctx context.Context, params []v1beta1.Param, arrayParams map[string]int, arrayIndexingParams *[]string) {
	for i := range params {
		taskrun.ExtractArrayIndexingParamReference(ctx, params[i].Value.StringVal, arrayParams, arrayIndexingParams)
		for _, v := range params[i].Value.ArrayVal {
			taskrun.ExtractArrayIndexingParamReference(ctx, v, arrayParams, arrayIndexingParams)
		}
		for _, v := range params[i].Value.ObjectVal {
			taskrun.ExtractArrayIndexingParamReference(ctx, v, arrayParams, arrayIndexingParams)
		}
	}
}
