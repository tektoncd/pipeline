/*
Copyright 2019 The Tekton Authors

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

var _ apis.Validatable = (*PipelineResource)(nil)

// Validate validates the PipelineResource's ObjectMeta and Spec
func (r *PipelineResource) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(r.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	if apis.IsInDelete(ctx) {
		return nil
	}
	return r.Spec.Validate(ctx)
}

// Validate validates the PipelineResourceSpec based on its type
func (rs *PipelineResourceSpec) Validate(ctx context.Context) *apis.FieldError {
	if equality.Semantic.DeepEqual(rs, &PipelineResourceSpec{}) {
		return apis.ErrMissingField("spec.type")
	}

	return apis.ErrInvalidValue("spec.type", rs.Type)
}
