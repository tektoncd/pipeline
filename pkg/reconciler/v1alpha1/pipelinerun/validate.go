/*
Copyright 2018 The Knative Authors

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

package pipelinerun

import (
	"fmt"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

// validate all ref exist
func (c *Reconciler) validate(pr *v1alpha1.PipelineRun) (*v1alpha1.Pipeline, string, error) {
	var sa string
	// verify pipeline reference and all corresponding tasks exist
	p, err := c.pipelineLister.Pipelines(pr.Namespace).Get(pr.Spec.PipelineRef.Name)
	if err != nil {
		return nil, sa, fmt.Errorf("Error listing pipeline ref %s: %v", pr.Spec.PipelineRef.Name, err)
	}
	if err := c.validatePipelineTask(p.Spec.Tasks, pr.Namespace); err != nil {
		return nil, sa, fmt.Errorf("Error validating tasks in pipeline %s: %v", p.Name, err)
	}

	if pr.Spec.PipelineParamsRef.Name != "" {
		pp, err := c.pipelineParamsLister.PipelineParamses(pr.Namespace).Get(pr.Spec.PipelineParamsRef.Name)
		if err != nil {
			return nil, "", fmt.Errorf("Error listing pipelineparams %s for pipelinerun %s: %v", pr.Spec.PipelineParamsRef.Name, pr.Name, err)
		}
		sa = pp.Spec.ServiceAccount
	}
	return p, sa, nil
}

//validateTasks that taskref in task matches the input and output bindings key
func validateTaskAndTaskRef(c *Reconciler, task v1alpha1.PipelineTask, taskref *v1alpha1.Task, ns string) error {
	// stores all the input keys to validate with taskreference input name
	inputMapping := map[string]string{}
	// stores all the output keys to validate with taskreference output name
	outMapping := map[string]string{}

	for _, source := range task.InputSourceBindings {
		inputMapping[source.Key] = ""
		if source.ResourceRef.Name != "" {
			if _, err := c.resourceLister.PipelineResources(ns).Get(source.ResourceRef.Name); err != nil {
				return fmt.Errorf("Error listing input pipeline resource for task %s: %v ", task.Name, err)
			}
		}
	}
	for _, source := range task.OutputSourceBindings {
		outMapping[source.Key] = ""
		if source.ResourceRef.Name != "" {
			if _, err := c.resourceLister.PipelineResources(ns).Get(source.ResourceRef.Name); err != nil {
				return fmt.Errorf("Error listing output pipeline resource for task %s: %v ", task.Name, err)
			}
		}
	}

	if taskref.Spec.Inputs != nil {
		for _, inputResource := range taskref.Spec.Inputs.Resources {
			if _, ok := inputMapping[inputResource.Name]; !ok {
				return fmt.Errorf("Mismatch of input key %s between task %s and task ref %s", inputResource.Name, task.Name, taskref.Name)
			}
		}
	}
	if taskref.Spec.Outputs != nil {
		for _, outputResource := range taskref.Spec.Outputs.Resources {
			if _, ok := outMapping[outputResource.Name]; !ok {
				return fmt.Errorf("Mismatch of output key %s between task %s and task ref %s", outputResource.Name, task.Name, taskref.Name)
			}
		}
	}
	return nil
}

func (c *Reconciler) validatePipelineTask(tasks []v1alpha1.PipelineTask, ns string) error {
	for _, t := range tasks {
		tr, terr := c.taskLister.Tasks(ns).Get(t.TaskRef.Name)
		if terr != nil {
			return terr
		}
		if verr := validateTaskAndTaskRef(c, t, tr, ns); verr != nil {
			return verr
		}
	}
	return nil
}
