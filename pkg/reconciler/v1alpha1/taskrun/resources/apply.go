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
	"strings"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
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

// ResourceGetter is the interface used to retrieve resources which are references via a TaskRunResource.
type ResourceGetter interface {
	Get(string) (*v1alpha1.PipelineResource, error)
}

// ApplyResources applies the templating from values in resources which are referenced in b as subitems
// of the replacementStr. It retrieves the referenced resources via the getter.
func ApplyResources(b *buildv1alpha1.Build, resources []v1alpha1.TaskResourceBinding, getter ResourceGetter, replacementStr string) (*buildv1alpha1.Build, error) {
	replacements := map[string]string{}

	for _, r := range resources {
		pr, err := getter.Get(r.ResourceRef.Name)
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

	applyReplacements := func(in string) string {
		for k, v := range replacements {
			in = strings.Replace(in, fmt.Sprintf("${%s}", k), v, -1)
		}
		return in
	}

	// Apply variable expansion to steps fields.
	steps := build.Spec.Steps
	for i := range steps {
		steps[i].Name = applyReplacements(steps[i].Name)
		steps[i].Image = applyReplacements(steps[i].Image)
		for ia, a := range steps[i].Args {
			steps[i].Args[ia] = applyReplacements(a)
		}
		for ie, e := range steps[i].Env {
			steps[i].Env[ie].Value = applyReplacements(e.Value)
		}
		steps[i].WorkingDir = applyReplacements(steps[i].WorkingDir)
		for ic, c := range steps[i].Command {
			steps[i].Command[ic] = applyReplacements(c)
		}
		for iv, v := range steps[i].VolumeMounts {
			steps[i].VolumeMounts[iv].Name = applyReplacements(v.Name)
			steps[i].VolumeMounts[iv].MountPath = applyReplacements(v.MountPath)
			steps[i].VolumeMounts[iv].SubPath = applyReplacements(v.SubPath)
		}
	}
	return build
}
