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

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var _ apis.Validatable = (*CustomRun)(nil)
var _ resourcesemantics.VerbLimited = (*CustomRun)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (r *CustomRun) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate customRun
func (r *CustomRun) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(r.GetObjectMeta()).ViaField("metadata"); err != nil {
		return err
	}
	return r.Spec.Validate(ctx)
}

// Validate CustomRun spec
func (rs *CustomRunSpec) Validate(ctx context.Context) *apis.FieldError {
	// this covers the case rs.customRef == nil && rs.customSpec == nil
	if equality.Semantic.DeepEqual(rs, &CustomRunSpec{}) {
		return apis.ErrMissingField("spec")
	}

	if rs.CustomRef != nil && rs.CustomSpec != nil {
		return apis.ErrMultipleOneOf("spec.customRef", "spec.customSpec")
	}
	if rs.CustomRef == nil && rs.CustomSpec == nil {
		return apis.ErrMissingOneOf("spec.customRef", "spec.customSpec")
	}
	if rs.CustomRef != nil {
		if rs.CustomRef.APIVersion == "" {
			return apis.ErrMissingField("spec.customRef.apiVersion")
		}
		if rs.CustomRef.Kind == "" {
			return apis.ErrMissingField("spec.customRef.kind")
		}
	}
	if rs.CustomSpec != nil {
		if rs.CustomSpec.APIVersion == "" {
			return apis.ErrMissingField("spec.customSpec.apiVersion")
		}
		if rs.CustomSpec.Kind == "" {
			return apis.ErrMissingField("spec.customSpec.kind")
		}
	}
	if rs.Status == "" {
		if rs.StatusMessage != "" {
			return apis.ErrInvalidValue(fmt.Sprintf("statusMessage should not be set if status is not set, but it is currently set to %s", rs.StatusMessage), "statusMessage")
		}
	}
	if err := ValidateParameters(ctx, rs.Params).ViaField("spec.params"); err != nil {
		return err
	}

	return ValidateWorkspaceBindings(ctx, rs.Workspaces).ViaField("spec.workspaces")
}
