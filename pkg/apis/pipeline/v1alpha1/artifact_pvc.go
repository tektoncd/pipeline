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

package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

var (
	pvcDir = "/pvc"
)

// ArtifactPVC represents the pvc created by the pipelinerun for artifacts temporary storage.
type ArtifactPVC struct {
	Name                  string
	PersistentVolumeClaim *corev1.PersistentVolumeClaim

	ShellImage string
}

// GetType returns the type of the artifact storage.
func (p *ArtifactPVC) GetType() string {
	return pipeline.ArtifactStoragePVCType
}

// StorageBasePath returns the path to be used to store artifacts in a pipelinerun temporary storage.
func (p *ArtifactPVC) StorageBasePath(pr *PipelineRun) string {
	return pvcDir
}

// GetCopyFromStorageToSteps returns a container used to download artifacts from temporary storage.
func (p *ArtifactPVC) GetCopyFromStorageToSteps(name, sourcePath, destinationPath string) []Step {
	return []Step{{Container: corev1.Container{
		Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("source-copy-%s", name)),
		Image:   p.ShellImage,
		Command: []string{"cp", "-r", fmt.Sprintf("%s/.", sourcePath), destinationPath},
	}}}
}

// GetCopyToStorageFromSteps returns a container used to upload artifacts for temporary storage.
func (p *ArtifactPVC) GetCopyToStorageFromSteps(name, sourcePath, destinationPath string) []Step {
	return []Step{{Container: corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("source-mkdir-%s", name)),
		Image:        p.ShellImage,
		Command:      []string{"mkdir", "-p", destinationPath},
		VolumeMounts: []corev1.VolumeMount{GetPvcMount(p.Name)},
	}}, {Container: corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("source-copy-%s", name)),
		Image:        p.ShellImage,
		Command:      []string{"cp", "-r", fmt.Sprintf("%s/.", sourcePath), destinationPath},
		VolumeMounts: []corev1.VolumeMount{GetPvcMount(p.Name)},
	}}}
}

// GetPvcMount returns a mounting of the volume with the mount path /pvc.
func GetPvcMount(name string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,   // taskrun pvc name
		MountPath: pvcDir, // nothing should be mounted here
	}
}

// CreateDirStep returns a container step to create a dir at destinationPath. The name
// of the step will include name.
func CreateDirStep(shellImage string, name, destinationPath string) Step {
	return Step{Container: corev1.Container{
		Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("create-dir-%s", strings.ToLower(name))),
		Image:   shellImage,
		Command: []string{"mkdir", "-p", destinationPath},
	}}
}

// GetSecretsVolumes returns the list of volumes for secrets to be mounted on
// pod.
func (p *ArtifactPVC) GetSecretsVolumes() []corev1.Volume { return nil }
