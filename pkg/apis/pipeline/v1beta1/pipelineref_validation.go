/*
Copyright 2022 The Tekton Authors

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

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"knative.dev/pkg/apis"
)

// Validate ensures that a supplied PipelineRef field is populated
// correctly. No errors are returned for a nil PipelineRef.
func (ref *PipelineRef) Validate(ctx context.Context) (errs *apis.FieldError) {
	cfg := config.FromContextOrDefaults(ctx)
	if ref == nil {
		return
	}
	if cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields {
		errs = errs.Also(ref.validateAlphaRef(ctx))
	} else {
		errs = errs.Also(ref.validateInTreeRef(ctx))
	}
	return
}

// validateInTreeRef returns errors if the given pipelineRef is not
// valid for Pipelines' built-in resolution machinery.
func (ref *PipelineRef) validateInTreeRef(ctx context.Context) (errs *apis.FieldError) {
	cfg := config.FromContextOrDefaults(ctx)
	if ref.Resolver != "" {
		errs = errs.Also(apis.ErrDisallowedFields("resolver"))
	}
	if ref.Resource != nil {
		errs = errs.Also(apis.ErrDisallowedFields("resource"))
	}
	if ref.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	if cfg.FeatureFlags.EnableTektonOCIBundles {
		if ref.Bundle != "" && ref.Name == "" {
			errs = errs.Also(apis.ErrMissingField("name"))
		}
		if ref.Bundle != "" {
			if _, err := name.ParseReference(ref.Bundle); err != nil {
				errs = errs.Also(apis.ErrInvalidValue("invalid bundle reference", "bundle", err.Error()))
			}
		}
	} else if ref.Bundle != "" {
		errs = errs.Also(apis.ErrDisallowedFields("bundle"))
	}
	return
}

// validateAlphaRef ensures that the user has passed either a
// valid remote resource reference or a valid in-tree resource reference,
// but not both.
func (ref *PipelineRef) validateAlphaRef(ctx context.Context) (errs *apis.FieldError) {
	switch {
	case ref.Resolver == "" && ref.Resource != nil:
		errs = errs.Also(apis.ErrMissingField("resolver"))
	case ref.Resolver == "":
		errs = errs.Also(ref.validateInTreeRef(ctx))
	default:
		if ref.Name != "" {
			errs = errs.Also(apis.ErrMultipleOneOf("name", "resolver"))
		}
		if ref.Bundle != "" {
			errs = errs.Also(apis.ErrMultipleOneOf("bundle", "resolver"))
		}
	}
	return
}
