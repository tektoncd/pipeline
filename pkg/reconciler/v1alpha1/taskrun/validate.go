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
)

// validate all references in taskrun exist at runtime
func validateTaskRun(c *Reconciler, tr *v1alpha1.TaskRun) error {
	// verify task reference exists, all params are provided
	// and all inputs/outputs are bound
	t, err := c.taskLister.Tasks(tr.Namespace).Get(tr.Spec.TaskRef.Name)
	if err != nil {
		return fmt.Errorf("Error listing task ref %s: %v",
			tr.Spec.TaskRef.Name, err)
	}
	return validateTaskRunAndTask(c, *tr, t, tr.Namespace)
}

//validateTaskRunTask validates task inputs, params and output matches taskrun
func validateTaskRunAndTask(c *Reconciler, tr v1alpha1.TaskRun, task *v1alpha1.Task, ns string) error {
	// stores all the input keys to validate with task input name
	inputMapping := map[string]string{}
	// stores all the output keys to validate with task output name
	outMapping := map[string]string{}
	// stores params to validate with task params
	paramsMapping := map[string]string{}

	for _, param := range tr.Spec.Inputs.Params {
		paramsMapping[param.Name] = ""
	}

	for _, source := range tr.Spec.Inputs.Resources {
		inputMapping[source.Name] = ""
		if source.ResourceRef.Name != "" {
			rr, err := c.resourceLister.PipelineResources(ns).Get(
				source.ResourceRef.Name)
			if err != nil {
				return fmt.Errorf("Error listing input task resource "+
					"for task %s: %v ", tr.Name, err)
			}
			inputMapping[source.Name] = string(rr.Spec.Type)
		}
	}
	for _, source := range tr.Spec.Outputs.Resources {
		outMapping[source.Name] = ""
		if source.ResourceRef.Name != "" {
			rr, err := c.resourceLister.PipelineResources(ns).Get(
				source.ResourceRef.Name)
			if err != nil {
				return fmt.Errorf("Error listing output task resource "+
					"for task %s: %v ", tr.Name, err)
			}
			outMapping[source.Name] = string(rr.Spec.Type)

		}
	}

	if task.Spec.Inputs != nil {
		for _, inputResource := range task.Spec.Inputs.Resources {
			inputResourceType, ok := inputMapping[inputResource.Name]
			if !ok {
				return fmt.Errorf("Mismatch of input key %q between "+
					"task %q and task %q", inputResource.Name,
					tr.Name, task.Name)
			}
			// Validate the type of resource match
			if string(inputResource.Type) != inputResourceType {
				return fmt.Errorf("Mismatch of input resource type %q "+
					"between task %q and task %q", inputResourceType,
					tr.Name, task.Name)
			}
		}
		for _, inputResourceParam := range task.Spec.Inputs.Params {
			if _, ok := paramsMapping[inputResourceParam.Name]; !ok {
				if inputResourceParam.Default == "" {
					return fmt.Errorf("Mismatch of input params %q between "+
						"task %q and task %q", inputResourceParam.Name, tr.Name,
						task.Name)
				}
			}
		}
	}

	if task.Spec.Outputs != nil {
		for _, outputResource := range task.Spec.Outputs.Resources {
			outputResourceType, ok := outMapping[outputResource.Name]
			if !ok {
				return fmt.Errorf("Mismatch of output key %q between "+
					"task %q and task %q", outputResource.Name,
					tr.Name, task.Name)
			}
			// Validate the type of resource match
			if string(outputResource.Type) != outputResourceType {
				return fmt.Errorf("Mismatch of output resource type %q "+
					"between task %q and task %q", outputResourceType,
					tr.Name, task.Name)
			}
		}
	}
	return nil
}
