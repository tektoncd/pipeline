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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

// For some reason gosec thinks this string has enough entropy to be a potential secret.
// The nosec comment disables it for this line.
/* #nosec */
const secretVolumeMountPath = "/var/bucketsecret"

// ArtifactBucket contains the Storage bucket configuration defined in the
// Bucket config map.
type ArtifactBucket struct {
	Name     string
	Location string
	Secrets  []SecretParam

	ShellImage  string
	GsutilImage string
}

// GetType returns the type of the artifact storage
func (b *ArtifactBucket) GetType() string {
	return pipeline.ArtifactStorageBucketType
}

// StorageBasePath returns the path to be used to store artifacts in a pipelinerun temporary storage
func (b *ArtifactBucket) StorageBasePath(pr *PipelineRun) string {
	return fmt.Sprintf("%s-%s-bucket", pr.Name, pr.Namespace)
}

// GetCopyFromStorageToSteps returns a container used to download artifacts from temporary storage
func (b *ArtifactBucket) GetCopyFromStorageToSteps(name, sourcePath, destinationPath string) []Step {
	envVars, secretVolumeMount := getSecretEnvVarsAndVolumeMounts("bucket", secretVolumeMountPath, b.Secrets)

	return []Step{{Container: corev1.Container{
		Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("artifact-dest-mkdir-%s", name)),
		Image:   b.ShellImage,
		Command: []string{"mkdir", "-p", destinationPath},
	}}, {Container: corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("artifact-copy-from-%s", name)),
		Image:        b.GsutilImage,
		Command:      []string{"gsutil"},
		Args:         []string{"cp", "-P", "-r", fmt.Sprintf("%s/%s/*", b.Location, sourcePath), destinationPath},
		Env:          envVars,
		VolumeMounts: secretVolumeMount,
	}}}
}

// GetCopyToStorageFromSteps returns a container used to upload artifacts for temporary storage
func (b *ArtifactBucket) GetCopyToStorageFromSteps(name, sourcePath, destinationPath string) []Step {
	envVars, secretVolumeMount := getSecretEnvVarsAndVolumeMounts("bucket", secretVolumeMountPath, b.Secrets)

	return []Step{{Container: corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("artifact-copy-to-%s", name)),
		Image:        b.GsutilImage,
		Command:      []string{"gsutil"},
		Args:         []string{"cp", "-P", "-r", sourcePath, fmt.Sprintf("%s/%s", b.Location, destinationPath)},
		Env:          envVars,
		VolumeMounts: secretVolumeMount,
	}}}
}

// GetSecretsVolumes returns the list of volumes for secrets to be mounted
// on pod
func (b *ArtifactBucket) GetSecretsVolumes() []corev1.Volume {
	volumes := []corev1.Volume{}
	for _, sec := range b.Secrets {
		volumes = append(volumes, corev1.Volume{
			Name: fmt.Sprintf("volume-bucket-%s", sec.SecretName),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: sec.SecretName,
				},
			},
		})
	}
	return volumes
}
