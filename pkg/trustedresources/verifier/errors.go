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
package verifier

import "errors"

var (
	// ErrorFailedLoadKeyFile is returned the key file cannot be read
	ErrorFailedLoadKeyFile = errors.New("the key file cannot be read")
	// ErrorDecodeKey is returned when the key cannot be decoded
	ErrorDecodeKey = errors.New("key cannot be decoded")
	// ErrorEmptyPublicKeys is returned when no public keys are founded
	ErrorEmptyPublicKeys = errors.New("no public keys are founded")
	// ErrorEmptySecretData is returned secret data is empty
	ErrorEmptySecretData = errors.New("secret data is empty")
	// ErrorSecretNotFound is returned when the secret is not found
	ErrorSecretNotFound = errors.New("secret not found")
	// ErrorMultipleSecretData is returned secret contains multiple data
	ErrorMultipleSecretData = errors.New("secret contains multiple data")
	// ErrorEmptyKey is returned when the key doesn't contain data or keyRef
	ErrorEmptyKey = errors.New("key doesn't contain data or keyRef")
	// ErrorK8sSpecificationInvalid is returned when kubernetes specification format is invalid
	ErrorK8sSpecificationInvalid = errors.New("kubernetes specification should be in the format k8s://<namespace>/<secret>")
	// ErrorLoadVerifier is returned when verifier cannot be loaded from the key
	ErrorLoadVerifier = errors.New("verifier cannot to be loaded")
	// ErrorAlgorithmInvalid is returned the hash algorithm is not supported
	ErrorAlgorithmInvalid = errors.New("unknown digest algorithm")
)
