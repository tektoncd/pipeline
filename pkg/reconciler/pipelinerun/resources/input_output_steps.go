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
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

// GetOutputSteps will add the correct `path` to the output resources for pt
func GetOutputSteps(outputs map[string]*v1alpha1.PipelineResource, taskName, storageBasePath string) []v1alpha1.TaskResourceBinding {
	var taskOutputResources []v1alpha1.TaskResourceBinding

	for name, outputResource := range outputs {
		taskOutputResource := v1alpha1.TaskResourceBinding{
			PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
				Name: name,
			},
			Paths: []string{filepath.Join(storageBasePath, taskName, name)},
		}
		// SelfLink is being checked there to determine if this PipelineResource is an instance that
		// exists in the cluster (in which case Kubernetes will populate this field) or is specified by Spec
		if outputResource.SelfLink != "" {
			taskOutputResource.ResourceRef = &v1alpha1.PipelineResourceRef{
				Name:       outputResource.Name,
				APIVersion: outputResource.APIVersion,
			}
		} else if outputResource.Spec.Type != "" {
			taskOutputResource.ResourceSpec = &v1alpha1.PipelineResourceSpec{
				Type:         outputResource.Spec.Type,
				Params:       outputResource.Spec.Params,
				SecretParams: outputResource.Spec.SecretParams,
			}
		}
		taskOutputResources = append(taskOutputResources, taskOutputResource)
	}
	return taskOutputResources
}

// GetInputSteps will add the correct `path` to the input resources for pt. If the resources are provided by
// a previous task, the correct `path` will be used so that the resource provided by that task will be used.
func GetInputSteps(inputs map[string]*v1alpha1.PipelineResource, inputResources []v1alpha1.PipelineTaskInputResource, storageBasePath string) []v1alpha1.TaskResourceBinding {
	var taskInputResources []v1alpha1.TaskResourceBinding

	for name, inputResource := range inputs {
		taskInputResource := v1alpha1.TaskResourceBinding{
			PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
				Name: name,
			},
		}
		// SelfLink is being checked there to determine if this PipelineResource is an instance that
		// exists in the cluster (in which case Kubernetes will populate this field) or is specified by Spec
		if inputResource.SelfLink != "" {
			taskInputResource.ResourceRef = &v1alpha1.PipelineResourceRef{
				Name:       inputResource.Name,
				APIVersion: inputResource.APIVersion,
			}
		} else if inputResource.Spec.Type != "" {
			taskInputResource.ResourceSpec = &v1alpha1.PipelineResourceSpec{
				Type:         inputResource.Spec.Type,
				Params:       inputResource.Spec.Params,
				SecretParams: inputResource.Spec.SecretParams,
			}
		}

		// Determine if the value is meant to come `from` a previous Task - if so, add the path to the pvc
		// that contains the data as the `path` the resulting TaskRun should get the data from.
		var stepSourceNames []string
		for _, pipelineTaskInput := range inputResources {
			if pipelineTaskInput.Name == name {
				for _, constr := range pipelineTaskInput.From {
					stepSourceNames = append(stepSourceNames, filepath.Join(storageBasePath, constr, name))
				}
			}
		}
		if len(stepSourceNames) > 0 {
			taskInputResource.Paths = append(taskInputResource.Paths, stepSourceNames...)
		}
		taskInputResources = append(taskInputResources, taskInputResource)
	}
	return taskInputResources
}

// WrapSteps will add the correct `paths` to all of the inputs and outputs for pt
func WrapSteps(tr *v1alpha1.TaskRunSpec, pt *v1alpha1.PipelineTask, inputs, outputs map[string]*v1alpha1.PipelineResource, storageBasePath string) {
	if pt == nil {
		return
	}
	if pt.Resources != nil {
		// Add presteps to setup updated input
		tr.Inputs.Resources = append(tr.Inputs.Resources, GetInputSteps(inputs, pt.Resources.Inputs, storageBasePath)...)
	}

	// Add poststeps to setup outputs
	tr.Outputs.Resources = append(tr.Outputs.Resources, GetOutputSteps(outputs, pt.Name, storageBasePath)...)
}
