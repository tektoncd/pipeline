/*
Copyright 2018 The Knative Authors

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

// Package validation provides methods to surface validation errors.
package validation

import "fmt"

// Error is for use with a FooInvalid Status Condition.
type Error struct {
	Reason  string
	Message string
}

func (ve *Error) Error() string {
	return fmt.Sprintf("%s: %s", ve.Reason, ve.Message)
}

// NewError returns a new validation error.
func NewError(reason, format string, fmtArgs ...interface{}) error {
	return &Error{
		Reason:  reason,
		Message: fmt.Sprintf(format, fmtArgs...),
	}
}
