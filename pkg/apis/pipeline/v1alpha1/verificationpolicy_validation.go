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

package v1alpha1

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*VerificationPolicy)(nil)

var (
	// InvalidResourcePatternErr is returned when the pattern is not valid regex expression
	InvalidResourcePatternErr = "resourcePattern cannot be compiled by regex"
)

// Validate VerificationPolicy
func (v *VerificationPolicy) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(validate.ObjectMetadata(v.GetObjectMeta()).ViaField("metadata"))
	errs = errs.Also(v.Spec.Validate(ctx))
	return errs
}

// Validate VerificationPolicySpec, the validation requires Resources is not empty, for each
// resource it must be able to be regex expression and can be compiled with no error. The Authorities
// shouldn't be empty and each Authority should be valid.
func (vs *VerificationPolicySpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	if len(vs.Resources) == 0 {
		errs = errs.Also(apis.ErrMissingField("resources"))
	}
	for _, r := range vs.Resources {
		errs = errs.Also(r.Validate(ctx))
	}
	if len(vs.Authorities) == 0 {
		errs = errs.Also(apis.ErrMissingField("authorities"))
	}
	for i, a := range vs.Authorities {
		if a.Key != nil {
			errs = errs.Also(a.Key.Validate(ctx).ViaFieldIndex("key", i))
		}
	}
	if vs.Mode != "" && vs.Mode != ModeEnforce && vs.Mode != ModeWarn {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("available values are: %s, %s, but got: %s", ModeEnforce, ModeWarn, vs.Mode), "mode"))
	}
	return errs
}

// Validate KeyRef will check if one of KeyRef's Data or SecretRef exists, and the
// Supported HashAlgorithm is in supportedSignatureAlgorithms.
func (key *KeyRef) Validate(ctx context.Context) (errs *apis.FieldError) {
	// Validate that one and only one of Data, SecretRef, KMS is defined.
	keyCount := 0
	if key.Data != "" {
		keyCount++
	}
	if key.SecretRef != nil {
		keyCount++
	}
	if key.KMS != "" {
		keyCount++
	}

	switch keyCount {
	case 0:
		errs = errs.Also(apis.ErrMissingOneOf("data", "kms", "secretref"))
	case 1:
		// do nothing -- a single key definition is valid
	default:
		errs = errs.Also(apis.ErrMultipleOneOf("data", "kms", "secretref"))
	}

	errs = errs.Also(validateHashAlgorithm(key.HashAlgorithm))

	return errs
}

// Validate ResourcePattern and make sure the Pattern is valid regex expression
func (r *ResourcePattern) Validate(ctx context.Context) (errs *apis.FieldError) {
	if _, err := regexp.Compile(r.Pattern); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(r.Pattern, "ResourcePattern", fmt.Sprintf("%v: %v", InvalidResourcePatternErr, err)))
		return errs
	}
	return nil
}

// validateHashAlgorithm checks if the algorithm is supported
func validateHashAlgorithm(algorithmName HashAlgorithm) (errs *apis.FieldError) {
	normalizedAlgo := strings.ToLower(string(algorithmName))
	_, exists := SupportedSignatureAlgorithms[HashAlgorithm(normalizedAlgo)]
	if !exists {
		return apis.ErrInvalidValue(algorithmName, "HashAlgorithm")
	}
	return nil
}
