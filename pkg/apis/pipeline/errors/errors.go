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

package errors

import "errors"

const UserErrorLabel = "[User error] "

type UserError struct {
	Reason   string
	Original error
}

var _ error = &UserError{}

// Error returns the original error message. This implements the error.Error interface.
func (e *UserError) Error() string {
	return e.Original.Error()
}

// Unwrap returns the original error without the Reason annotation. This is
// intended to support usage of errors.Is and errors.As with Errors.
func (e *UserError) Unwrap() error {
	return e.Original
}

// newUserError returns a UserError with the given reason and underlying
// original error.
func newUserError(reason string, err error) *UserError {
	return &UserError{
		Reason:   reason,
		Original: err,
	}
}

// WrapUserError wraps the original error with the user error label
func WrapUserError(err error) error {
	return newUserError(UserErrorLabel, err)
}

// LabelUserError labels the failure RunStatus message if any of its error messages has been
// wrapped as an UserError. It indicates that the user is responsible for an error.
// See github.com/tektoncd/pipeline/blob/main/docs/pipelineruns.md#marking-off-user-errors
// for more details.
func LabelUserError(messageFormat string, messageA []interface{}) string {
	for _, message := range messageA {
		if ue, ok := message.(*UserError); ok {
			return ue.Reason + messageFormat
		}
	}
	return messageFormat
}

// GetErrorMessage returns the error message with the user error label if it is of type user
// error
func GetErrorMessage(err error) string {
	var ue *UserError
	if errors.As(err, &ue) {
		return ue.Reason + err.Error()
	}
	return err.Error()
}
