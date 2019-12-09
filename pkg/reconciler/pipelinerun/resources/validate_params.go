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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

// Validate that parameters in PipelineRun override corresponding parameters in Pipeline of the same type.
func ValidateParamTypesMatching(p *v1alpha1.PipelineSpec, pr *v1alpha1.PipelineRun) error {
	// Build a map of parameter names/types declared in p.
	paramTypes := make(map[string]v1alpha1.ParamType)
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
