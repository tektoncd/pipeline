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
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/templating"
	corev1 "k8s.io/api/core/v1"
)

// ApplyParameters applies the params from a TaskRun.Input.Parameters to a TaskSpec
func ApplyParameters(spec *v1alpha1.TaskSpec, tr *v1alpha1.TaskRun, defaults ...v1alpha1.ParamSpec) *v1alpha1.TaskSpec {
	// This assumes that the TaskRun inputs have been validated against what the Task requests.

	// stringReplacements is used for standard single-string stringReplacements, while arrayReplacements contains arrays
	// that need to be further processed.
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}

	// Set all the default stringReplacements
	for _, p := range defaults {
		if p.Default != nil {
			if p.Default.Type == v1alpha1.ParamTypeString {
				stringReplacements[fmt.Sprintf("inputs.params.%s", p.Name)] = p.Default.StringVal
			} else {
				arrayReplacements[fmt.Sprintf("inputs.params.%s", p.Name)] = p.Default.ArrayVal
			}
		}
	}
	// Set and overwrite params with the ones from the TaskRun
	for _, p := range tr.Spec.Inputs.Params {
		if p.Value.Type == v1alpha1.ParamTypeString {
			stringReplacements[fmt.Sprintf("inputs.params.%s", p.Name)] = p.Value.StringVal
		} else {
			arrayReplacements[fmt.Sprintf("inputs.params.%s", p.Name)] = p.Value.ArrayVal
		}
	}

	return ApplyReplacements(spec, stringReplacements, arrayReplacements)
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
	return ApplyReplacements(spec, replacements, map[string][]string{})
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(spec *v1alpha1.TaskSpec, stringReplacements map[string]string, arrayReplacements map[string][]string) *v1alpha1.TaskSpec {
	spec = spec.DeepCopy()

	// Apply variable expansion to steps fields.
	steps := spec.Steps
	for i := range steps {
		applyContainerReplacements(&steps[i], stringReplacements, arrayReplacements)
	}

	// Apply variable expansion to containerTemplate fields.
	// Should eventually be removed; ContainerTemplate is the deprecated previous name of the StepTemplate field (#977).
	if spec.ContainerTemplate != nil {
		applyContainerReplacements(spec.ContainerTemplate, stringReplacements, arrayReplacements)
	}

	// Apply variable expansion to stepTemplate fields.
	if spec.StepTemplate != nil {
		applyContainerReplacements(spec.StepTemplate, stringReplacements, arrayReplacements)
	}

	// Apply variable expansion to the build's volumes
	for i, v := range spec.Volumes {
		spec.Volumes[i].Name = templating.ApplyReplacements(v.Name, stringReplacements)
		if v.VolumeSource.ConfigMap != nil {
			spec.Volumes[i].ConfigMap.Name = templating.ApplyReplacements(v.ConfigMap.Name, stringReplacements)
		}
		if v.VolumeSource.Secret != nil {
			spec.Volumes[i].Secret.SecretName = templating.ApplyReplacements(v.Secret.SecretName, stringReplacements)
		}
		if v.PersistentVolumeClaim != nil {
			spec.Volumes[i].PersistentVolumeClaim.ClaimName = templating.ApplyReplacements(v.PersistentVolumeClaim.ClaimName, stringReplacements)
		}
	}

	return spec
}

func applyContainerReplacements(container *corev1.Container, stringReplacements map[string]string, arrayReplacements map[string][]string) {
	container.Name = templating.ApplyReplacements(container.Name, stringReplacements)
	container.Image = templating.ApplyReplacements(container.Image, stringReplacements)

	//Use ApplyArrayReplacements here, as additional args may be added via an array parameter.
	var newArgs []string
	for _, a := range container.Args {
		newArgs = append(newArgs, templating.ApplyArrayReplacements(a, stringReplacements, arrayReplacements)...)
	}
	container.Args = newArgs

	for ie, e := range container.Env {
		container.Env[ie].Value = templating.ApplyReplacements(e.Value, stringReplacements)
		if container.Env[ie].ValueFrom != nil {
			if e.ValueFrom.SecretKeyRef != nil {
				container.Env[ie].ValueFrom.SecretKeyRef.LocalObjectReference.Name = templating.ApplyReplacements(e.ValueFrom.SecretKeyRef.LocalObjectReference.Name, stringReplacements)
				container.Env[ie].ValueFrom.SecretKeyRef.Key = templating.ApplyReplacements(e.ValueFrom.SecretKeyRef.Key, stringReplacements)
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				container.Env[ie].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = templating.ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, stringReplacements)
				container.Env[ie].ValueFrom.ConfigMapKeyRef.Key = templating.ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.Key, stringReplacements)
			}
		}
	}

	for ie, e := range container.EnvFrom {
		container.EnvFrom[ie].Prefix = templating.ApplyReplacements(e.Prefix, stringReplacements)
		if e.ConfigMapRef != nil {
			container.EnvFrom[ie].ConfigMapRef.LocalObjectReference.Name = templating.ApplyReplacements(e.ConfigMapRef.LocalObjectReference.Name, stringReplacements)
		}
		if e.SecretRef != nil {
			container.EnvFrom[ie].SecretRef.LocalObjectReference.Name = templating.ApplyReplacements(e.SecretRef.LocalObjectReference.Name, stringReplacements)
		}
	}
	container.WorkingDir = templating.ApplyReplacements(container.WorkingDir, stringReplacements)

	//Use ApplyArrayReplacements here, as additional commands may be added via an array parameter.
	var newCommand []string
	for _, c := range container.Command {
		newCommand = append(newCommand, templating.ApplyArrayReplacements(c, stringReplacements, arrayReplacements)...)
	}
	container.Command = newCommand

	for iv, v := range container.VolumeMounts {
		container.VolumeMounts[iv].Name = templating.ApplyReplacements(v.Name, stringReplacements)
		container.VolumeMounts[iv].MountPath = templating.ApplyReplacements(v.MountPath, stringReplacements)
		container.VolumeMounts[iv].SubPath = templating.ApplyReplacements(v.SubPath, stringReplacements)
	}
}
