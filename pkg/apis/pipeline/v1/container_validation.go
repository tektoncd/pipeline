/*
Copyright 2023 The Tekton Authors

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
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

// Validate ensures that a supplied Ref field is populated
// correctly. No errors are returned for a nil Ref.
func (ref *Ref) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ref == nil {
		return
	}

	switch {
	case ref.Resolver != "" || ref.Params != nil:
		if ref.Resolver != "" {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "resolver", config.BetaAPIFields).ViaField("resolver"))
			if ref.Name != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "resolver"))
			}
		}
		if ref.Params != nil {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "resolver params", config.BetaAPIFields).ViaField("params"))
			if ref.Name != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "params"))
			}
			if ref.Resolver == "" {
				errs = errs.Also(apis.ErrMissingField("resolver"))
			}
			errs = errs.Also(ValidateParameters(ctx, ref.Params))
		}
	case ref.Name != "":
		// ref name must be a valid k8s name
		if errSlice := validation.IsQualifiedName(ref.Name); len(errSlice) != 0 {
			errs = errs.Also(apis.ErrInvalidValue(strings.Join(errSlice, ","), "name"))
		}
	default:
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	return errs
}
