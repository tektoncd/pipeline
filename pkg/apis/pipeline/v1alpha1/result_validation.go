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
	"fmt"
	"regexp"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"knative.dev/pkg/apis"
)

const (
	// resultNameFormat Constant used to define the regex Result.Name should follow
	resultNameFormat = `^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`
)

var resultNameFormatRegex = regexp.MustCompile(resultNameFormat)

// validate implements apis.Validatable
func (sar StepActionResult) validate(ctx context.Context) (errs *apis.FieldError) {
	if !resultNameFormatRegex.MatchString(sar.Name) {
		return apis.ErrInvalidKeyName(sar.Name, "name", fmt.Sprintf("Name must consist of alphanumeric characters, '-', '_', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my-name',  or 'my_name', regex used for validation is '%s')", resultNameFormat))
	}

	switch {
	case sar.Type == v1.ResultsTypeObject:
		errs := validateObjectResult(sar)
		return errs
	case sar.Type == v1.ResultsTypeArray:
		return nil
	// Resources created before the result. Type was introduced may not have Type set
	// and should be considered valid
	case sar.Type == "":
		return nil
	// By default, the result type is string
	case sar.Type != v1.ResultsTypeString:
		return apis.ErrInvalidValue(sar.Type, "type", "type must be string")
	}

	return nil
}

// validateObjectResult validates the object result and check if the Properties is missing
// for Properties values it will check if the type is string.
func validateObjectResult(sar StepActionResult) (errs *apis.FieldError) {
	if v1.ParamType(sar.Type) == v1.ParamTypeObject && sar.Properties == nil {
		return apis.ErrMissingField(fmt.Sprintf("%s.properties", sar.Name))
	}

	invalidKeys := []string{}
	for key, propertySpec := range sar.Properties {
		if propertySpec.Type != v1.ParamTypeString {
			invalidKeys = append(invalidKeys, key)
		}
	}

	if len(invalidKeys) != 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("The value type specified for these keys %v is invalid, the type must be string", invalidKeys),
			Paths:   []string{fmt.Sprintf("%s.properties", sar.Name)},
		}
	}
	return nil
}
