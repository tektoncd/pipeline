/*
Copyright 2025 The Tekton Authors

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

package matcher

import (
	"fmt"
	"reflect"
)

// VolumePath is the path where build secrets are written.
// It is mutable and exported for testing.
var VolumePath = "/tekton/creds-secrets"

// Secret is the minimal interface needed for credential matching
type Secret interface {
	GetName() string
	GetAnnotations() map[string]string
}

// Matcher is the interface for a credential initializer of any type.
type Matcher interface {
	// MatchingAnnotations extracts flags for the credential
	// helper from the supplied secret and returns a slice (of length 0 or greater)
	MatchingAnnotations(secret Secret) []string
}

// VolumeName returns the full path to the secret, inside the VolumePath.
func VolumeName(secretName string) string {
	return fmt.Sprintf("%s/%s", VolumePath, secretName)
}

// GetSecretType returns secret type from secret interface using reflection
func GetSecretType(secret Secret) string {
	if secret == nil {
		return ""
	}
	v := reflect.ValueOf(secret)
	// If a pointer, check if it's nil before dereferencing
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	// access the Type field for Kubernetes secrets
	f := v.FieldByName("Type")
	if !f.IsValid() || !f.CanInterface() {
		return ""
	}
	return fmt.Sprintf("%v", f.Interface())
}
