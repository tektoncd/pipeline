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
	"github.com/tektoncd/pipeline/pkg/apis/version"
	"knative.dev/pkg/apis"
)

// Validate ensures that a supplied TaskRef field is populated
// correctly. No errors are returned for a nil TaskRef.
func (ref *TaskRef) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ref == nil {
		return
	}

	switch {
	case ref.Resolver != "":
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "resolver", config.AlphaAPIFields).ViaField("resolver"))
		if ref.Name != "" {
			errs = errs.Also(apis.ErrMultipleOneOf("name", "resolver"))
		}
		if ref.Bundle != "" {
			errs = errs.Also(apis.ErrMultipleOneOf("bundle", "resolver"))
		}
	case ref.Resource != nil:
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "resource", config.AlphaAPIFields).ViaField("resource"))
		if ref.Name != "" {
			errs = errs.Also(apis.ErrMultipleOneOf("name", "resource"))
		}
		if ref.Bundle != "" {
			errs = errs.Also(apis.ErrMultipleOneOf("bundle", "resource"))
		}
		if ref.Resolver == "" {
			errs = errs.Also(apis.ErrMissingField("resolver"))
		}
	case ref.Name == "":
		errs = errs.Also(apis.ErrMissingField("name"))
	case ref.Bundle != "":
		errs = errs.Also(validateBundleFeatureFlag(ctx, "bundle", true).ViaField("bundle"))
		if _, err := name.ParseReference(ref.Bundle); err != nil {
			errs = errs.Also(apis.ErrInvalidValue("invalid bundle reference", "bundle", err.Error()))
		}
	}
	return
}
