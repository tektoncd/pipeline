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

// GetPipeline is used to retrieve existing instance of Pipeline called name
type GetPipeline func(name string) (*v1alpha1.Pipeline, error)

// GetResource is a function used to retrieve PipelineResources.
type GetResource func(string) (*v1alpha1.PipelineResource, error)

// GetTask is a function that will retrieve the task called name.
type GetTask func(name string) (*v1alpha1.Task, error)

// ValidatePipelineRun validates all references in pipelinerun exist at runtime
func ValidatePipelineRun(pr *v1alpha1.PipelineRun, getPipeline GetPipeline, getTask GetTask, getResource GetResource) (*v1alpha1.Pipeline, string, error) {
	sa := pr.Spec.ServiceAccount
	// verify pipeline reference and all corresponding tasks exist
	p, err := getPipeline(pr.Spec.PipelineRef.Name)
	if err != nil {
		return nil, sa, fmt.Errorf("error listing pipeline ref %s: %v", pr.Spec.PipelineRef.Name, err)
	}
	if err := validatePipelineTask(getTask, getResource, p.Spec.Tasks, pr.Spec.PipelineTaskResources, pr.Namespace); err != nil {
		return nil, sa, fmt.Errorf("error validating tasks in pipeline %s: %v", p.Name, err)
	}
	return p, sa, nil
}

func validatePipelineTaskAndTask(getResource GetResource, ptask v1alpha1.PipelineTask, task *v1alpha1.Task, inputs []v1alpha1.TaskResourceBinding, outputs []v1alpha1.TaskResourceBinding, ns string) error {
	// a map from the name of the input resource to the type
	inputMapping := map[string]v1alpha1.PipelineResourceType{}
	// a map from the name of the output resource to the type
	outputMapping := map[string]v1alpha1.PipelineResourceType{}
	// a map that can be used to lookup if params have been specified
	paramsMapping := map[string]string{}

	for _, param := range ptask.Params {
		paramsMapping[param.Name] = ""
	}

	for _, r := range inputs {
		inputMapping[r.Name] = ""
		// TODO(#213): if this is the empty string should it be an error? or maybe let the lookup fail?
		if r.ResourceRef.Name != "" {
			rr, err := getResource(r.ResourceRef.Name)
			if err != nil {
				return fmt.Errorf("error listing input pipeline resource for task %s: %v ", ptask.Name, err)
			}
			inputMapping[r.Name] = rr.Spec.Type
		}
	}

	for _, r := range outputs {
		outputMapping[r.Name] = ""
		// TODO(#213): if this is the empty string should it be an error? or maybe let the lookup fail?
		if r.ResourceRef.Name != "" {
			rr, err := getResource(r.ResourceRef.Name)
			if err != nil {
				return fmt.Errorf("error listing output pipeline resource for task %s: %v ", ptask.Name, err)
			}
			outputMapping[r.Name] = rr.Spec.Type
		}
	}
	if task.Spec.Inputs != nil {
		for _, inputResource := range task.Spec.Inputs.Resources {
			inputResourceType, ok := inputMapping[inputResource.Name]
			if !ok {
				return fmt.Errorf("input resource %q not provided for pipeline task %q (task %q)", inputResource.Name, ptask.Name, task.Name)
			}
			// Validate the type of resource match
			if inputResource.Type != inputResourceType {
				return fmt.Errorf("input resource %q for pipeline task %q (task %q) should be type %q but was %q", inputResource.Name, ptask.Name, task.Name, inputResourceType, inputResource.Type)
			}
		}
		for _, inputResourceParam := range task.Spec.Inputs.Params {
			// TODO(#213): should check if the param has default values here
			if _, ok := paramsMapping[inputResourceParam.Name]; !ok {
				return fmt.Errorf("input param %q not provided for pipeline task %q (task %q)", inputResourceParam.Name, ptask.Name, task.Name)
			}
		}
	}

	if task.Spec.Outputs != nil {
		for _, outputResource := range task.Spec.Outputs.Resources {
			outputResourceType, ok := outputMapping[outputResource.Name]
			if !ok {
				return fmt.Errorf("output resource %q not provided for pipeline task %q (task %q)", outputResource.Name, ptask.Name, task.Name)
			}
			if outputResource.Type != outputResourceType {
				return fmt.Errorf("output resource %q for pipeline task %q (task %q) should be type %q but was %q", outputResource.Name, ptask.Name, task.Name, outputResourceType, outputResource.Type)
			}
		}
	}
	return nil
}

func validatePipelineTask(getTask GetTask, getResource GetResource, tasks []v1alpha1.PipelineTask, resources []v1alpha1.PipelineTaskResource, ns string) error {
	for _, pt := range tasks {
		t, err := getTask(pt.TaskRef.Name)
		if err != nil {
			return fmt.Errorf("couldn't get task %q: %s", pt.TaskRef.Name, err)
		}

		found := false
		for _, tr := range resources {
			if tr.Name == pt.Name {
				if err := validatePipelineTaskAndTask(getResource, pt, t, tr.Inputs, tr.Outputs, ns); err != nil {
					return fmt.Errorf("pipelineTask %q is invalid: %s", pt.Name, err)
				}
				found = true
			}
		}

		if !found &&
			((t.Spec.Inputs != nil && len(t.Spec.Inputs.Resources) > 0) ||
				(t.Spec.Outputs != nil && len(t.Spec.Outputs.Resources) > 0)) {
			return fmt.Errorf("resources required for pipeline task %q (task %q) but were missing", pt.Name, t.Name)
		}
	}
	return nil
}
