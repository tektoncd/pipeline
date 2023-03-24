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
package trustedresources

import (
	"errors"
)

var (
	// ErrResourceVerificationFailed is returned when trusted resources fails verification.
	ErrResourceVerificationFailed = errors.New("resource verification failed")
	// ErrNoMatchedPolicies is returned when no policies are matched
	ErrNoMatchedPolicies = errors.New("no policies are matched")
	// ErrRegexMatch is returned when regex match returns error
	ErrRegexMatch = errors.New("regex failed to match")
)

type verificationResultType int

const (
	verificationResultError = iota
	verificationResultWarn
	verificationResultPass
)

// VerificationError wraps the verification result with the error
type VerificationError struct {
	// err wraps the verification error
	err error
	// resultType indicates if the verification is of error, warn or pass.
	resultType verificationResultType
}

// Error returns the error message
func (e *VerificationError) Error() string {
	return e.err.Error()
}

// Unwrap returns the error
func (e *VerificationError) Unwrap() error {
	return e.err
}

// IsVerificationResultError checks if the resultType is verificationResultError
func IsVerificationResultError(err error) bool {
	var verr *VerificationError
	if errors.As(err, &verr) {
		return verr.resultType == verificationResultError
	}
	return false
}

// IsVerificationResultPass checks if the resultType is verificationResultPass
func IsVerificationResultPass(err error) bool {
	var verr *VerificationError
	if errors.As(err, &verr) {
		return verr.resultType == verificationResultPass
	}
	return false
}
