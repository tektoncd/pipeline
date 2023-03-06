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
	// ErrorResourceVerificationFailed is returned when trusted resources fails verification.
	ErrorResourceVerificationFailed = errors.New("resource verification failed")
	// ErrorEmptyVerificationConfig is returned when no VerificationPolicy is founded
	ErrorEmptyVerificationConfig = errors.New("no policies founded for verification")
	// ErrorNoMatchedPolicies is returned when no policies are matched
	ErrorNoMatchedPolicies = errors.New("no policies are matched")
	// ErrorRegexMatch is returned when regex match returns error
	ErrorRegexMatch = errors.New("regex failed to match")
	// ErrorSignatureMissing is returned when signature is missing in resource
	ErrorSignatureMissing = errors.New("signature is missing")
)
