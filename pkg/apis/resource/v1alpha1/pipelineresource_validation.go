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
	"fmt"
	"strings"

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
	if rs.Type == PipelineResourceTypeStorage {
		foundTypeParam := false
		var location string
		for _, param := range rs.Params {
			switch {
			case strings.EqualFold(param.Name, "type"):
				if !AllowedStorageType(param.Value) {
					return apis.ErrInvalidValue(param.Value, "spec.params.type")
				}
				foundTypeParam = true
			case strings.EqualFold(param.Name, "Location"):
				location = param.Value
			}
		}

		if !foundTypeParam {
			return apis.ErrMissingField("spec.params.type")
		}
		if location == "" {
			return apis.ErrMissingField("spec.params.location")
		}
	}

	if rs.Type == PipelineResourceTypePullRequest {
		if err := validatePullRequest(rs); err != nil {
			return err
		}
	}

	for _, allowedType := range AllResourceTypes {
		if allowedType == rs.Type {
			return nil
		}
	}

	return apis.ErrInvalidValue("spec.type", rs.Type)
}

// AllowedStorageType returns true if the provided string can be used as a storage type, and false otherwise
func AllowedStorageType(gotType string) bool {
	return gotType == PipelineResourceTypeGCS
}

func validatePullRequest(s *PipelineResourceSpec) *apis.FieldError {
	for _, param := range s.SecretParams {
		if param.FieldName != "authToken" {
			return apis.ErrInvalidValue(fmt.Sprintf("invalid field name %q in secret parameter. Expected %q", param.FieldName, "authToken"), "spec.secrets.fieldName")
		}
	}
	return nil
}
