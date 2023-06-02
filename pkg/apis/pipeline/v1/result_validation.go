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

package v1

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"
)

// Validate implements apis.Validatable
func (tr TaskResult) Validate(ctx context.Context) (errs *apis.FieldError) {
	if !resultNameFormatRegex.MatchString(tr.Name) {
		return apis.ErrInvalidKeyName(tr.Name, "name", fmt.Sprintf("Name must consist of alphanumeric characters, '-', '_', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my-name',  or 'my_name', regex used for validation is '%s')", ResultNameFormat))
	}

	switch {
	case tr.Type == ResultsTypeObject:
		errs := validateObjectResult(tr)
		return errs
	case tr.Type == ResultsTypeArray:
		return nil
	// Resources created before the result. Type was introduced may not have Type set
	// and should be considered valid
	case tr.Type == "":
		return nil
	// By default, the result type is string
	case tr.Type != ResultsTypeString:
		return apis.ErrInvalidValue(tr.Type, "type", "type must be string")
	}

	return nil
}

// validateObjectResult validates the object result and check if the Properties is missing
// for Properties values it will check if the type is string.
func validateObjectResult(tr TaskResult) (errs *apis.FieldError) {
	if ParamType(tr.Type) == ParamTypeObject && tr.Properties == nil {
		return apis.ErrMissingField(fmt.Sprintf("%s.properties", tr.Name))
	}

	invalidKeys := []string{}
	for key, propertySpec := range tr.Properties {
		if propertySpec.Type != ParamTypeString {
			invalidKeys = append(invalidKeys, key)
		}
	}

	if len(invalidKeys) != 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("The value type specified for these keys %v is invalid, the type must be string", invalidKeys),
			Paths:   []string{fmt.Sprintf("%s.properties", tr.Name)},
		}
	}
	return nil
}
