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

// validate all references in pipelinerun exist at runtime
func validatePipelineRun(c *Reconciler, pr *v1alpha1.PipelineRun) (*v1alpha1.Pipeline, string, error) {
	var sa string
	// verify pipeline reference and all corresponding tasks exist
	p, err := c.pipelineLister.Pipelines(pr.Namespace).Get(pr.Spec.PipelineRef.Name)
	if err != nil {
		return nil, sa, fmt.Errorf("Error listing pipeline ref %s: %v", pr.Spec.PipelineRef.Name, err)
	}
	if err := validatePipelineTask(c, p.Spec.Tasks, pr.Namespace); err != nil {
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

//validatePipelineTaskAndTask validates pipelinetask inputs, params and output matches task
func validatePipelineTaskAndTask(c *Reconciler, ptask v1alpha1.PipelineTask, task *v1alpha1.Task, ns string) error {
	// stores all the input keys to validate with task input name
	inputMapping := map[string]string{}
	// stores all the output keys to validate with task output name
	outMapping := map[string]string{}
	// stores params to validate with task params
	paramsMapping := map[string]string{}

	for _, param := range ptask.Params {
		paramsMapping[param.Name] = ""
	}

	for _, source := range ptask.InputSourceBindings {
		inputMapping[source.Name] = ""
		if source.ResourceRef.Name != "" {
			rr, err := c.resourceLister.PipelineResources(ns).Get(source.ResourceRef.Name)
			if err != nil {
				return fmt.Errorf("Error listing input pipeline resource for task %s: %v ", ptask.Name, err)
			}
			inputMapping[source.Name] = string(rr.Spec.Type)
		}
	}
	for _, source := range ptask.OutputSourceBindings {
		outMapping[source.Name] = ""
		if source.ResourceRef.Name != "" {
			if _, err := c.resourceLister.PipelineResources(ns).Get(source.ResourceRef.Name); err != nil {
				return fmt.Errorf("Error listing output pipeline resource for task %s: %v ", ptask.Name, err)
			}
		}
	}

	if task.Spec.Inputs != nil {
		for _, inputResource := range task.Spec.Inputs.Resources {
			inputResourceType, ok := inputMapping[inputResource.Name]
			if !ok {
				return fmt.Errorf("Mismatch of input key %q between pipeline task %q and task %q", inputResource.Name, ptask.Name, task.Name)
			}
			// Validate the type of resource match
			if string(inputResource.Type) != inputResourceType {
				return fmt.Errorf("Mismatch of input resource type %q between pipeline task %q and task %q", inputResourceType, ptask.Name, task.Name)
			}
		}
		for _, inputResourceParam := range task.Spec.Inputs.Params {
			if _, ok := paramsMapping[inputResourceParam.Name]; !ok {
				return fmt.Errorf("Mismatch of input params %q between pipeline task %q and task %q", inputResourceParam.Name, ptask.Name, task.Name)
			}
		}
	}

	if task.Spec.Outputs != nil {
		for _, outputResource := range task.Spec.Outputs.Resources {
			if _, ok := outMapping[outputResource.Name]; !ok {
				return fmt.Errorf("Mismatch of output key %q between task %q and task ref %q", outputResource.Name, ptask.Name, task.Name)
			}
		}
	}
	return nil
}

func validatePipelineTask(c *Reconciler, tasks []v1alpha1.PipelineTask, ns string) error {
	for _, t := range tasks {
		tr, terr := c.taskLister.Tasks(ns).Get(t.TaskRef.Name)
		if terr != nil {
			return terr
		}
		if verr := validatePipelineTaskAndTask(c, t, tr, ns); verr != nil {
			return verr
		}
	}
	return nil
}
