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
	// ErrFailedLoadKeyFile is returned the key file cannot be read
	ErrFailedLoadKeyFile = errors.New("the key file cannot be read")
	// ErrDecodeKey is returned when the key cannot be decoded
	ErrDecodeKey = errors.New("key cannot be decoded")
	// ErrEmptyPublicKeys is returned when no public keys are founded
	ErrEmptyPublicKeys = errors.New("no public keys are founded")
	// ErrEmptySecretData is returned secret data is empty
	ErrEmptySecretData = errors.New("secret data is empty")
	// ErrSecretNotFound is returned when the secret is not found
	ErrSecretNotFound = errors.New("secret not found")
	// ErrMultipleSecretData is returned secret contains multiple data
	ErrMultipleSecretData = errors.New("secret contains multiple data")
	// ErrEmptyKey is returned when the key doesn't contain data or keyRef
	ErrEmptyKey = errors.New("key doesn't contain data or keyRef")
	// ErrK8sSpecificationInvalid is returned when kubernetes specification format is invalid
	ErrK8sSpecificationInvalid = errors.New("kubernetes specification should be in the format k8s://<namespace>/<secret>")
	// ErrLoadVerifier is returned when verifier cannot be loaded from the key
	ErrLoadVerifier = errors.New("verifier cannot to be loaded")
	// ErrAlgorithmInvalid is returned the hash algorithm is not supported
	ErrAlgorithmInvalid = errors.New("unknown digest algorithm")
)
