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
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/storage"
	"github.com/tektoncd/pipeline/pkg/artifacts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func getBoundResource(resourceName string, boundResources []v1beta1.TaskResourceBinding) (*v1beta1.TaskResourceBinding, error) {
	for _, br := range boundResources {
		if br.Name == resourceName {
			return &br, nil
		}
	}
	return nil, fmt.Errorf("couldnt find resource named %q in bound resources %v", resourceName, boundResources)
}

// AddInputResource reads the inputs resources and adds the corresponding container steps
// This function reads the `paths` to check if resource copies needs to be fetched from previous tasks output(from PVC)
// 1. If resource has paths declared then serially copies the resource from previous task output paths into current resource destination.
// 2. If resource has custom destination directory using targetPath then that directory is created and resource is fetched / copied
// from  previous task
// 3. If resource has paths declared then fresh copy of resource is not fetched
func AddInputResource(
	ctx context.Context,
	kubeclient kubernetes.Interface,
	images pipeline.Images,
	taskName string,
	taskSpec *v1beta1.TaskSpec,
	taskRun *v1beta1.TaskRun,
	inputResources map[string]v1beta1.PipelineResourceInterface,
) (*v1beta1.TaskSpec, error) {
	if taskSpec == nil || taskSpec.Resources == nil || taskSpec.Resources.Inputs == nil {
		return taskSpec, nil
	}
	taskSpec = taskSpec.DeepCopy()

	pvcName := taskRun.GetPipelineRunPVCName()
	mountPVC := false
	mountSecrets := false

	prNameFromLabel := taskRun.Labels[pipeline.PipelineRunLabelKey]
	if prNameFromLabel == "" {
		prNameFromLabel = pvcName
	}
	as := artifacts.GetArtifactStorage(ctx, images, prNameFromLabel, kubeclient)

	// Iterate in reverse through the list, each element prepends but we want the first one to remain first.
	for i := len(taskSpec.Resources.Inputs) - 1; i >= 0; i-- {
		input := taskSpec.Resources.Inputs[i]
		if taskRun.Spec.Resources == nil {
			if input.Optional {
				continue
			}
			return nil, fmt.Errorf("couldnt find resource named %q, no bounded resources", input.Name)
		}
		boundResource, err := getBoundResource(input.Name, taskRun.Spec.Resources.Inputs)
		// Continue if the declared resource is optional and not specified in TaskRun
		// boundResource is nil if the declared resource in Task does not have any resource specified in the TaskRun
		if input.Optional && boundResource == nil {
			continue
		} else if err != nil {
			// throw an error for required resources, if not specified in the TaskRun
			return nil, fmt.Errorf("failed to get bound resource: %w", err)
		}
		resource, ok := inputResources[boundResource.Name]
		if !ok || resource == nil {
			return nil, fmt.Errorf("failed to Get Pipeline Resource for task %s with boundResource %v", taskName, boundResource)
		}
		var copyStepsFromPrevTasks []v1beta1.Step
		dPath := destinationPath(input.Name, input.TargetPath)
		// if taskrun is fetching resource from previous task then execute copy step instead of fetching new copy
		// to the desired destination directory, as long as the resource exports output to be copied
		if v1beta1.AllowedOutputResources[resource.GetType()] && taskRun.HasPipelineRunOwnerReference() {
			for _, path := range boundResource.Paths {
				cpSteps := as.GetCopyFromStorageToSteps(boundResource.Name, path, dPath)
				if as.GetType() == pipeline.ArtifactStoragePVCType {
					mountPVC = true
					for _, s := range cpSteps {
						s.VolumeMounts = []corev1.VolumeMount{storage.GetPvcMount(pvcName)}
						copyStepsFromPrevTasks = append(copyStepsFromPrevTasks,
							storage.CreateDirStep(images.ShellImage, boundResource.Name, dPath),
							s)
					}
				} else {
					// bucket
					copyStepsFromPrevTasks = append(copyStepsFromPrevTasks, cpSteps...)
				}
			}
		}
		// source is copied from previous task so skip fetching download container definition
		if len(copyStepsFromPrevTasks) > 0 {
			taskSpec.Steps = append(copyStepsFromPrevTasks, taskSpec.Steps...)
			mountSecrets = true
		} else {
			// Allow the resource to mutate the task.
			modifier, err := resource.GetInputTaskModifier(taskSpec, dPath)
			if err != nil {
				return nil, err
			}
			if err := v1beta1.ApplyTaskModifier(taskSpec, modifier); err != nil {
				return nil, fmt.Errorf("unabled to apply Resource %s: %w", boundResource.Name, err)
			}
		}
	}

	if mountPVC {
		taskSpec.Volumes = append(taskSpec.Volumes, GetPVCVolume(pvcName))
	}
	if mountSecrets {
		taskSpec.Volumes = appendNewSecretsVolumes(taskSpec.Volumes, as.GetSecretsVolumes()...)
	}
	return taskSpec, nil
}

const workspaceDir = "/workspace"

func destinationPath(name, path string) string {
	if path == "" {
		return filepath.Join(workspaceDir, name)
	}
	return filepath.Join(workspaceDir, path)
}
