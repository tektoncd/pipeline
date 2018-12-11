/*
Copyright 2018 The Knative Authors.

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
	"path/filepath"
	"strings"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

var (
	outputDir = "/workspace/output/"
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
	b *buildv1alpha1.Build,
	taskName string,
	taskSpec *v1alpha1.TaskSpec,
	taskRun *v1alpha1.TaskRun,
	pipelineResourceLister listers.PipelineResourceLister,
	logger *zap.SugaredLogger,
) error {

	if taskSpec == nil || taskSpec.Outputs == nil {
		return nil
	}

	pipelineRunpvcName := taskRun.GetPipelineRunPVCName()

	// track resources that are present in input of task cuz these resources will be copied onto PVC
	inputResourceMap := map[string]string{}

	if taskSpec.Inputs != nil {
		for _, input := range taskSpec.Inputs.Resources {
			var targetPath = workspaceDir
			if input.TargetPath != "" {
				targetPath = filepath.Join(workspaceDir, input.TargetPath)
			}
			inputResourceMap[input.Name] = targetPath
		}
	}

	for _, output := range taskSpec.Outputs.Resources {
		boundResource, err := getBoundResource(output.Name, taskRun.Spec.Outputs.Resources)
		if err != nil {
			return fmt.Errorf("Failed to get bound resource: %s", err)
		}

		resource, err := pipelineResourceLister.PipelineResources(taskRun.Namespace).Get(boundResource.ResourceRef.Name)
		if err != nil {
			return fmt.Errorf("Failed to get output pipeline Resource for task %q: %q", boundResource.ResourceRef.Name, taskName)
		}

		// if resource is declared in input then copy outputs to pvc
		// To build copy step it needs source path(which is targetpath of input resourcemap) from task input source
		sourcePath := inputResourceMap[boundResource.Name]
		if sourcePath == "" {
			sourcePath = filepath.Join(outputDir, boundResource.Name)
		}
		switch resource.Spec.Type {
		case v1alpha1.PipelineResourceTypeStorage:
			storageResource, err := v1alpha1.NewStorageResource(resource)
			if err != nil {
				return fmt.Errorf("task %q invalid git Pipeline Resource: %q",
					taskName,
					boundResource.ResourceRef.Name,
				)
			}
			err = addStoreUploadStep(b, storageResource, sourcePath)
			if err != nil {
				return fmt.Errorf("task %q invalid Pipeline Resource: %q; invalid upload steps err: %v",
					taskName, boundResource.ResourceRef.Name, err)
			}
		}

		// copy to pvc if pvc is present
		if pipelineRunpvcName != "" {
			var newSteps []corev1.Container
			for _, dPath := range boundResource.Paths {
				newSteps = append(newSteps, []corev1.Container{{
					Name:  fmt.Sprintf("source-mkdir-%s", resource.GetName()),
					Image: *bashNoopImage,
					Args: []string{
						"-args", strings.Join([]string{"mkdir", "-p", dPath}, " "),
					},
					VolumeMounts: []corev1.VolumeMount{getPvcMount(pipelineRunpvcName)},
				}, {
					Name:  fmt.Sprintf("source-copy-%s", resource.GetName()),
					Image: *bashNoopImage,
					Args: []string{
						"-args", strings.Join([]string{"cp", "-r", fmt.Sprintf("%s/.", sourcePath), dPath}, " "),
					},
					VolumeMounts: []corev1.VolumeMount{getPvcMount(pipelineRunpvcName)},
				}}...)
			}
			b.Spec.Steps = append(b.Spec.Steps, newSteps...)
		}
	}

	if pipelineRunpvcName == "" {
		return nil
	}

	// attach pvc volume only if it is not already attached
	for _, buildVol := range b.Spec.Volumes {
		if buildVol.Name == pipelineRunpvcName {
			return nil
		}
	}
	b.Spec.Volumes = append(b.Spec.Volumes, GetPVCVolume(pipelineRunpvcName))
	return nil
}

func addStoreUploadStep(build *buildv1alpha1.Build,
	storageResource v1alpha1.PipelineStorageResourceInterface,
	sourcePath string,
) error {
	storageResource.SetDestinationDirectory(sourcePath)
	gcsContainers, err := storageResource.GetUploadContainerSpec()
	if err != nil {
		return err
	}
	mountedSecrets := map[string]string{}

	for _, volume := range build.Spec.Volumes {
		mountedSecrets[volume.Name] = ""
	}
	var buildSteps []corev1.Container
	for _, gcsContainer := range gcsContainers {
		gcsContainer.VolumeMounts = append(gcsContainer.VolumeMounts, corev1.VolumeMount{
			Name:      "workspace",
			MountPath: workspaceDir,
		})
		// Map holds list of secrets that are mounted as volumes
		for _, secretParam := range storageResource.GetSecretParams() {
			volName := fmt.Sprintf("volume-%s-%s", storageResource.GetName(), secretParam.SecretName)

			gcsSecretVolume := corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretParam.SecretName,
					},
				},
			}

			if _, ok := mountedSecrets[volName]; !ok {
				build.Spec.Volumes = append(build.Spec.Volumes, gcsSecretVolume)
				mountedSecrets[volName] = ""
			}
		}
		buildSteps = append(buildSteps, gcsContainer)
	}
	build.Spec.Steps = append(build.Spec.Steps, buildSteps...)
	return nil
}
