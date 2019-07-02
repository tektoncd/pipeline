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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/templating"
	corev1 "k8s.io/api/core/v1"
)

// ApplyParameters applies the params from a TaskRun.Input.Parameters to a TaskSpec
func ApplyParameters(spec *v1alpha1.TaskSpec, tr *v1alpha1.TaskRun, defaults ...v1alpha1.ParamSpec) *v1alpha1.TaskSpec {
	// This assumes that the TaskRun inputs have been validated against what the Task requests.
	replacements := map[string]string{}
	// Set all the default replacements
	for _, p := range defaults {
		if p.Default != "" {
			replacements[fmt.Sprintf("inputs.params.%s", p.Name)] = p.Default
		}
	}
	// Set and overwrite params with the ones from the TaskRun
	for _, p := range tr.Spec.Inputs.Params {
		replacements[fmt.Sprintf("inputs.params.%s", p.Name)] = p.Value
	}

	return ApplyReplacements(spec, replacements)
}

// ApplyResources applies the templating from values in resources which are referenced in spec as subitems
// of the replacementStr.
func ApplyResources(spec *v1alpha1.TaskSpec, resolvedResources map[string]v1alpha1.PipelineResourceInterface, replacementStr string) *v1alpha1.TaskSpec {
	replacements := map[string]string{}
	for name, r := range resolvedResources {
		for k, v := range r.Replacements() {
			replacements[fmt.Sprintf("%s.resources.%s.%s", replacementStr, name, k)] = v
		}
	}
	return ApplyReplacements(spec, replacements)
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(spec *v1alpha1.TaskSpec, replacements map[string]string) *v1alpha1.TaskSpec {
	spec = spec.DeepCopy()

	// Apply variable expansion to steps fields.
	steps := spec.Steps
	for i := range steps {
		applyContainerReplacements(&steps[i], replacements)
	}

	// Apply variable expansion to containerTemplate fields.
	if spec.ContainerTemplate != nil {
		applyContainerReplacements(spec.ContainerTemplate, replacements)
	}

	// Apply variable expansion to the build's volumes
	for i, v := range spec.Volumes {
		spec.Volumes[i].Name = templating.ApplyReplacements(v.Name, replacements)
		if v.VolumeSource.ConfigMap != nil {
			spec.Volumes[i].ConfigMap.Name = templating.ApplyReplacements(v.ConfigMap.Name, replacements)
		}
		if v.VolumeSource.Secret != nil {
			spec.Volumes[i].Secret.SecretName = templating.ApplyReplacements(v.Secret.SecretName, replacements)
		}
		if v.PersistentVolumeClaim != nil {
			spec.Volumes[i].PersistentVolumeClaim.ClaimName = templating.ApplyReplacements(v.PersistentVolumeClaim.ClaimName, replacements)
		}
	}

	return spec
}

func applyContainerReplacements(container *corev1.Container, replacements map[string]string) {
	container.Name = templating.ApplyReplacements(container.Name, replacements)
	container.Image = templating.ApplyReplacements(container.Image, replacements)
	for ia, a := range container.Args {
		container.Args[ia] = templating.ApplyReplacements(a, replacements)
	}
	for ie, e := range container.Env {
		container.Env[ie].Value = templating.ApplyReplacements(e.Value, replacements)
		if container.Env[ie].ValueFrom != nil {
			if e.ValueFrom.SecretKeyRef != nil {
				container.Env[ie].ValueFrom.SecretKeyRef.LocalObjectReference.Name = templating.ApplyReplacements(e.ValueFrom.SecretKeyRef.LocalObjectReference.Name, replacements)
				container.Env[ie].ValueFrom.SecretKeyRef.Key = templating.ApplyReplacements(e.ValueFrom.SecretKeyRef.Key, replacements)
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				container.Env[ie].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = templating.ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, replacements)
				container.Env[ie].ValueFrom.ConfigMapKeyRef.Key = templating.ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.Key, replacements)
			}
		}
	}
	for ie, e := range container.EnvFrom {
		container.EnvFrom[ie].Prefix = templating.ApplyReplacements(e.Prefix, replacements)
		if e.ConfigMapRef != nil {
			container.EnvFrom[ie].ConfigMapRef.LocalObjectReference.Name = templating.ApplyReplacements(e.ConfigMapRef.LocalObjectReference.Name, replacements)
		}
		if e.SecretRef != nil {
			container.EnvFrom[ie].SecretRef.LocalObjectReference.Name = templating.ApplyReplacements(e.SecretRef.LocalObjectReference.Name, replacements)
		}
	}
	container.WorkingDir = templating.ApplyReplacements(container.WorkingDir, replacements)
	for ic, c := range container.Command {
		container.Command[ic] = templating.ApplyReplacements(c, replacements)
	}
	for iv, v := range container.VolumeMounts {
		container.VolumeMounts[iv].Name = templating.ApplyReplacements(v.Name, replacements)
		container.VolumeMounts[iv].MountPath = templating.ApplyReplacements(v.MountPath, replacements)
		container.VolumeMounts[iv].SubPath = templating.ApplyReplacements(v.SubPath, replacements)
	}
}
