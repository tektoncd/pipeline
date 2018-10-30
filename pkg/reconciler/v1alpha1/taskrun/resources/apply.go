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

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/build/pkg/builder"
)

// ApplyParameters applies the params from a TaskRun.Input.Parameters to a BuildSpec.
func ApplyParameters(b *buildv1alpha1.Build, tr *v1alpha1.TaskRun) *buildv1alpha1.Build {
	// This assumes that the TaskRun inputs have been validated against what the Task requests.
	replacements := map[string]string{}
	for _, p := range tr.Spec.Inputs.Params {
		replacements[fmt.Sprintf("inputs.params.%s", p.Name)] = p.Value
	}

	return builder.ApplyReplacements(b, replacements)
}

// ResourceGetter is the interface used to refreive resources which are references via a TaskRunResourceVersion.
type ResourceGetter interface {
	Get(string) (*v1alpha1.PipelineResource, error)
}

// ApplyResources applies the templating from values in resources which are referenced in b as subitems
// of the replacementStr. It retrieves the referenced resources via the getter.
func ApplyResources(b *buildv1alpha1.Build, resources []v1alpha1.TaskRunResourceVersion, getter ResourceGetter, replacementStr string) (*buildv1alpha1.Build, error) {
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
	return builder.ApplyReplacements(b, replacements), nil
}
