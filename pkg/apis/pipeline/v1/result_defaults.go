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

package v1

import "context"

// SetDefaults set the default type for TaskResult
func (tr *TaskResult) SetDefaults(context.Context) {
	if tr == nil {
		return
	}
	switch tr.Type {
	case "":
		if tr.Properties != nil {
			// Set type to object if `properties` is given
			tr.Type = ResultsTypeObject
		} else {
			// ResultsTypeString is the default value
			tr.Type = ResultsTypeString
		}
	case ResultsTypeArtifact:
		// SetDefaults is invoked before validation
		// If custom properties are set, it's up to the validation logic to
		// decide wheather to fail
		if tr.Properties == nil {
			// Artifact type cannot be inferred
			tr.Properties = artifactSchema
		}
	}

	// Set default type of object values to string
	for key, propertySpec := range tr.Properties {
		if propertySpec.Type == "" {
			tr.Properties[key] = PropertySpec{Type: ParamType(ResultsTypeString)}
		}
	}
}
