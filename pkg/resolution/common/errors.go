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

package common

import (
	"errors"
	"fmt"
)

// Error embeds both a short machine-readable string reason for resolution
// problems alongside the original error generated during the resolution flow.
type Error struct {
	Reason   string
	Original error
}

var _ error = &Error{}

// Error returns the original error's message. This is intended to meet the error.Error interface.
func (e *Error) Error() string {
	return e.Original.Error()
}

// Unwrap returns the original error without the Reason annotation. This is
// intended to support usage of errors.Is and errors.As with Errors.
func (e *Error) Unwrap() error {
	return e.Original
}

// NewError returns a Error with the given reason and underlying
// original error.
func NewError(reason string, err error) *Error {
	return &Error{
		Reason:   reason,
		Original: err,
	}
}

var (
	// ErrRequestInProgress is a sentinel value to indicate that
	// a resource request is still in progress.
	ErrRequestInProgress = NewError("RequestInProgress", errors.New("Resource request is still in-progress"))

	// ErrorRequestInProgress is an alias to ErrRequestInProgress
	//
	// Deprecated: use ErrRequestInProgress instead.
	ErrorRequestInProgress = ErrRequestInProgress
)

// InvalidResourceKeyError indicates that a string key given to the
// Reconcile function does not match the expected "name" or "namespace/name"
// format.
type InvalidResourceKeyError struct {
	Key      string
	Original error
}

// ErrorInvalidResourceKey is an alias to type InvalidResourceKeyError.
//
// Deprecated: use type InvalidResourceKeyError instead.
type ErrorInvalidResourceKey = InvalidResourceKeyError

var _ error = &InvalidResourceKeyError{}

func (e *InvalidResourceKeyError) Error() string {
	return fmt.Sprintf("invalid resource key %q: %v", e.Key, e.Original)
}

func (e *InvalidResourceKeyError) Unwrap() error {
	return e.Original
}

// InvalidRequestError is an error received when a
// resource request is badly formed for some reason: either the
// parameters don't match the resolver's expectations or there is some
// other structural issue.
type InvalidRequestError struct {
	ResolutionRequestKey string
	Message              string
}

// ErrorInvalidRequest is an alias to type InvalidRequestError.
//
// Deprecated: use type InvalidRequestError instead.
type ErrorInvalidRequest = InvalidRequestError

var _ error = &InvalidRequestError{}

func (e *InvalidRequestError) Error() string {
	return fmt.Sprintf("invalid resource request %q: %s", e.ResolutionRequestKey, e.Message)
}

// GetResourceError is an error received during what should
// otherwise have been a successful resource request.
type GetResourceError struct {
	ResolverName string
	Key          string
	Original     error
}

// ErrorGettingResource is an alias to type GetResourceError.
//
// Deprecated: use type GetResourceError instead.
type ErrorGettingResource = GetResourceError

var _ error = &GetResourceError{}

func (e *GetResourceError) Error() string {
	return fmt.Sprintf("error getting %q %q: %v", e.ResolverName, e.Key, e.Original)
}

func (e *GetResourceError) Unwrap() error {
	return e.Original
}

// UpdatingRequestError is an error during any part of the update
// process for a ResolutionRequest, e.g. when attempting to patch the
// ResolutionRequest with resolved data.
type UpdatingRequestError struct {
	ResolutionRequestKey string
	Original             error
}

// ErrorUpdatingRequest is an alias to UpdatingRequestError
//
// Deprecated: use UpdatingRequestError instead.
type ErrorUpdatingRequest = UpdatingRequestError

var _ error = &UpdatingRequestError{}

func (e *UpdatingRequestError) Error() string {
	return fmt.Sprintf("error updating resource request %q with data: %v", e.ResolutionRequestKey, e.Original)
}

func (e *UpdatingRequestError) Unwrap() error {
	return e.Original
}

// ReasonError extracts the reason and underlying error
// embedded in a given error or returns some sane defaults
// if the error isn't a common.Error.
func ReasonError(err error) (string, error) {
	reason := ReasonResolutionFailed
	resolutionError := err

	var e *Error
	if errors.As(err, &e) {
		reason = e.Reason
		resolutionError = e.Unwrap()
	}

	return reason, resolutionError
}
