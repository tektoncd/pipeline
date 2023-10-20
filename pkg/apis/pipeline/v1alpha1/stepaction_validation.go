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

package v1alpha1

import (
	"context"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var _ apis.Validatable = (*StepAction)(nil)
var _ resourcesemantics.VerbLimited = (*StepAction)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (s *StepAction) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate implements apis.Validatable
func (s *StepAction) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = validate.ObjectMetadata(s.GetObjectMeta()).ViaField("metadata")
	errs = errs.Also(s.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
	return errs
}

// Validate implements apis.Validatable
func (ss *StepActionSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ss.Image == "" {
		errs = errs.Also(apis.ErrMissingField("Image"))
	}

	if ss.Script != "" {
		if len(ss.Command) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: "script cannot be used with command",
				Paths:   []string{"script"},
			})
		}

		cleaned := strings.TrimSpace(ss.Script)
		if strings.HasPrefix(cleaned, "#!win") {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "windows script support", config.AlphaAPIFields).ViaField("script"))
		}
	}
	return errs
}
