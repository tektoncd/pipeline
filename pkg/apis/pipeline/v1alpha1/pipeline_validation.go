/*
Copyright 2018 The Knative Authors

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

	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/list"
	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
)

// Validate checks that the Pipeline structure is valid but does not validate
// that any references resources exist, that is done at run time.
func (p *Pipeline) Validate() *apis.FieldError {
	if err := validateObjectMetadata(p.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	return nil
}

func isOutput(task PipelineTask, resource string) bool {
	for _, output := range task.Resources.Outputs {
		if output.Resource == resource {
			return true
		}
	}
	return false
}

func validateDeclaredResources(ps *PipelineSpec) error {
	needed := []string{}
	for _, t := range ps.Tasks {
		if t.Resources != nil {
			for _, input := range t.Resources.Inputs {
				needed = append(needed, input.Resource)
			}
			for _, output := range t.Resources.Outputs {
				needed = append(needed, output.Resource)
			}
		}
	}

	provided := make([]string, 0, len(ps.Resources))
	for _, resource := range ps.Resources {
		provided = append(provided, resource.Name)
	}
	err := list.IsSame(needed, provided)
	if err != nil {
		return fmt.Errorf("Pipeline declared resources didn't match usage in Tasks: %s", err)
	}
	return nil
}

// Validate checks that taskNames in the Pipeline are valid and that the graph
// of Tasks expressed in the Pipeline makes sense.
func (ps *PipelineSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(ps, &PipelineSpec{}) {
		return apis.ErrMissingField(apis.CurrentField)
	}

	// Names cannot be duplicated
	taskNames := map[string]struct{}{}
	for _, t := range ps.Tasks {
		if _, ok := taskNames[t.Name]; ok {
			return apis.ErrMultipleOneOf("spec.tasks.name")
		}
		taskNames[t.Name] = struct{}{}
	}

	// All declared resources should be used, and the Pipeline shouldn't try to use any resources
	// that aren't declared
	if err := validateDeclaredResources(ps); err != nil {
		return apis.ErrInvalidValue(err.Error(), "spec.resources")
	}

	// providedBy should match future tasks
	// TODO(#168) when pipelines don't just execute linearly this will need to be more sophisticated
	for i, t := range ps.Tasks {
		if t.Resources != nil {
			for _, rd := range t.Resources.Inputs {
				for _, pb := range rd.ProvidedBy {
					if i == 0 {
						// First Task can't depend on anything before it (b/c there is nothing)
						return apis.ErrInvalidValue(pb, "spec.tasks.resources.inputs.providedBy")
					}
					found := false
					// Look for previous Task that satisfies constraint
					for j := i - 1; j >= 0; j-- {
						if ps.Tasks[j].Name == pb {
							// The input resource must actually be an output of the providedBy tasks
							if !isOutput(ps.Tasks[j], rd.Resource) {
								return apis.ErrInvalidKeyName(pb, "spec.tasks.resources.inputs.providedBy")
							}
							found = true
						}
					}
					if !found {
						return apis.ErrInvalidKeyName(pb, "spec.tasks.resources.inputs.providedBy")
					}
				}
			}
		}
	}
	return nil
}
