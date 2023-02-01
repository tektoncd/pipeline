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
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/client-go/kubernetes"
)

var (
	outputDir = "/workspace/output/"
)

func getBoundResource(resourceName string, boundResources []v1beta1.TaskResourceBinding) (*v1beta1.TaskResourceBinding, error) {
	for _, br := range boundResources {
		if br.Name == resourceName {
			return &br, nil
		}
	}
	return nil, fmt.Errorf("couldnt find resource named %q in bound resources %v", resourceName, boundResources)
}

// AddOutputResources reads the output resources and adds the corresponding container steps
// This function also reads the inputs to check if resources are redeclared in inputs and has any custom
// target directory.
// Steps executed:
//  1. If taskrun has owner reference as pipelinerun then all outputs are copied to parents PVC
//
// and also runs any custom upload steps (upload to blob store)
//  2. If taskrun does not have pipelinerun as owner reference then all outputs resources execute their custom
//
// upload steps (like upload to blob store )
//
// Resource source path determined
// 1. If resource has a targetpath that is used. Otherwise:
// 2. If resource is declared in outputs only then the default is /output/resource_name
func AddOutputResources(
	ctx context.Context,
	kubeclient kubernetes.Interface,
	images pipeline.Images,
	taskName string,
	taskSpec *v1beta1.TaskSpec,
	taskRun *v1beta1.TaskRun,
	outputResources map[string]v1beta1.PipelineResourceInterface,
) (*v1beta1.TaskSpec, error) {
	if taskSpec == nil || taskSpec.Resources == nil || taskSpec.Resources.Outputs == nil {
		return taskSpec, nil
	}

	taskSpec = taskSpec.DeepCopy()
	for _, output := range taskSpec.Resources.Outputs {
		if taskRun.Spec.Resources == nil {
			if output.Optional {
				continue
			}
			return nil, fmt.Errorf("couldnt find resource named %q, no bounded resources", output.Name)
		}
		boundResource, err := getBoundResource(output.Name, taskRun.Spec.Resources.Outputs)
		// Continue if the declared resource is optional and not specified in TaskRun
		// boundResource is nil if the declared resource in Task does not have any resource specified in the TaskRun
		if output.Optional && boundResource == nil {
			continue
		} else if err != nil {
			// throw an error for required resources, if not specified in the TaskRun
			return nil, fmt.Errorf("failed to get bound resource: %w", err)
		}
		resource, ok := outputResources[boundResource.Name]
		if !ok || resource == nil {
			return nil, fmt.Errorf("failed to get output pipeline Resource for task %q resource %v", taskName, boundResource)
		}

		var sourcePath string
		if output.TargetPath == "" {
			sourcePath = filepath.Join(outputDir, boundResource.Name)
		} else {
			sourcePath = output.TargetPath
		}

		if v1beta1.AllowedOutputResources[resource.GetType()] && taskRun.HasPipelineRunOwnerReference() {
			var newSteps []v1beta1.Step
			taskSpec.Steps = append(taskSpec.Steps, newSteps...)
		}

		// Allow the resource to mutate the task.
		modifier, err := resource.GetOutputTaskModifier(taskSpec, sourcePath)
		if err != nil {
			return nil, err
		}
		if err := v1beta1.ApplyTaskModifier(taskSpec, modifier); err != nil {
			return nil, fmt.Errorf("Unabled to apply Resource %s: %w", boundResource.Name, err)
		}
	}
	return taskSpec, nil
}
