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

package v1alpha1

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*Run)(nil)

// Validate taskrun
func (r *Run) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(r.GetObjectMeta()).ViaField("metadata"); err != nil {
		return err
	}
	if apis.IsInDelete(ctx) {
		return nil
	}
	return r.Spec.Validate(ctx)
}

// Validate Run spec
func (rs *RunSpec) Validate(ctx context.Context) *apis.FieldError {
	// this covers the case rs.Ref == nil && rs.Spec == nil
	if equality.Semantic.DeepEqual(rs, &RunSpec{}) {
		return apis.ErrMissingField("spec")
	}

	if rs.Ref != nil && rs.Spec != nil {
		return apis.ErrMultipleOneOf("spec.ref", "spec.spec")
	}
	if rs.Ref == nil && rs.Spec == nil {
		return apis.ErrMissingOneOf("spec.ref", "spec.spec")
	}
	if rs.Ref != nil {
		if rs.Ref.APIVersion == "" {
			return apis.ErrMissingField("spec.ref.apiVersion")
		}
		if rs.Ref.Kind == "" {
			return apis.ErrMissingField("spec.ref.kind")
		}
	}
	if rs.Spec != nil {
		if rs.Spec.APIVersion == "" {
			return apis.ErrMissingField("spec.spec.apiVersion")
		}
		if rs.Spec.Kind == "" {
			return apis.ErrMissingField("spec.spec.kind")
		}
	}
	if err := validateParameters("spec.params", rs.Params); err != nil {
		return err
	}

	return validateWorkspaceBindings(ctx, rs.Workspaces)
}
