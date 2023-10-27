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

package v1alpha1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// StepActionResult used to describe the results of a task
type StepActionResult struct {
	// Name the given name
	Name string `json:"name"`

	// Type is the user-specified type of the result. The possible type
	// is currently "string" and will support "array" in following work.
	// +optional
	Type v1.ResultsType `json:"type,omitempty"`

	// Properties is the JSON Schema properties to support key-value pairs results.
	// +optional
	Properties map[string]v1.PropertySpec `json:"properties,omitempty"`

	// Description is a human-readable description of the result
	// +optional
	Description string `json:"description,omitempty"`
}

// SetDefaults set the default type for StepActionResult
func (sar *StepActionResult) SetDefaults(context.Context) {
	if sar == nil {
		return
	}
	if sar.Type == "" {
		if sar.Properties != nil {
			// Set type to object if `properties` is given
			sar.Type = v1.ResultsTypeObject
		} else {
			// ResultsTypeString is the default value
			sar.Type = v1.ResultsTypeString
		}
	}

	// Set default type of object values to string
	for key, propertySpec := range sar.Properties {
		if propertySpec.Type == "" {
			sar.Properties[key] = v1.PropertySpec{Type: v1.ParamType(v1.ResultsTypeString)}
		}
	}
}
