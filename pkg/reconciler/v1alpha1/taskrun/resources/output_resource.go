/*
Copyright 2019 The Tekton Authors.

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
	"github.com/tektoncd/pipeline/pkg/artifacts"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	outputDir = "/workspace/output/"

	// allowedOutputResource checks if an output resource type produces
	// an output that should be copied to the PVC
	allowedOutputResources = map[v1alpha1.PipelineResourceType]bool{
		v1alpha1.PipelineResourceTypeStorage: true,
		v1alpha1.PipelineResourceTypeGit:     true,
	}
)

// AddOutputResources reads the output resources and adds the corresponding container steps
// This function also reads the inputs to check if resources are redeclared in inputs and has any custom
// target directory.
// Steps executed:
//  1. If taskrun has owner reference as pipelinerun then all outputs are copied to parents PVC
// and also runs any custom upload steps (upload to blob store)
//  2.  If taskrun does not have pipelinerun as owner reference then all outputs resources execute their custom
// upload steps (like upload to blob store )
//
// Resource source path determined
// 1. If resource is declared in inputs then target path from input resource is used to identify source path
// 2. If resource is declared in outputs only then the default is /output/resource_name
func AddOutputResources(
	kubeclient kubernetes.Interface,
	taskName string,
	taskSpec *v1alpha1.TaskSpec,
	taskRun *v1alpha1.TaskRun,
	outputResources map[string]v1alpha1.PipelineResourceInterface,
	logger *zap.SugaredLogger,
) (*v1alpha1.TaskSpec, error) {

	if taskSpec == nil || taskSpec.Outputs == nil {
		return taskSpec, nil
	}

	taskSpec = taskSpec.DeepCopy()

	pvcName := taskRun.GetPipelineRunPVCName()
	as, err := artifacts.GetArtifactStorage(pvcName, kubeclient, logger)
	if err != nil {
		return nil, err
	}

	// track resources that are present in input of task cuz these resources will be copied onto PVC
	inputResourceMap := map[string]string{}

	if taskSpec.Inputs != nil {
		for _, input := range taskSpec.Inputs.Resources {
			inputResourceMap[input.Name] = destinationPath(input.Name, input.TargetPath)
		}
	}

	for _, output := range taskSpec.Outputs.Resources {
		boundResource, err := getBoundResource(output.Name, taskRun.Spec.Outputs.Resources)
		if err != nil {
			return nil, xerrors.Errorf("failed to get bound resource: %w", err)
		}

		resource, ok := outputResources[boundResource.Name]
		if !ok || resource == nil {
			return nil, xerrors.Errorf("failed to get output pipeline Resource for task %q resource %v", taskName, boundResource)
		}
		var (
			resourceContainers []corev1.Container
			resourceVolumes    []corev1.Volume
		)
		// if resource is declared in input then copy outputs to pvc
		// To build copy step it needs source path(which is targetpath of input resourcemap) from task input source
		sourcePath := inputResourceMap[boundResource.Name]
		if sourcePath != "" {
			logger.Warn(`This task uses the same resource as an input and output. The behavior of this will change in a future release.
		See https://github.com/tektoncd/pipeline/issues/1118 for more information.`)
		} else {
			if output.TargetPath == "" {
				sourcePath = filepath.Join(outputDir, boundResource.Name)
			} else {
				sourcePath = output.TargetPath
			}
		}
		resource.SetDestinationDirectory(sourcePath)

		resourceContainers, err = resource.GetUploadContainerSpec()
		if err != nil {
			return nil, xerrors.Errorf("task %q invalid upload spec: %q; error %w", taskName, boundResource.ResourceRef.Name, err)
		}

		resourceVolumes, err = resource.GetUploadVolumeSpec(taskSpec)
		if err != nil {
			return nil, xerrors.Errorf("task %q invalid upload spec: %q; error %w", taskName, boundResource.ResourceRef.Name, err)
		}

		if allowedOutputResources[resource.GetType()] && taskRun.HasPipelineRunOwnerReference() {
			var newSteps []corev1.Container
			for _, dPath := range boundResource.Paths {
				containers := as.GetCopyToStorageFromContainerSpec(resource.GetName(), sourcePath, dPath)
				newSteps = append(newSteps, containers...)
			}
			resourceContainers = append(resourceContainers, newSteps...)
			resourceVolumes = append(resourceVolumes, as.GetSecretsVolumes()...)
		}

		taskSpec.Steps = append(taskSpec.Steps, resourceContainers...)
		taskSpec.Volumes = append(taskSpec.Volumes, resourceVolumes...)

		if as.GetType() == v1alpha1.ArtifactStoragePVCType {
			if pvcName == "" {
				return taskSpec, nil
			}

			// attach pvc volume only if it is not already attached
			for _, buildVol := range taskSpec.Volumes {
				if buildVol.Name == pvcName {
					return taskSpec, nil
				}
			}
			taskSpec.Volumes = append(taskSpec.Volumes, GetPVCVolume(pvcName))
		}
	}
	return taskSpec, nil
}
