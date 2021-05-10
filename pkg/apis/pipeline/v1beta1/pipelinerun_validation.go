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
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*PipelineRun)(nil)

// Validate pipelinerun
func (pr *PipelineRun) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(pr.GetObjectMeta()).ViaField("metadata")

	if pr.IsPending() && pr.HasStarted() {
		errs = errs.Also(apis.ErrInvalidValue("PipelineRun cannot be Pending after it is started", "spec.status"))
	}

	return errs.Also(pr.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
}

// Validate pipelinerun spec
func (ps *PipelineRunSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	cfg := config.FromContextOrDefaults(ctx)
	// can't have both pipelineRef and pipelineSpec at the same time
	if (ps.PipelineRef != nil && ps.PipelineRef.Name != "") && ps.PipelineSpec != nil {
		errs = errs.Also(apis.ErrDisallowedFields("pipelineref", "pipelinespec"))
	}

	// Check that one of PipelineRef and PipelineSpec is present
	if (ps.PipelineRef == nil || (ps.PipelineRef != nil && ps.PipelineRef.Name == "")) && ps.PipelineSpec == nil {
		errs = errs.Also(apis.ErrMissingField("pipelineref.name", "pipelinespec"))
	}

	// If EnableTektonOCIBundles feature flag is on validate it.
	// Otherwise, fail if it is present (as it won't be allowed nor used)
	if cfg.FeatureFlags.EnableTektonOCIBundles {
		// Check that if a bundle is specified, that a PipelineRef is specified as well.
		if (ps.PipelineRef != nil && ps.PipelineRef.Bundle != "") && ps.PipelineRef.Name == "" {
			errs = errs.Also(apis.ErrMissingField("pipelineref.name"))
		}

		// If a bundle url is specified, ensure it is parseable.
		if ps.PipelineRef != nil && ps.PipelineRef.Bundle != "" {
			if _, err := name.ParseReference(ps.PipelineRef.Bundle); err != nil {
				errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("invalid bundle reference (%s)", err.Error()), "pipelineref.bundle"))
			}
		}
	} else if ps.PipelineRef != nil && ps.PipelineRef.Bundle != "" {
		errs = errs.Also(apis.ErrDisallowedFields("pipelineref.bundle"))
	}

	// Validate PipelineSpec if it's present
	if ps.PipelineSpec != nil {
		errs = errs.Also(ps.PipelineSpec.Validate(ctx).ViaField("pipelinespec"))
	}

	if ps.Timeout != nil {
		// timeout should be a valid duration of at least 0.
		if ps.Timeout.Duration < 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s should be >= 0", ps.Timeout.Duration.String()), "timeout"))
		}
	}

	if ps.Status != "" {
		if ps.Status != PipelineRunSpecStatusCancelled && ps.Status != PipelineRunSpecStatusPending {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s should be %s or %s", ps.Status, PipelineRunSpecStatusCancelled, PipelineRunSpecStatusPending), "status"))
		}
	}

	if ps.Workspaces != nil {
		wsNames := make(map[string]int)
		for idx, ws := range ps.Workspaces {
			errs = errs.Also(ws.Validate(ctx).ViaFieldIndex("workspaces", idx))
			if prevIdx, alreadyExists := wsNames[ws.Name]; alreadyExists {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("workspace %q provided by pipelinerun more than once, at index %d and %d", ws.Name, prevIdx, idx), "name").ViaFieldIndex("workspaces", idx))
			}
			wsNames[ws.Name] = idx
		}
	}

	return errs
}
