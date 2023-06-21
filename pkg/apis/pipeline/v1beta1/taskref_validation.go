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
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

// Validate ensures that a supplied TaskRef field is populated
// correctly. No errors are returned for a nil TaskRef.
func (ref *TaskRef) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ref == nil {
		return
	}

	switch {
	case ref.Resolver != "" || ref.Params != nil:
		if ref.Resolver != "" {
			if ref.Name != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "resolver"))
			}
			if ref.Bundle != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("bundle", "resolver"))
			}
		}
		if ref.Params != nil {
			if ref.Name != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "params"))
			}
			if ref.Bundle != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("bundle", "params"))
			}
			if ref.Resolver == "" {
				errs = errs.Also(apis.ErrMissingField("resolver"))
			}
			errs = errs.Also(ValidateParameters(ctx, ref.Params))
		}
	case ref.Bundle != "":
		if ref.Name == "" {
			errs = errs.Also(apis.ErrMissingField("name"))
		}
		errs = errs.Also(validateBundleFeatureFlag(ctx, "bundle", true).ViaField("bundle"))
		if _, err := name.ParseReference(ref.Bundle); err != nil {
			errs = errs.Also(apis.ErrInvalidValue("invalid bundle reference", "bundle", err.Error()))
		}
	default:
		if ref.Name == "" {
			errs = errs.Also(apis.ErrMissingField("name"))
		} else if errSlice := validation.IsQualifiedName(ref.Name); len(errSlice) != 0 {
			// TaskRef name must be a valid k8s name
			errs = errs.Also(apis.ErrInvalidValue(strings.Join(errSlice, ","), "name"))
		}
	}
	return errs
}
