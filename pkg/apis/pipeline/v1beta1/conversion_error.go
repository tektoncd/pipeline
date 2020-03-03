/*
Copyright 2020 The Tekton Authors.

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
	"fmt"

	"knative.dev/pkg/apis"
)

const (
	// ConditionTypeConvertible is a Warning condition that is set on
	// resources when they cannot be converted to warn of a forthcoming
	// breakage.
	ConditionTypeConvertible apis.ConditionType = "Convertible"
)

// CannotConvertError is returned when a field cannot be converted.
type CannotConvertError struct {
	Message string
	Field   string
}

var _ error = (*CannotConvertError)(nil)

// Error implements error
func (cce *CannotConvertError) Error() string {
	return cce.Message
}

// ConvertErrorf creates a CannotConvertError from the field name and format string.
func ConvertErrorf(field, msg string, args ...interface{}) error {
	return &CannotConvertError{
		Message: fmt.Sprintf(msg, args...),
		Field:   field,
	}
}
