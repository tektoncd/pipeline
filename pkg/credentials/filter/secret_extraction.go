package filter

import (
	"encoding/json"
	"fmt"
	"path"

	corev1 "k8s.io/api/core/v1"
)

const (
	downwardMountNamePrefix          = "tekton-internal-secretlocations-"
	downwardMountPath                = "/tekton/secretlocations/"
	downwardMountSecretLocationsFile = "secret-locations.json"
	// SecretLocationsAnnotationPrefix is the prefix for the annotation key that will be used to store
	// secret locations for a specific container. The container name will be appended to the prefix.
	// nolint: gosec
	SecretLocationsAnnotationPrefix = "tekton.dev/secret-locations-"
)

// ApplySecretLocationsToPod applies an environment variable to each container in the pod
// reflecting the locations of secrets that are available in that container.
// This is done by the controller to tell the entrypoint runner where to find secrets that
// need to be redacted.
func ApplySecretLocationsToPod(pod *corev1.Pod) error {
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		secretLocations, err := ExtractSecretLocationsFromPodContainer(pod, container)
		if err != nil {
			return err
		}

		jsonString, err := json.Marshal(secretLocations)
		if err != nil {
			return err
		}

		annotationName := SecretLocationsAnnotationPrefix + container.Name
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[annotationName] = string(jsonString)

		mountName := downwardMountNamePrefix + container.Name
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: mountName,
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{{
						Path: downwardMountSecretLocationsFile,
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: fmt.Sprintf("metadata.annotations['%s']", annotationName),
						},
					}},
				},
			},
		})

		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      mountName,
			ReadOnly:  true,
			MountPath: downwardMountPath,
		})

	}

	return nil
}

// ExtractSecretLocationsFromPodContainer searches in the given pod and container for any referenced secrets
// and returns the location of those secrets in the container when it will be running.
func ExtractSecretLocationsFromPodContainer(pod *corev1.Pod, container *corev1.Container) (*SecretLocations, error) {
	secretLocations := SecretLocations{
		EnvironmentVariables: []string{},
		Files:                []string{},
	}
	for _, envVar := range container.Env {
		if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
			secretLocations.EnvironmentVariables = append(secretLocations.EnvironmentVariables, envVar.Name)
		}
	}

	appendItems := func(mountPath string, files []string, items []corev1.KeyToPath) []string {
		for _, item := range items {
			filePath := item.Key
			if item.Path != "" {
				filePath = item.Path
			}
			files = append(files, path.Join(mountPath, filePath))
		}

		return files
	}

	for _, volMount := range container.VolumeMounts {
		vol := getVolume(pod, volMount.Name)
		if vol == nil {
			return nil, fmt.Errorf("could not find matching volume for volume mount %s", volMount.Name)
		}
		// nolint:gocritic
		if vol.Secret != nil {
			if len(vol.Secret.Items) > 0 {
				secretLocations.Files = appendItems(volMount.MountPath, secretLocations.Files, vol.Secret.Items)
			} else {
				secretLocations.Files = append(secretLocations.Files, path.Join(volMount.MountPath))
			}
		} else if vol.Projected != nil {
			for _, source := range vol.Projected.Sources {
				if source.Secret != nil {
					if len(source.Secret.Items) > 0 {
						secretLocations.Files = appendItems(volMount.MountPath, secretLocations.Files, source.Secret.Items)
					} else {
						secretLocations.Files = append(secretLocations.Files, volMount.MountPath)
					}
				}
			}
		} else if vol.CSI != nil && vol.CSI.Driver == "secrets-store.csi.k8s.io" {
			secretLocations.Files = append(secretLocations.Files, path.Join(volMount.MountPath))
		}
	}

	return &secretLocations, nil
}

func getVolume(pod *corev1.Pod, volumeName string) *corev1.Volume {
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == volumeName {
			return &vol
		}
	}

	return nil
}
