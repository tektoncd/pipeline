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
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	trresources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
)

// ValidateParamTypesMatching validate that parameters in PipelineRun override corresponding parameters in Pipeline of the same type.
func ValidateParamTypesMatching(p *v1.PipelineSpec, pr *v1.PipelineRun) error {
	// Build a map of parameter names/types declared in p.
	paramTypes := make(map[string]v1.ParamType)
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
func ValidateRequiredParametersProvided(pipelineParameters *v1.ParamSpecs, pipelineRunParameters *v1.Params) error {
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
func ValidateObjectParamRequiredKeys(pipelineParameters []v1.ParamSpec, pipelineRunParameters []v1.Param) error {
	missings := taskrun.MissingKeysObjectParamNames(pipelineParameters, pipelineRunParameters)
	if len(missings) != 0 {
		return fmt.Errorf("PipelineRun missing object keys for parameters: %v", missings)
	}

	return nil
}

// ValidateParameterTypesInMatrix validates the type of Parameter for Matrix.Params
// and Matrix.Include.Params after any replacements are made from Task parameters or results
// Matrix.Params must be of type array. Matrix.Include.Params must be of type string.
func ValidateParameterTypesInMatrix(state PipelineRunState) error {
	for _, rpt := range state {
		m := rpt.PipelineTask.Matrix
		if m.HasInclude() {
			for _, include := range m.Include {
				for _, param := range include.Params {
					if param.Value.Type != v1.ParamTypeString {
						return fmt.Errorf("parameters of type string only are allowed, but param %s has type %s", param.Name, string(param.Value.Type))
					}
				}
			}
		}
		if m.HasParams() {
			for _, param := range m.Params {
				if param.Value.Type != v1.ParamTypeArray {
					return fmt.Errorf("parameters of type array only are allowed, but param %s has type %s", param.Name, string(param.Value.Type))
				}
			}
		}
	}
	return nil
}

// ValidateParamArrayIndex validates if the param reference to an array param is out of bound.
// error is returned when the array indexing reference is out of bound of the array param
// e.g. if a param reference of $(params.array-param[2]) and the array param is of length 2.
func ValidateParamArrayIndex(ps *v1.PipelineSpec, params v1.Params) error {
	return trresources.ValidateOutOfBoundArrayParams(ps.Params, params, ps.GetIndexingReferencesToArrayParams())
}
