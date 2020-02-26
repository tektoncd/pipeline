/*
Copyright 2019-2020 The Tekton Authors

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

package storage

import (
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const (
	// ArtifactStorageBucketType holds the name of the PipelineResource type for a bucket
	ArtifactStorageBucketType = "bucket"

	// ArtifactStoragePVCType holds the name of the PipelineResource type for a pvc
	ArtifactStoragePVCType = "pvc"
)

// It adds a function to the PipelineResourceInterface for retrieving secrets that are usually
// needed for storage PipelineResources.
type PipelineStorageResourceInterface interface {
	v1alpha1.PipelineResourceInterface
	GetSecretParams() []v1alpha1.SecretParam
}

// NewResource returns an instance of the requested storage subtype, which can be used
// to add input and output steps and volumes to an executing pod.
func NewResource(images pipeline.Images, r *resource.PipelineResource) (PipelineStorageResourceInterface, error) {
	if r.Spec.Type != v1alpha1.PipelineResourceTypeStorage {
		return nil, fmt.Errorf("StoreResource: Cannot create a storage resource from a %s Pipeline Resource", r.Spec.Type)
	}

	for _, param := range r.Spec.Params {
		if strings.EqualFold(param.Name, "type") {
			switch {
			case strings.EqualFold(param.Value, string(resource.PipelineResourceTypeGCS)):
				return NewGCSResource(images, r)
			case strings.EqualFold(param.Value, string(resource.PipelineResourceTypeBuildGCS)):
				return NewBuildGCSResource(images, r)
			default:
				return nil, fmt.Errorf("%s is an invalid or unimplemented PipelineStorageResource", param.Value)
			}
		}
	}
	return nil, fmt.Errorf("StoreResource: Cannot create a storage resource without type %s in spec", r.Name)
}

func getStorageVolumeSpec(s PipelineStorageResourceInterface, spec v1alpha1.TaskSpec) []corev1.Volume {
	var storageVol []corev1.Volume
	mountedSecrets := map[string]string{}

	for _, volume := range spec.Volumes {
		mountedSecrets[volume.Name] = ""
	}

	// Map holds list of secrets that are mounted as volumes
	for _, secretParam := range s.GetSecretParams() {
		volName := fmt.Sprintf("volume-%s-%s", s.GetName(), secretParam.SecretName)

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
	return storageVol
}

// of the step will include name.
func CreateDirStep(shellImage string, name, destinationPath string) v1alpha1.Step {
	return v1alpha1.Step{Container: corev1.Container{
		Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("create-dir-%s", strings.ToLower(name))),
		Image:   shellImage,
		Command: []string{"mkdir", "-p", destinationPath},
	}}
}
