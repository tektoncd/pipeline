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

	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"knative.dev/pkg/apis"
)

// Validate checks that a submitted ResolutionRequest is structurally
// sound before the controller receives it.
func (rr *ResolutionRequest) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(validateTypeLabel(rr))
	return errs.Also(rr.Spec.Validate(ctx).ViaField("spec"))
}

// Validate checks the spec field of a ResolutionRequest is valid.
func (rs *ResolutionRequestSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	return nil
}

func validateTypeLabel(rr *ResolutionRequest) *apis.FieldError {
	typeLabel := getTypeLabel(rr.ObjectMeta.Labels)
	if typeLabel == "" {
		return apis.ErrMissingField(common.LabelKeyResolverType).ViaField("labels").ViaField("meta")
	}
	return nil
}

func getTypeLabel(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	return labels[common.LabelKeyResolverType]
}
