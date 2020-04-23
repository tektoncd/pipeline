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
	corev1 "k8s.io/api/core/v1"
)

// GetPVCVolume gets pipelinerun pvc volume
func GetPVCVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name},
		},
	}
}

// appendNewSecretsVolumes takes a variadic list of secret volumes and a list of volumes, and
// appends any secret volumes that aren't already present. The secret volumes are volumes whose
// VolumeSource is set to *corev1.SecretVolumeSource with only the SecretName field filled in.
// Specifically, they have the following structure as defined by
// (pkg/apis/resource/v1alpha1/storage.ArtifactBucket).GetSecretsVolumes():
//
// corev1.Volume{
//   Name: fmt.Sprintf("volume-bucket-%s", sec.SecretName),
//   VolumeSource: corev1.VolumeSource{
//     Secret: &corev1.SecretVolumeSource{
//       SecretName: sec.SecretName,
//     },
//   },
// }
//
// Any new volumes that don't match this structure are added regardless of whether they are already
// present in the list of volumes.
func appendNewSecretsVolumes(vols []corev1.Volume, newVols ...corev1.Volume) []corev1.Volume {
	for _, newv := range newVols {
		alreadyExists := false
		for _, oldv := range vols {
			if newv.Name != oldv.Name {
				continue
			}
			if oldv.Secret == nil {
				continue
			}
			if oldv.Secret.SecretName == newv.Secret.SecretName {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			vols = append(vols, newv)
		}
	}
	return vols
}
