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
	"net/url"

	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
)

// Validate pipelinerun
func (pr *PipelineRun) Validate() *apis.FieldError {
	if err := validateObjectMetadata(pr.GetObjectMeta()).ViaField("metadata"); err != nil {
		return err
	}
	return pr.Spec.Validate()
}

// Validate pipelinerun spec
func (ps *PipelineRunSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(ps, &PipelineRunSpec{}) {
		return apis.ErrMissingField("spec")
	}
	// pipeline reference should be present for pipelinerun
	if ps.PipelineRef.Name == "" {
		return apis.ErrMissingField("pipelinerun.spec.Pipelineref.Name")
	}
	if ps.PipelineTriggerRef.Type != PipelineTriggerTypeManual {
		return apis.ErrInvalidValue(string(ps.PipelineTriggerRef.Type), "pipelinerun.spec.triggerRef.type")
	}

	if ps.Results != nil {
		// Results.Runs should have a valid URL and ResultTargetType
		if err := validateURL(ps.Results.Runs.URL, "pipelinerun.spec.Results.Runs.URL"); err != nil {
			return err
		}
		if err := validateResultTargetType(ps.Results.Runs.Type, "pipelinerun.spec.Results.Runs.Type"); err != nil {
			return err
		}

		// Results.Logs should have a valid URL and ResultTargetType
		if err := validateURL(ps.Results.Logs.URL, "pipelinerun.spec.Results.Logs.URL"); err != nil {
			return err
		}
		if err := validateResultTargetType(ps.Results.Logs.Type, "pipelinerun.spec.Results.Logs.Type"); err != nil {
			return err
		}

		// If Results.Tests exists, it should have a valid URL and ResultTargetType
		if ps.Results.Tests != nil {
			if err := validateURL(ps.Results.Tests.URL, "pipelinerun.spec.Results.Tests.URL"); err != nil {
				return err
			}
			if err := validateResultTargetType(ps.Results.Tests.Type, "pipelinerun.spec.Results.Tests.Type"); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateURL(u, path string) *apis.FieldError {
	if u == "" {
		return nil
	}
	_, err := url.ParseRequestURI(u)
	if err != nil {
		return apis.ErrInvalidValue(u, path)
	}
	return nil
}

func validateResultTargetType(r ResultTargetType, path string) *apis.FieldError {
	for _, a := range AllResultTargetTypes {
		if a == r {
			return nil
		}
	}
	return apis.ErrInvalidValue(string(r), path)
}
