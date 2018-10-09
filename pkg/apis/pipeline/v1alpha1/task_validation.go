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

func (t *Task) Validate() *apis.FieldError {
	if err := validateObjectMetadata(t.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	return t.Spec.Validate()
}

func (ts *TaskSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(ts, &TaskSpec{}) {
		return apis.ErrMissingField(apis.CurrentField)
	}

	// A task doesn't have to have inputs or outputs, but if it does they must be valid.
	// A task can't duplicate input or output names.

	if ts.Inputs != nil {
		for _, source := range ts.Inputs.Sources {
			if err := validateResourceType(source, fmt.Sprintf("taskspec.Inputs.Sources.%s.Type", source.Name)); err != nil {
				return err
			}
		}
		if err := checkForDuplicates(ts.Inputs.Sources, "taskspec.Inputs.Sources.Name"); err != nil {
			return err
		}
	}
	if ts.Outputs != nil {
		for _, source := range ts.Outputs.Sources {
			if err := validateResourceType(source, fmt.Sprintf("taskspec.Outputs.Sources.%s.Type", source.Name)); err != nil {
				return err
			}
			if err := checkForDuplicates(ts.Outputs.Sources, "taskspec.Outputs.Sources.Name"); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkForDuplicates(sources []Source, path string) *apis.FieldError {
	encountered := map[string]struct{}{}
	for _, s := range sources {
		if _, ok := encountered[s.Name]; ok {
			return apis.ErrMultipleOneOf(path)
		}
		encountered[s.Name] = struct{}{}
	}
	return nil
}

func validateResourceType(s Source, path string) *apis.FieldError {
	for _, allowed := range AllResourceTypes {
		if s.Type == allowed {
			return nil
		}
	}
	return apis.ErrInvalidValue(string(s.Type), path)
}
