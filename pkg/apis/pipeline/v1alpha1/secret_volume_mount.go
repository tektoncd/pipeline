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
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var (
	secretFileVarNames = []string{
		"GOOGLE_APPLICATION_CREDENTIALS",
	}
	secretEnvVarNames = []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
	}
)

func isSecretAuthFileVar(name string) bool {
	for _, a := range secretFileVarNames {
		if a == name {
			return true
		}
	}
	return false
}
func isEnvSecretVar(name string) bool {
	for _, a := range secretEnvVarNames {
		if a == name {
			return true
		}
	}
	return false
}

func getSecretEnvVarsAndVolumeMounts(name, mountPath string, secrets []SecretParam) ([]corev1.EnvVar, []corev1.VolumeMount) {
	mountPaths := make(map[string]struct{})
	var (
		envVars           []corev1.EnvVar
		secretVolumeMount []corev1.VolumeMount
	)
OUTER:
	for _, secretParam := range secrets {
		if isSecretAuthFileVar(secretParam.FieldName) {
			// We dont want dupe env vars specified / mounted
			for _, e := range envVars {
				if strings.ToUpper(secretParam.FieldName) == e.Name {
					continue OUTER
				}
			}

			mountPath := filepath.Join(mountPath, secretParam.SecretName)

			envVars = append(envVars, corev1.EnvVar{
				Name:  strings.ToUpper(secretParam.FieldName),
				Value: filepath.Join(mountPath, secretParam.SecretKey),
			})

			if _, ok := mountPaths[mountPath]; !ok {
				secretVolumeMount = append(secretVolumeMount, corev1.VolumeMount{
					Name:      fmt.Sprintf("volume-%s-%s", name, secretParam.SecretName),
					MountPath: mountPath,
				})
				mountPaths[mountPath] = struct{}{}
			}
		}
	}
	return envVars, secretVolumeMount
}

func getEnvVarsSecrets(name string, secrets []SecretParam) []corev1.EnvVar {
	var envVars []corev1.EnvVar
OUTER:
	for _, secretParam := range secrets {
		if isEnvSecretVar(secretParam.FieldName) {
			// We dont want dupe env vars specified
			for _, e := range envVars {
				if strings.ToUpper(secretParam.FieldName) == e.Name {
					continue OUTER
				}
			}

			envVars = append(envVars, corev1.EnvVar{
				Name: strings.ToUpper(secretParam.FieldName),
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: secretParam.SecretKey,
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretParam.SecretName,
						},
					},
				},
			})
		}
	}
	return envVars
}
