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

package v1alpha2

import (
	"context"
	"fmt"
	"strings"

	"knative.dev/pkg/apis"
)

func (tr *TaskResources) Validate(ctx context.Context) *apis.FieldError {
	if tr == nil {
		return nil
	}
	if err := validateTaskResources(tr.Inputs, "inputs"); err != nil {
		return err
	}
	if err := validateTaskResources(tr.Outputs, "outputs"); err != nil {
		return err
	}
	return nil
}

func validateTaskResources(resources []TaskResource, name string) *apis.FieldError {
	for _, resource := range resources {
		if err := validateResourceType(resource, fmt.Sprintf("taskspec.resources.%s.%s.Type", name, resource.Name)); err != nil {
			return err
		}
	}
	if err := checkForDuplicates(resources, fmt.Sprintf("taskspec.resources.%s.name", name)); err != nil {
		return err
	}
	return nil
}

func checkForDuplicates(resources []TaskResource, path string) *apis.FieldError {
	encountered := map[string]struct{}{}
	for _, r := range resources {
		if _, ok := encountered[strings.ToLower(r.Name)]; ok {
			return apis.ErrMultipleOneOf(path)
		}
		encountered[strings.ToLower(r.Name)] = struct{}{}
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
