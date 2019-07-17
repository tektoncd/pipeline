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
	"fmt"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/artifacts"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func getBoundResource(resourceName string, boundResources []v1alpha1.TaskResourceBinding) (*v1alpha1.TaskResourceBinding, error) {
	for _, br := range boundResources {
		if br.Name == resourceName {
			return &br, nil
		}
	}
	return nil, xerrors.Errorf("couldnt find resource named %q in bound resources %v", resourceName, boundResources)
}

// AddInputResource reads the inputs resources and adds the corresponding container steps
// This function reads the `paths` to check if resource copies needs to be fetched from previous tasks output(from PVC)
// 1. If resource has paths declared then serially copies the resource from previous task output paths into current resource destination.
// 2. If resource has custom destination directory using targetPath then that directory is created and resource is fetched / copied
// from  previous task
// 3. If resource has paths declared then fresh copy of resource is not fetched
func AddInputResource(
	kubeclient kubernetes.Interface,
	taskName string,
	taskSpec *v1alpha1.TaskSpec,
	taskRun *v1alpha1.TaskRun,
	inputResources map[string]v1alpha1.PipelineResourceInterface,
	logger *zap.SugaredLogger,
) (*v1alpha1.TaskSpec, error) {

	if taskSpec.Inputs == nil {
		return taskSpec, nil
	}
	taskSpec = taskSpec.DeepCopy()

	pvcName := taskRun.GetPipelineRunPVCName()
	mountPVC := false

	prNameFromLabel := taskRun.Labels[pipeline.GroupName+pipeline.PipelineRunLabelKey]
	if prNameFromLabel == "" {
		prNameFromLabel = pvcName
	}
	as, err := artifacts.GetArtifactStorage(prNameFromLabel, kubeclient, logger)
	if err != nil {
		return nil, err
	}

	allResourceContainers := []corev1.Container{}

	for _, input := range taskSpec.Inputs.Resources {
		boundResource, err := getBoundResource(input.Name, taskRun.Spec.Inputs.Resources)
		if err != nil {
			return nil, xerrors.Errorf("failed to get bound resource: %w", err)
		}
		resource, ok := inputResources[boundResource.Name]
		if !ok || resource == nil {
			return nil, xerrors.Errorf("failed to Get Pipeline Resource for task %s with boundResource %v", taskName, boundResource)
		}
		var (
			resourceContainers     []corev1.Container
			resourceVolumes        []corev1.Volume
			copyStepsFromPrevTasks []corev1.Container
			dPath                  = destinationPath(input.Name, input.TargetPath)
		)
		resource.SetDestinationDirectory(dPath)
		// if taskrun is fetching resource from previous task then execute copy step instead of fetching new copy
		// to the desired destination directory, as long as the resource exports output to be copied
		if allowedOutputResources[resource.GetType()] && taskRun.HasPipelineRunOwnerReference() {
			for _, path := range boundResource.Paths {
				cpContainers := as.GetCopyFromStorageToContainerSpec(boundResource.Name, path, dPath)
				if as.GetType() == v1alpha1.ArtifactStoragePVCType {

					mountPVC = true
					for _, ct := range cpContainers {
						ct.VolumeMounts = []corev1.VolumeMount{v1alpha1.GetPvcMount(pvcName)}
						createAndCopyContainers := []corev1.Container{v1alpha1.CreateDirContainer(boundResource.Name, dPath), ct}
						copyStepsFromPrevTasks = append(copyStepsFromPrevTasks, createAndCopyContainers...)
					}
				} else {
					// bucket
					copyStepsFromPrevTasks = append(copyStepsFromPrevTasks, cpContainers...)
				}
			}
		}
		// source is copied from previous task so skip fetching download container definition
		if len(copyStepsFromPrevTasks) > 0 {
			taskSpec.Steps = append(copyStepsFromPrevTasks, taskSpec.Steps...)
			taskSpec.Volumes = append(taskSpec.Volumes, as.GetSecretsVolumes()...)
		} else {
			switch resource.GetType() {
			case v1alpha1.PipelineResourceTypeStorage:
				{
					storageResource, ok := resource.(v1alpha1.PipelineStorageResourceInterface)
					if !ok {
						return nil, xerrors.Errorf("task %q invalid gcs Pipeline Resource: %q", taskName, boundResource.ResourceRef.Name)
					}
					resourceContainers, resourceVolumes, err = addStorageFetchStep(taskSpec, storageResource)
					if err != nil {
						return nil, xerrors.Errorf("task %q invalid gcs Pipeline Resource download steps: %q: %w", taskName, boundResource.ResourceRef.Name, err)
					}
				}
			default:
				{
					resourceContainers, err = resource.GetDownloadContainerSpec()
					if err != nil {
						return nil, xerrors.Errorf("task %q invalid resource download spec: %q; error %w", taskName, boundResource.ResourceRef.Name, err)
					}
				}
			}

			allResourceContainers = append(allResourceContainers, resourceContainers...)
			taskSpec.Volumes = append(taskSpec.Volumes, resourceVolumes...)
		}
	}
	taskSpec.Steps = append(allResourceContainers, taskSpec.Steps...)

	if mountPVC {
		taskSpec.Volumes = append(taskSpec.Volumes, GetPVCVolume(pvcName))
	}
	return taskSpec, nil
}

func addStorageFetchStep(taskSpec *v1alpha1.TaskSpec, storageResource v1alpha1.PipelineStorageResourceInterface) ([]corev1.Container, []corev1.Volume, error) {
	gcsContainers, err := storageResource.GetDownloadContainerSpec()
	if err != nil {
		return nil, nil, err
	}

	var storageVol []corev1.Volume
	mountedSecrets := map[string]string{}
	for _, volume := range taskSpec.Volumes {
		mountedSecrets[volume.Name] = ""
	}

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
			storageVol = append(storageVol, gcsSecretVolume)
			mountedSecrets[volName] = ""
		}
	}
	return gcsContainers, storageVol, nil
}

func getResource(r *v1alpha1.TaskResourceBinding, getter GetResource) (*v1alpha1.PipelineResource, error) {
	// Check both resource ref or resource Spec are not present. Taskrun webhook should catch this in validation error.
	if r.ResourceRef.Name != "" && r.ResourceSpec != nil {
		return nil, xerrors.New("Both ResourseRef and ResourceSpec are defined. Expected only one")
	}

	if r.ResourceRef.Name != "" {
		return getter(r.ResourceRef.Name)
	}
	if r.ResourceSpec != nil {
		return &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: r.Name,
			},
			Spec: *r.ResourceSpec,
		}, nil
	}
	return nil, xerrors.New("Neither ResourseRef not ResourceSpec is defined")
}

func destinationPath(name, path string) string {
	if path == "" {
		return filepath.Join(workspaceDir, name)
	}
	return filepath.Join(workspaceDir, path)
}
