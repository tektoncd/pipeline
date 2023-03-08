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

package resolution

import (
	"errors"
	"fmt"
)

var (
	// ErrNilResource is returned when remote resolution
	// appears to have succeeded but the resolved resource is nil.
	ErrNilResource = errors.New("unknown error occurred: requested resource is nil")

	// ErrorRequestedResourceIsNil is a deprecated alias for ErrNilResource and will
	// be removed in a future release.
	//
	// Deprecated: use ErrNilResource instead.
	ErrorRequestedResourceIsNil = ErrNilResource
)

// InvalidRuntimeObjectError is returned when remote resolution
// succeeded but the returned data is not a valid runtime.Object.
type InvalidRuntimeObjectError struct {
	original error
}

// ErrorInvalidRuntimeObject is an alias to InvalidRuntimeObjectError.
//
// Deprecated: use InvalidRuntimeObjectError instead.
type ErrorInvalidRuntimeObject = InvalidRuntimeObjectError

var (
	_ error = &InvalidRuntimeObjectError{}
	_ error = &ErrorInvalidRuntimeObject{}
)

// Error returns the string representation of this error.
func (e *InvalidRuntimeObjectError) Error() string {
	return fmt.Sprintf("invalid runtime object: %v", e.original)
}

// Unwrap returns the underlying original error.
func (e *InvalidRuntimeObjectError) Unwrap() error {
	return e.original
}

// Is returns true if the given error coerces into an error of this type.
func (e *InvalidRuntimeObjectError) Is(that error) bool {
	return errors.As(that, &e)
}

// DataAccessError is returned when remote resolution succeeded but
// attempting to access the resolved data failed. An example of this
// type of error would be if a ResolutionRequest contained malformed base64.
type DataAccessError struct {
	original error
}

// ErrorAccessingData is an alias to DataAccessError
//
// Deprecated: use DataAccessError instead.
type ErrorAccessingData = DataAccessError

var (
	_ error = &DataAccessError{}
	_ error = &ErrorAccessingData{}
)

// Error returns the string representation of this error.
func (e *DataAccessError) Error() string {
	return fmt.Sprintf("error accessing data from remote resource: %v", e.original)
}

// Unwrap returns the underlying original error.
func (e *DataAccessError) Unwrap() error {
	return e.original
}

// Is returns true if the given error coerces into an error of this type.
func (e *DataAccessError) Is(that error) bool {
	return errors.As(that, &e)
}
