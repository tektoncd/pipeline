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

import "strings"

// TaskResult used to describe the results of a task
type TaskResult struct {
	// Name the given name
	Name string `json:"name"`

	// Type is the user-specified type of the result. The possible type
	// is currently "string" and will support "array" in following work.
	// +optional
	Type ResultsType `json:"type,omitempty"`

	// Properties is the JSON Schema properties to support key-value pairs results.
	// +optional
	Properties map[string]PropertySpec `json:"properties,omitempty"`

	// Description is a human-readable description of the result
	// +optional
	Description string `json:"description,omitempty"`

	// Value the expression used to retrieve the value of the result from an underlying Step.
	// +optional
	Value *ResultValue `json:"value,omitempty"`
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
	Value ResultValue `json:"value"`
}

// TaskRunStepResult is a type alias of TaskRunResult
type TaskRunStepResult = TaskRunResult

// ResultValue is a type alias of ParamValue
type ResultValue = ParamValue

// ResultsType indicates the type of a result;
// Used to distinguish between a single string and an array of strings.
// Note that there is ResultType used to find out whether a
// RunResult is from a task result or not, which is different from
// this ResultsType.
type ResultsType string

// Valid ResultsType:
const (
	ResultsTypeString ResultsType = "string"
	ResultsTypeArray  ResultsType = "array"
	ResultsTypeObject ResultsType = "object"
)

// AllResultsTypes can be used for ResultsTypes validation.
var AllResultsTypes = []ResultsType{ResultsTypeString, ResultsTypeArray, ResultsTypeObject}

// ResultsArrayReference returns the reference of the result. e.g. results.resultname from $(results.resultname[*])
func ResultsArrayReference(a string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(a, "$("), ")"), "[*]")
}
