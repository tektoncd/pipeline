/*
Copyright 2020 The Tekton Authors

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

package v1beta1

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"

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

	// Check that if a bundle is specified, that a PipelineRef is specified as well.
	if (ps.PipelineRef != nil && ps.PipelineRef.Bundle != "") && ps.PipelineRef.Name == "" {
		return apis.ErrMissingField("spec.pipelineref.name")
	}

	// If a bundle url is specified, ensure it is parseable.
	if ps.PipelineRef != nil && ps.PipelineRef.Bundle != "" {
		if _, err := name.ParseReference(ps.PipelineRef.Bundle); err != nil {
			return apis.ErrInvalidValue(fmt.Sprintf("invalid bundle reference (%s)", err.Error()), "spec.pipelineref.bundle")
		}
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

	if ps.Status != "" {
		if ps.Status != PipelineRunSpecStatusCancelled {
			return apis.ErrInvalidValue(fmt.Sprintf("%s should be %s", ps.Status, PipelineRunSpecStatusCancelled), "spec.status")
		}
	}

	if ps.Workspaces != nil {
		wsNames := make(map[string]int)
		for idx, ws := range ps.Workspaces {
			field := fmt.Sprintf("spec.workspaces[%d]", idx)
			if err := ws.Validate(ctx).ViaField(field); err != nil {
				return err
			}
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
