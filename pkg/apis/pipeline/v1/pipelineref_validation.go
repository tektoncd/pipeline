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

package v1

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/version"
	"knative.dev/pkg/apis"
)

// Validate ensures that a supplied PipelineRef field is populated
// correctly. No errors are returned for a nil PipelineRef.
func (ref *PipelineRef) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ref == nil {
		return
	}

	if ref.Resolver != "" || ref.Params != nil {
		if ref.Resolver != "" {
			errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "resolver", config.BetaAPIFields).ViaField("resolver"))
			if ref.Name != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "resolver"))
			}
		}
		if ref.Params != nil {
			errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "resolver params", config.BetaAPIFields).ViaField("params"))
			if ref.Name != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "params"))
			}
			if ref.Resolver == "" {
				errs = errs.Also(apis.ErrMissingField("resolver"))
			}
			errs = errs.Also(ValidateParameters(ctx, ref.Params))
		}
	} else if ref.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	return
}
