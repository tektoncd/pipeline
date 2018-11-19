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

	// providedBy should match other tasks.
	for _, t := range ps.Tasks {
		for _, rd := range t.ResourceDependencies {
			for _, pb := range rd.ProvidedBy {
				if _, ok := taskNames[pb]; !ok {
					return apis.ErrInvalidKeyName(pb, fmt.Sprintf("spec.tasks.resources.%s", pb))
				}
			}
		}
	}
	return nil
}
