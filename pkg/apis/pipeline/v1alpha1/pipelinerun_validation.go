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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*PipelineRun)(nil)

// Validate pipelinerun
func (pr *PipelineRun) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(pr.GetObjectMeta()).ViaField("metadata"); err != nil {
		return err
	}
	return pr.Spec.Validate(ctx)
}

// Validate pipelinerun spec
func (ps *PipelineRunSpec) Validate(ctx context.Context) *apis.FieldError {
	if equality.Semantic.DeepEqual(ps, &PipelineRunSpec{}) {
		return apis.ErrMissingField("spec")
	}

	// can't have both pipelineRef and pipelineSpec at the same time
	if (ps.PipelineRef != nil && ps.PipelineRef.Name != "") && ps.PipelineSpec != nil {
		return apis.ErrDisallowedFields("spec.pipelineref", "spec.pipelinespec")
	}

	// Check that one of PipelineRef and PipelineSpec is present
	if (ps.PipelineRef == nil || (ps.PipelineRef != nil && ps.PipelineRef.Name == "")) && ps.PipelineSpec == nil {
		return apis.ErrMissingField("spec.pipelineref.name", "spec.pipelinespec")
	}

	// Validate PipelineSpec if it's present
	if ps.PipelineSpec != nil {
		if err := ps.PipelineSpec.Validate(ctx); err != nil {
			return err
		}
	}

	if ps.Timeout != nil {
		// timeout should be a valid duration of at least 0.
		if ps.Timeout.Duration < 0 {
			return apis.ErrInvalidValue(fmt.Sprintf("%s should be >= 0", ps.Timeout.Duration.String()), "spec.timeout")
		}
	}

	if ps.Workspaces != nil {
		wsNames := make(map[string]int)
		for idx, ws := range ps.Workspaces {
			if prevIdx, alreadyExists := wsNames[ws.Name]; alreadyExists {
				return &apis.FieldError{
					Message: fmt.Sprintf("workspace %q provided by pipelinerun more than once, at index %d and %d", ws.Name, prevIdx, idx),
					Paths:   []string{"spec.workspaces"},
				}
			}
			wsNames[ws.Name] = idx
		}
	}

	return nil
}
