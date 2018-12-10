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
	"flag"
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
	kubeconfigWriterImage = flag.String("kubeconfig-writer-image", "override-with-kubeconfig-writer:latest", "The container image containing our kubeconfig writer binary.")
	bashNoopImage         = flag.String("bash-noop-image", "override-with-bash-noop:latest", "The container image containing bash shell")
)

func getBoundResource(resourceName string, boundResources []v1alpha1.TaskResourceBinding) (*v1alpha1.TaskResourceBinding, error) {
	for _, br := range boundResources {
		if br.Name == resourceName {
			return &br, nil
		}
	}
	return nil, fmt.Errorf("couldnt find resource named %q in bound resources %s", resourceName, boundResources)
}

// AddInputResource reads the inputs resources and adds the corresponding container steps
// This function reads the `paths` to check if resource copies needs to be fetched from previous tasks output(from PVC)
// 1. If resource has paths declared then serially copies the resource from previous task output paths into current resource destination.
// 2. If resource has custom destination directory using targetPath then that directory is created and resource is fetched / copied
// from  previous task
// 3. If resource has paths declared then fresh copy of resource is not fetched
func AddInputResource(
	build *buildv1alpha1.Build,
	taskName string,
	taskSpec *v1alpha1.TaskSpec,
	taskRun *v1alpha1.TaskRun,
	pipelineResourceLister listers.PipelineResourceLister,
	logger *zap.SugaredLogger,
) (*buildv1alpha1.Build, error) {

	if taskSpec.Inputs == nil {
		return build, nil
	}
	pvcName := taskRun.GetPipelineRunPVCName()
	mountPVC := false

	for _, input := range taskSpec.Inputs.Resources {
		boundResource, err := getBoundResource(input.Name, taskRun.Spec.Inputs.Resources)
		if err != nil {
			return nil, fmt.Errorf("Failed to get bound resource: %s", err)
		}

		resource, err := pipelineResourceLister.PipelineResources(taskRun.Namespace).Get(boundResource.ResourceRef.Name)
		if err != nil {
			return nil, fmt.Errorf("task %q failed to Get Pipeline Resource: %q", taskName, boundResource)
		}

		// if taskrun is fetching resource from previous task then execute copy step instead of fetching new copy
		// to the desired destination directory
		var copyStepsFromPrevTasks []corev1.Container

		for i, path := range boundResource.Paths {
			var dPath string
			if input.TargetPath == "" {
				dPath = workspaceDir
			} else {
				dPath = filepath.Join(workspaceDir, input.TargetPath)
			}
			cpContainer := copyContainer(fmt.Sprintf("%s-%d", boundResource.Name, i), path, dPath)
			cpContainer.VolumeMounts = []corev1.VolumeMount{getPvcMount(pvcName)}
			copyStepsFromPrevTasks = append(copyStepsFromPrevTasks, cpContainer)
			mountPVC = true
		}

		// source is copied from previous task so skip fetching cluster , storage types
		if len(copyStepsFromPrevTasks) > 0 {
			build.Spec.Steps = append(copyStepsFromPrevTasks, build.Spec.Steps...)
		} else {
			switch resource.Spec.Type {
			case v1alpha1.PipelineResourceTypeGit:
				{
					gitResource, err := v1alpha1.NewGitResource(resource)
					if err != nil {
						return nil, fmt.Errorf("task %q invalid git Pipeline Resource: %q; error %s", taskName, boundResource.ResourceRef.Name, err.Error())
					}
					gitSourceSpec := &buildv1alpha1.GitSourceSpec{
						Url:      gitResource.URL,
						Revision: gitResource.Revision,
					}
					build.Spec.Sources = append(build.Spec.Sources, buildv1alpha1.SourceSpec{
						Git:        gitSourceSpec,
						TargetPath: input.TargetPath,
						Name:       gitResource.Name,
					})
				}
			case v1alpha1.PipelineResourceTypeCluster:
				{
					clusterResource, err := v1alpha1.NewClusterResource(resource)
					if err != nil {
						return nil, fmt.Errorf("task %q invalid cluster Pipeline Resource: %q: error %s", taskName, boundResource.ResourceRef.Name, err.Error())
					}
					addClusterBuildStep(build, clusterResource)
				}
			case v1alpha1.PipelineResourceTypeStorage:
				{
					storageResource, err := v1alpha1.NewStorageResource(resource)
					if err != nil {
						return nil, fmt.Errorf("task %q invalid gcs Pipeline Resource: %q: %s", taskName, boundResource.ResourceRef.Name, err.Error())
					}
					fetchErr := addStorageFetchStep(build, storageResource, input.TargetPath)
					if err != nil {
						return nil, fmt.Errorf("task %q invalid gcs Pipeline Resource download steps: %q: %s", taskName, boundResource.ResourceRef.Name, fetchErr.Error())
					}
				}
			}
		}
	}

	if mountPVC {
		build.Spec.Volumes = append(build.Spec.Volumes, GetPVCVolume(pvcName))
	}
	return build, nil
}

func addClusterBuildStep(build *buildv1alpha1.Build, clusterResource *v1alpha1.ClusterResource) {
	var envVars []corev1.EnvVar
	for _, sec := range clusterResource.Secrets {
		ev := corev1.EnvVar{
			Name: strings.ToUpper(sec.FieldName),
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: sec.SecretName,
					},
					Key: sec.SecretKey,
				},
			},
		}
		envVars = append(envVars, ev)
	}

	clusterContainer := corev1.Container{
		Name:  "kubeconfig",
		Image: *kubeconfigWriterImage,
		Args: []string{
			"-clusterConfig", clusterResource.String(),
		},
		Env: envVars,
	}

	buildSteps := append([]corev1.Container{clusterContainer}, build.Spec.Steps...)
	build.Spec.Steps = buildSteps
}

func addStorageFetchStep(build *buildv1alpha1.Build, storageResource v1alpha1.PipelineStorageResourceInterface, destPath string) error {
	var destDirectory = workspaceDir
	if destPath != "" {
		destDirectory = filepath.Join(workspaceDir, destPath)
	}

	storageResource.SetDestinationDirectory(destDirectory)
	gcsCreateDirContainer := createDirContainer(storageResource.GetName(), destDirectory)
	gcsDownloadContainers, err := storageResource.GetDownloadContainerSpec()
	if err != nil {
		return err
	}
	mountedSecrets := map[string]string{}
	for _, volume := range build.Spec.Volumes {
		mountedSecrets[volume.Name] = ""
	}
	var buildSteps []corev1.Container
	gcsContainers := append([]corev1.Container{gcsCreateDirContainer}, gcsDownloadContainers...)
	for _, gcsContainer := range gcsContainers {

		gcsContainer.VolumeMounts = append(gcsContainer.VolumeMounts, corev1.VolumeMount{
			Name:      "workspace",
			MountPath: workspaceDir,
		})
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
	build.Spec.Steps = append(buildSteps, build.Spec.Steps...)
	return nil
}

func createDirContainer(name, destinationPath string) corev1.Container {
	return corev1.Container{
		Name:  fmt.Sprintf("create-dir-%s", name),
		Image: *bashNoopImage,
		Args:  []string{"-args", strings.Join([]string{"mkdir", "-p", destinationPath}, " ")},
	}
}

func copyContainer(name, sourcePath, destinationPath string) corev1.Container {
	return corev1.Container{
		Name:  fmt.Sprintf("source-copy-%s", name),
		Image: *bashNoopImage,
		Args:  []string{"-args", strings.Join([]string{"cp", "-r", fmt.Sprintf("%s/.", sourcePath), destinationPath}, " ")},
	}
}
