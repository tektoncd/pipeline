/*
Copyright 2018 The Knative Authors.

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

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/templating"
)

// ApplyParameters applies the params from a TaskRun.Input.Parameters to a BuildSpec.
func ApplyParameters(b *buildv1alpha1.Build, tr *v1alpha1.TaskRun, defaults ...v1alpha1.TaskParam) *buildv1alpha1.Build {
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

	return ApplyReplacements(b, replacements)
}

// ApplyResources applies the templating from values in resources which are referenced in b as subitems
// of the replacementStr. It retrieves the referenced resources via the getter.
func ApplyResources(b *buildv1alpha1.Build, resources []v1alpha1.TaskResourceBinding, getter GetResource, replacementStr string) (*buildv1alpha1.Build, error) {
	replacements := map[string]string{}

	for _, r := range resources {
		pr, err := getResource(&r, getter)
		if err != nil {
			return nil, err
		}

		resource, err := v1alpha1.ResourceFromType(pr)
		if err != nil {
			return nil, err
		}
		for k, v := range resource.Replacements() {
			replacements[fmt.Sprintf("%s.resources.%s.%s", replacementStr, r.Name, k)] = v
		}
	}
	return ApplyReplacements(b, replacements), nil
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(build *buildv1alpha1.Build, replacements map[string]string) *buildv1alpha1.Build {
	build = build.DeepCopy()

	// Apply variable expansion to steps fields.
	steps := build.Spec.Steps
	for i := range steps {
		steps[i].Name = templating.ApplyReplacements(steps[i].Name, replacements)
		steps[i].Image = templating.ApplyReplacements(steps[i].Image, replacements)
		for ia, a := range steps[i].Args {
			steps[i].Args[ia] = templating.ApplyReplacements(a, replacements)
		}
		for ie, e := range steps[i].Env {
			steps[i].Env[ie].Value = templating.ApplyReplacements(e.Value, replacements)
		}
		steps[i].WorkingDir = templating.ApplyReplacements(steps[i].WorkingDir, replacements)
		for ic, c := range steps[i].Command {
			steps[i].Command[ic] = templating.ApplyReplacements(c, replacements)
		}
		for iv, v := range steps[i].VolumeMounts {
			steps[i].VolumeMounts[iv].Name = templating.ApplyReplacements(v.Name, replacements)
			steps[i].VolumeMounts[iv].MountPath = templating.ApplyReplacements(v.MountPath, replacements)
			steps[i].VolumeMounts[iv].SubPath = templating.ApplyReplacements(v.SubPath, replacements)
		}
	}

	// Apply variable expansion to the build's volumes
	for i, v := range build.Spec.Volumes {
		build.Spec.Volumes[i].Name = templating.ApplyReplacements(v.Name, replacements)
		if v.VolumeSource.ConfigMap != nil {
			build.Spec.Volumes[i].ConfigMap.Name = templating.ApplyReplacements(v.ConfigMap.Name, replacements)
		}
		if v.VolumeSource.Secret != nil {
			build.Spec.Volumes[i].Secret.SecretName = templating.ApplyReplacements(v.Secret.SecretName, replacements)
		}
		if v.PersistentVolumeClaim != nil {
			build.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = templating.ApplyReplacements(v.PersistentVolumeClaim.ClaimName, replacements)
		}
	}

	return build
}
