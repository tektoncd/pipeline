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

func (p *Pipeline) Validate() *apis.FieldError {
	if err := validateObjectMetadata(p.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	return nil
}

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

	// passedConstraints should match other tasks.
	for _, t := range ps.Tasks {
		for _, isb := range t.InputSourceBindings {
			for _, pc := range isb.PassedConstraints {
				if _, ok := taskNames[pc]; !ok {
					return apis.ErrInvalidKeyName(pc, fmt.Sprintf("spec.tasks.inputSourceBindings.%s", pc))
				}
			}
		}
	}
	return nil
}
