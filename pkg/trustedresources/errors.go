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

import "errors"

var (
	// ErrResourceVerificationFailed is returned when trusted resources fails verification.
	ErrResourceVerificationFailed = errors.New("resource verification failed")
	// ErrEmptyVerificationConfig is returned when no VerificationPolicy is found
	ErrEmptyVerificationConfig = errors.New("no policies founded for verification")
	// ErrNoMatchedPolicies is returned when no policies are matched
	ErrNoMatchedPolicies = errors.New("no policies are matched")
	// ErrRegexMatch is returned when regex match returns error
	ErrRegexMatch = errors.New("regex failed to match")
	// ErrSignatureMissing is returned when signature is missing in resource
	ErrSignatureMissing = errors.New("signature is missing")
)
