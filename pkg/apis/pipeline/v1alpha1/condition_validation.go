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

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*Condition)(nil)

// Validate performs validation on the Condition's metadata and spec
func (c Condition) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(c.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	if apis.IsInDelete(ctx) {
		return nil
	}
	return c.Spec.Validate(ctx).ViaField("Spec")
}

// Validate makes sure the ConditionSpec is actually configured and that its name is a valid DNS label,
// and finally validates its steps.
func (cs *ConditionSpec) Validate(ctx context.Context) *apis.FieldError {
	if equality.Semantic.DeepEqual(cs, ConditionSpec{}) {
		return apis.ErrMissingField(apis.CurrentField)
	}

	// Validate condition check name
	if errs := validation.IsDNS1123Label(cs.Check.Name); cs.Check.Name != "" && len(errs) > 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid value %q", cs.Check.Name),
			Paths:   []string{"Check.name"},
			Details: "Condition check name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		}
	}

	return validateSteps([]Step{cs.Check}).ViaField("Check")
}
