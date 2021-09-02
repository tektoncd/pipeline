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

package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

func (tr *TaskResources) Validate(ctx context.Context) (errs *apis.FieldError) {
	if tr != nil {
		errs = errs.Also(validateTaskResources(tr.Inputs).ViaField("inputs"))
		errs = errs.Also(validateTaskResources(tr.Outputs).ViaField("outputs"))
	}
	return errs
}

func validateTaskResources(resources []TaskResource) (errs *apis.FieldError) {
	for idx, resource := range resources {
		errs = errs.Also(validateResourceType(resource, fmt.Sprintf("%s.type", resource.Name))).ViaIndex(idx)
	}
	return errs.Also(checkForDuplicates(resources, "name"))
}

func checkForDuplicates(resources []TaskResource, path string) *apis.FieldError {
	encountered := sets.NewString()
	for _, r := range resources {
		if encountered.Has(strings.ToLower(r.Name)) {
			return apis.ErrMultipleOneOf(path)
		}
		encountered.Insert(strings.ToLower(r.Name))
	}
	return nil
}

func validateResourceType(r TaskResource, path string) *apis.FieldError {
	for _, allowed := range AllResourceTypes {
		if r.Type == allowed {
			return nil
		}
	}
	return apis.ErrInvalidValue(string(r.Type), path)
}

func (tr *TaskRunResources) Validate(ctx context.Context) *apis.FieldError {
	if tr == nil {
		return nil
	}
	if err := validateTaskRunResources(ctx, tr.Inputs, "spec.resources.inputs.name"); err != nil {
		return err
	}
	return validateTaskRunResources(ctx, tr.Outputs, "spec.resources.outputs.name")
}

// validateTaskRunResources validates that
//	1. resource is not declared more than once
//	2. if both resource reference and resource spec is defined at the same time
//	3. at least resource ref or resource spec is defined
func validateTaskRunResources(ctx context.Context, resources []TaskResourceBinding, path string) *apis.FieldError {
	encountered := sets.NewString()
	for _, r := range resources {
		// We should provide only one binding for each resource required by the Task.
		name := strings.ToLower(r.Name)
		if encountered.Has(strings.ToLower(name)) {
			return apis.ErrMultipleOneOf(path)
		}
		encountered.Insert(name)
		// Check that both resource ref and resource Spec are not present
		if r.ResourceRef != nil && r.ResourceSpec != nil {
			return apis.ErrDisallowedFields(fmt.Sprintf("%s.resourceRef", path), fmt.Sprintf("%s.resourceSpec", path))
		}
		// Check that one of resource ref and resource Spec is present
		if (r.ResourceRef == nil || r.ResourceRef.Name == "") && r.ResourceSpec == nil {
			return apis.ErrMissingField(fmt.Sprintf("%s.resourceRef", path), fmt.Sprintf("%s.resourceSpec", path))
		}
		if r.ResourceSpec != nil && r.ResourceSpec.Validate(ctx) != nil {
			return r.ResourceSpec.Validate(ctx)
		}

	}
	return nil
}
