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

package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (t *TaskRun) Validate() *apis.FieldError {
	if err := validateObjectMetadata(t.GetObjectMeta()).ViaField("metadata"); err != nil {
		return err
	}
	return t.Spec.Validate()
}

func (ts *TaskRunSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(ts, &TaskRunSpec{}) {
		return apis.ErrMissingField("spec")
	}

	// Check for TaskRef
	if ts.TaskRef.Name == "" {
		return apis.ErrMissingField("spec.taskref.name")
	}

	// Check for Trigger
	if err := validateTaskTriggerType(ts.Trigger.TriggerRef, "spec.trigger.triggerref.type"); err != nil {
		return err
	}

	// check for input resources
	if err := checkForPipelineResourceDuplicates(ts.Inputs.Resources, "spec.Inputs.Resources.Name"); err != nil {
		return err
	}
	if err := validateParameters(ts.Inputs.Params); err != nil {
		return err
	}

	// check for outputs
	for _, source := range ts.Outputs.Resources {
		if err := validateResourceType(source, fmt.Sprintf("spec.Outputs.Resources.%s.Type", source.Name)); err != nil {
			return err
		}
		if err := checkForDuplicates(ts.Outputs.Resources, "spec.Outputs.Resources.Name"); err != nil {
			return err
		}
	}

	// check for results
	if err := validateResultTarget(ts.Results.Logs, "spec.results.logs"); err != nil {
		return err
	}
	if err := validateResultTarget(ts.Results.Runs, "spec.results.runs"); err != nil {
		return err
	}
	if ts.Results.Tests != nil {
		if err := validateResultTarget(*ts.Results.Tests, "spec.results.tests"); err != nil {
			return err
		}
	}

	return nil
}

func validateResultTarget(r ResultTarget, path string) *apis.FieldError {
	// if result target is not set then do not error
	var emptyTarget = ResultTarget{}
	if r == emptyTarget {
		return nil
	}
	// If set then verify all variables pass the validation
	if r.Name == "" {
		return apis.ErrMissingField(fmt.Sprintf("%s.name", path))
	}

	if r.Type != ResultTargetTypeGCS {
		return apis.ErrInvalidValue(string(r.Type), fmt.Sprintf("%s.Type", path))
	}

	if r.URL == "" {
		return apis.ErrMissingField(fmt.Sprintf("%s.URL", path))
	}
	return nil
}

func checkForPipelineResourceDuplicates(resources []PipelineResourceVersion, path string) *apis.FieldError {
	encountered := map[string]struct{}{}
	for _, r := range resources {
		// Check the unique combination of resource+version. Covers the use case of inputs with same resource name
		// and different versions
		key := fmt.Sprintf("%s%s", r.ResourceRef.Name, r.Version)
		if _, ok := encountered[strings.ToLower(key)]; ok {
			return apis.ErrMultipleOneOf(path)
		}
		encountered[key] = struct{}{}
	}
	return nil
}

func validateTaskTriggerType(r TaskTriggerRef, path string) *apis.FieldError {
	if r.Type == "" {
		return nil
	}
	for _, allowed := range []TaskTriggerType{TaskTriggerTypePipelineRun, TaskTriggerTypeManual} {
		if strings.ToLower(string(r.Type)) == strings.ToLower(string(allowed)) {
			return nil
		}
	}
	return apis.ErrInvalidValue(string(r.Type), path)
}

func validateParameters(params []Param) *apis.FieldError {
	// Template must not duplicate parameter names.
	seen := map[string]struct{}{}
	for _, p := range params {
		if _, ok := seen[strings.ToLower(p.Name)]; ok {
			return apis.ErrMultipleOneOf("spec.inputs.params")
		}
		seen[p.Name] = struct{}{}
	}
	return nil
}
