/*
Copyright 2025 The Tekton Authors

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

package pod

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// SecretMaskVolumeName is the name of the emptyDir volume for secret masking
	SecretMaskVolumeName = "tekton-internal-secret-mask" //nolint:gosec // G101: not a hardcoded credential, just a volume name
	// SecretMaskMountPath is the mount path for the secret mask volume
	SecretMaskMountPath = "/tekton/secret-mask" //nolint:gosec // G101: not a hardcoded credential, just a mount path
	// SecretMaskFileName is the name of the file containing secrets to mask
	SecretMaskFileName = "secrets"
	// SecretMaskDataEnvVar is the env var carrying base64(gzip(secret mask data)) to the init container.
	SecretMaskDataEnvVar = "TEKTON_SECRET_MASK_DATA" //nolint:gosec // G101: not a hardcoded credential, just a variable name
)

// SecretMaskFilePath returns the full path to the secrets mask file.
func SecretMaskFilePath() string {
	return SecretMaskMountPath + "/" + SecretMaskFileName
}

// CollectSecretsForMasking gathers secret values from steps and volumes and returns them base64-encoded, one per line.
func CollectSecretsForMasking(ctx context.Context, kubeclient kubernetes.Interface, namespace string, taskSpec *v1.TaskSpec, steps []v1.Step, volumes []corev1.Volume) (string, error) {
	secretValues := make(map[string]struct{})

	addValues := func(values []string) {
		for _, value := range values {
			if value == "" {
				continue
			}
			secretValues[value] = struct{}{}
		}
	}

	for _, step := range steps {
		for _, env := range step.Env {
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				continue
			}
			secretRef := env.ValueFrom.SecretKeyRef
			value, err := getSecretValue(ctx, kubeclient, namespace, secretRef.Name, secretRef.Key)
			if err != nil {
				if secretRef.Optional != nil && *secretRef.Optional {
					continue
				}
				return "", err
			}
			addValues([]string{value})
		}

		for _, envFrom := range step.EnvFrom {
			if envFrom.SecretRef == nil {
				continue
			}
			values, err := getAllSecretValues(ctx, kubeclient, namespace, envFrom.SecretRef.Name)
			if err != nil {
				if envFrom.SecretRef.Optional != nil && *envFrom.SecretRef.Optional {
					continue
				}
				return "", err
			}
			addValues(values)
		}
	}

	addSecretVolumes := func(vols []corev1.Volume) error {
		for _, vol := range vols {
			if vol.Secret == nil {
				continue
			}
			values, err := getAllSecretValues(ctx, kubeclient, namespace, vol.Secret.SecretName)
			if err != nil {
				if vol.Secret.Optional != nil && *vol.Secret.Optional {
					continue
				}
				return err
			}
			addValues(values)
		}
		return nil
	}

	if err := addSecretVolumes(volumes); err != nil {
		return "", err
	}
	if taskSpec != nil {
		if err := addSecretVolumes(taskSpec.Volumes); err != nil {
			return "", err
		}
	}

	var lines []string
	for secret := range secretValues {
		if len(secret) < 3 {
			continue
		}
		lines = append(lines, base64.StdEncoding.EncodeToString([]byte(secret)))
	}

	return strings.Join(lines, "\n"), nil
}

func getSecretValue(ctx context.Context, kubeclient kubernetes.Interface, namespace, secretName, key string) (string, error) {
	secret, err := kubeclient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	data, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s missing key %s", secretName, key)
	}
	return string(data), nil
}

func getAllSecretValues(ctx context.Context, kubeclient kubernetes.Interface, namespace, secretName string) ([]string, error) {
	secret, err := kubeclient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(secret.Data))
	for _, data := range secret.Data {
		values = append(values, string(data))
	}
	return values, nil
}

func encodeSecretMaskData(data string) (string, error) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write([]byte(data)); err != nil {
		return "", fmt.Errorf("compress secret mask data: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("finalize compressed secret mask data: %w", err)
	}
	return base64.StdEncoding.EncodeToString(gz.Bytes()), nil
}

// SecretMaskVolume returns the emptyDir volume for storing secret mask data.
func SecretMaskVolume() corev1.Volume {
	return corev1.Volume{
		Name: SecretMaskVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
}

// SecretMaskVolumeMount returns the volume mount for the secret mask volume.
func SecretMaskVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      SecretMaskVolumeName,
		MountPath: SecretMaskMountPath,
		ReadOnly:  true,
	}
}

// SecretMaskInitVolumeMount returns the volume mount for the init container (writable).
func SecretMaskInitVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      SecretMaskVolumeName,
		MountPath: SecretMaskMountPath,
		ReadOnly:  false,
	}
}

func secretMaskInitContainer(image, secretData string, securityContext SecurityContextConfig, windows bool) corev1.Container {
	command := []string{"/ko-app/entrypoint", "secret-mask-init", SecretMaskFilePath()}

	container := corev1.Container{
		Name:       "secret-mask-init",
		Image:      image,
		Command:    command,
		WorkingDir: "/",
		Env: []corev1.EnvVar{
			{
				Name:  SecretMaskDataEnvVar,
				Value: secretData,
			},
		},
		VolumeMounts: []corev1.VolumeMount{SecretMaskInitVolumeMount()},
	}

	if securityContext.SetSecurityContext {
		container.SecurityContext = securityContext.GetSecurityContext(windows)
	}

	return container
}
