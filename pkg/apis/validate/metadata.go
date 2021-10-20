/*
Copyright 2019 The Tekton Authors

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

package validate

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

// MaxLength is the maximum length that an object's name can be
const MaxLength = validation.DNS1123LabelMaxLength

// ObjectMetadata validates that the given object's name is a valid DNS name and isn't longer than the max length
func ObjectMetadata(meta metav1.Object) *apis.FieldError {
	name := meta.GetName()

	if err := validation.IsDNS1123Subdomain(name); len(err) > 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid resource name %q: must be a valid DNS label", name),
			Paths:   []string{"name"},
		}
	}

	if len(name) > MaxLength {
		return &apis.FieldError{
			Message: "Invalid resource name: length must be no more than 63 characters",
			Paths:   []string{"name"},
		}
	}
	return nil
}
