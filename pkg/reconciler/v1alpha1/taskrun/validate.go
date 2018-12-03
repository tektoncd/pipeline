/*
Copyright 2018 The Knative Authors

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
	"fmt"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
)

// ValidateTaskRunAndTask validates task inputs, params and output matches taskrun
func ValidateTaskRunAndTask(params []v1alpha1.Param, rtr *resources.ResolvedTaskResources) error {
	// stores params to validate with task params
	paramsMapping := map[string]string{}

	for _, param := range params {
		paramsMapping[param.Name] = ""
	}

	if rtr.TaskSpec != nil {
		if rtr.TaskSpec.Inputs != nil {
			for _, inputResource := range rtr.TaskSpec.Inputs.Resources {
				r, ok := rtr.Inputs[inputResource.Name]
				if !ok {
					return fmt.Errorf("input resource %q not provided for task %q", inputResource.Name, rtr.TaskName)
				}
				// Validate the type of resource match
				if inputResource.Type != r.Spec.Type {
					return fmt.Errorf("input resource %q for task %q should be type %q but was %q", inputResource.Name, rtr.TaskName, r.Spec.Type, inputResource.Type)
				}
			}
			for _, inputResourceParam := range rtr.TaskSpec.Inputs.Params {
				if _, ok := paramsMapping[inputResourceParam.Name]; !ok {
					if inputResourceParam.Default == "" {
						return fmt.Errorf("input param %q not provided for task %q", inputResourceParam.Name, rtr.TaskName)
					}
				}
			}
		}

		if rtr.TaskSpec.Outputs != nil {
			for _, outputResource := range rtr.TaskSpec.Outputs.Resources {
				r, ok := rtr.Outputs[outputResource.Name]
				if !ok {
					return fmt.Errorf("output resource %q not provided for task %q", outputResource.Name, rtr.TaskName)
				}
				// Validate the type of resource match
				if outputResource.Type != r.Spec.Type {
					return fmt.Errorf("output resource %q for task %q should be type %q but was %q", outputResource.Name, rtr.TaskName, r.Spec.Type, outputResource.Type)
				}
			}
		}
	}
	return nil
}
