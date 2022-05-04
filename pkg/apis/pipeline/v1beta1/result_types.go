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

package v1beta1

// TaskResult used to describe the results of a task
type TaskResult struct {
	// Name the given name
	Name string `json:"name"`

	// Type is the user-specified type of the result. The possible type
	// is currently "string" and will support "array" in following work.
	// +optional
	Type ResultsType `json:"type,omitempty"`

	// Description is a human-readable description of the result
	// +optional
	Description string `json:"description"`
}

// TaskRunResult used to describe the results of a task
type TaskRunResult struct {
	// Name the given name
	Name string `json:"name"`

	// Type is the user-specified type of the result. The possible type
	// is currently "string" and will support "array" in following work.
	// +optional
	Type ResultsType `json:"type,omitempty"`

	// Value the given value of the result
	Value ArrayOrString `json:"value"`
}

// ResultsType indicates the type of a result;
// Used to distinguish between a single string and an array of strings.
// Note that there is ResultType used to find out whether a
// PipelineResourceResult is from a task result or not, which is different from
// this ResultsType.
// TODO(#4723): add "array" and "object" support
// TODO(#4723): align ResultsType and ParamType in ArrayOrString
type ResultsType string

// Valid ResultsType:
const (
	ResultsTypeString ResultsType = "string"
	ResultsTypeArray  ResultsType = "array"
	ResultsTypeObject ResultsType = "object"
)

// AllResultsTypes can be used for ResultsTypes validation.
var AllResultsTypes = []ResultsType{ResultsTypeString, ResultsTypeArray, ResultsTypeObject}
