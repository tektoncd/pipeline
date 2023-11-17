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
	if tr.Type == "" {
		if tr.Properties != nil {
			// Set type to object if `properties` is given
			tr.Type = ResultsTypeObject
		} else {
			// ResultsTypeString is the default value
			tr.Type = ResultsTypeString
		}
	}

	// Set default type of object values to string
	for key, propertySpec := range tr.Properties {
		if propertySpec.Type == "" {
			tr.Properties[key] = PropertySpec{Type: ParamType(ResultsTypeString)}
		}
	}
}

// SetDefaults set the default type for StepResult
func (sr *StepResult) SetDefaults(context.Context) {
	if sr == nil {
		return
	}
	if sr.Type == "" {
		if sr.Properties != nil {
			// Set type to object if `properties` is given
			sr.Type = ResultsTypeObject
		} else {
			// ResultsTypeString is the default value
			sr.Type = ResultsTypeString
		}
	}

	// Set default type of object values to string
	for key, propertySpec := range sr.Properties {
		if propertySpec.Type == "" {
			sr.Properties[key] = PropertySpec{Type: ParamType(ResultsTypeString)}
		}
	}
}
