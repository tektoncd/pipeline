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

package v1beta1

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

func validateRef(ctx context.Context, refName string, refResolver ResolverName, refParams Params) (errs *apis.FieldError) {
	switch {
	case refResolver != "" || refParams != nil:
		if refParams != nil {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "resolver params", config.BetaAPIFields).ViaField("params"))
			if refName != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "params"))
			}
			if refResolver == "" {
				errs = errs.Also(apis.ErrMissingField("resolver"))
			}
			errs = errs.Also(ValidateParameters(ctx, refParams))
		}
		if refResolver != "" {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "resolver", config.BetaAPIFields).ViaField("resolver"))
			if refName != "" {
				// make sure that the name is url-like.
				err := RefNameLikeUrl(refName)
				if err == nil && !config.FromContextOrDefaults(ctx).FeatureFlags.EnableConciseResolverSyntax {
					// If name is url-like then concise resolver syntax must be enabled
					errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use concise resolver syntax", config.EnableConciseResolverSyntax), ""))
				}
				if err != nil {
					errs = errs.Also(apis.ErrInvalidValue(err, "name"))
				}
			}
		}
	case refName != "":
		// ref name can be a Url-like format.
		if err := RefNameLikeUrl(refName); err == nil {
			// If name is url-like then concise resolver syntax must be enabled
			if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableConciseResolverSyntax {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use concise resolver syntax", config.EnableConciseResolverSyntax), ""))
			}
			// In stage1 of concise remote resolvers syntax, this is a required field.
			// TODO: remove this check when implementing stage 2 where this is optional.
			if refResolver == "" {
				errs = errs.Also(apis.ErrMissingField("resolver"))
			}
			// Or, it must be a valid k8s name
		} else {
			// ref name must be a valid k8s name
			if errSlice := validation.IsQualifiedName(refName); len(errSlice) != 0 {
				errs = errs.Also(apis.ErrInvalidValue(strings.Join(errSlice, ","), "name"))
			}
		}
	default:
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	return errs
}

// Validate ensures that a supplied Ref field is populated
// correctly. No errors are returned for a nil Ref.
func (ref *Ref) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ref == nil {
		return errs
	}
	return validateRef(ctx, ref.Name, ref.Resolver, ref.Params)
}

// RefNameLikeUrl checks if the name is url parsable and returns an error if it isn't.
func RefNameLikeUrl(name string) error {
	schemeRegex := regexp.MustCompile(`[\w-]+:\/\/*`)
	if !schemeRegex.MatchString(name) {
		return errors.New("invalid URI for request")
	}
	return nil
}
