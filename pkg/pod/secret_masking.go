/*
Copyright 2026 The Tekton Authors

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
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const (
	secretMaskVolumePrefix     = "tekton-secret-mask"
	secretMaskVolumeHashLength = 8
)

// secretMaskingVolumes inspects a TaskSpec for any references to Kubernetes
// Secrets (via env vars, envFrom, or volumes) and returns Volumes and
// VolumeMounts that expose those secrets read-only under SecretMaskDir.
// The entrypoint reads these at startup to build exact-match redaction strings.
func secretMaskingVolumes(taskSpec v1.TaskSpec) ([]corev1.Volume, []corev1.VolumeMount) {
	secretNames := collectSecretNames(taskSpec)
	if len(secretNames) == 0 {
		return nil, nil
	}

	var volumes []corev1.Volume
	var mounts []corev1.VolumeMount
	usedNames := make(map[string]struct{})

	for _, secretName := range secretNames {
		volName := names.GenerateHashedName(secretMaskVolumePrefix, secretName, secretMaskVolumeHashLength)
		for _, exists := usedNames[volName]; exists; _, exists = usedNames[volName] {
			volName = names.GenerateHashedName(secretMaskVolumePrefix, volName+"$", secretMaskVolumeHashLength)
		}
		usedNames[volName] = struct{}{}

		volumes = append(volumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
					Optional:   boolPtr(true),
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      volName,
			MountPath: filepath.Join(pipeline.SecretMaskDir, secretName),
			ReadOnly:  true,
		})
	}

	return volumes, mounts
}

// collectSecretNames walks the TaskSpec and returns a deduplicated, ordered
// list of Secret names referenced by steps, step template, sidecars, and
// task-level volumes.
func collectSecretNames(taskSpec v1.TaskSpec) []string {
	seen := make(map[string]struct{})
	var result []string

	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}

	collectFromEnv := func(env []corev1.EnvVar) {
		for _, e := range env {
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				add(e.ValueFrom.SecretKeyRef.LocalObjectReference.Name)
			}
		}
	}
	collectFromEnvFrom := func(envFrom []corev1.EnvFromSource) {
		for _, ef := range envFrom {
			if ef.SecretRef != nil {
				add(ef.SecretRef.LocalObjectReference.Name)
			}
		}
	}

	// StepTemplate
	if taskSpec.StepTemplate != nil {
		collectFromEnv(taskSpec.StepTemplate.Env)
		collectFromEnvFrom(taskSpec.StepTemplate.EnvFrom)
	}

	// Steps
	for _, step := range taskSpec.Steps {
		collectFromEnv(step.Env)
		collectFromEnvFrom(step.EnvFrom)
	}

	// Sidecars
	for _, sc := range taskSpec.Sidecars {
		collectFromEnv(sc.Env)
		collectFromEnvFrom(sc.EnvFrom)
	}

	// Task-level volumes that reference a Secret
	for _, vol := range taskSpec.Volumes {
		if vol.VolumeSource.Secret != nil {
			add(vol.VolumeSource.Secret.SecretName)
		}
	}

	return result
}

func boolPtr(b bool) *bool {
	return &b
}
