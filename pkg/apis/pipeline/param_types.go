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

package pipeline

import (
	"encoding/json"
	"fmt"
)

// ParamsPrefix is the prefix used in $(...) expressions referring to parameters
const ParamsPrefix = "params"

// ParamSpec defines arbitrary parameters needed beyond typed inputs (such as
// resources). Parameter values are provided by users as inputs on a TaskRun
// or PipelineRun.
type ParamSpec struct {
	// Name declares the name by which a parameter is referenced.
	Name string
	// Type is the user-specified type of the parameter. The possible types
	// are currently "string", "array" and "object", and "string" is the default.
	// +optional
	Type ParamType
	// Description is a user-facing description of the parameter that may be
	// used to populate a UI.
	// +optional
	Description string
	// Properties is the JSON Schema properties to support key-value pairs parameter.
	// +optional
	Properties map[string]PropertySpec
	// Default is the value a parameter takes if no input value is supplied. If
	// default is set, a Task may be executed without a supplied value for the
	// parameter.
	// +optional
	Default *ParamValue
}

// ParamSpecs is a list of ParamSpec
type ParamSpecs []ParamSpec

// PropertySpec defines the struct for object keys
type PropertySpec struct {
	Type ParamType
}

// Param declares an ParamValues to use for the parameter called name.
type Param struct {
	Name  string
	Value ParamValue
}

// Params is a list of Param
type Params []Param

// ParamType indicates the type of an input parameter;
// Used to distinguish between a single string and an array of strings.
type ParamType string

// Valid ParamTypes:
const (
	ParamTypeString ParamType = "string"
	ParamTypeArray  ParamType = "array"
	ParamTypeObject ParamType = "object"
)

// AllParamTypes can be used for ParamType validation.
var AllParamTypes = []ParamType{ParamTypeString, ParamTypeArray, ParamTypeObject}

// ParamValues is modeled after IntOrString in kubernetes/apimachinery:

// ParamValue is a type that can hold a single string, string array, or string map.
// Used in JSON unmarshalling so that a single JSON field can accept
// either an individual string or an array of strings.
type ParamValue struct {
	Type      ParamType // Represents the stored type of ParamValues.
	StringVal string
	// +listType=atomic
	ArrayVal  []string
	ObjectVal map[string]string
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (paramValues *ParamValue) UnmarshalJSON(value []byte) error {
	// ParamValues is used for Results Value as well, the results can be any kind of
	// data so we need to check if it is empty.
	if len(value) == 0 {
		paramValues.Type = ParamTypeString
		return nil
	}
	if value[0] == '[' {
		// We're trying to Unmarshal to []string, but for cases like []int or other types
		// of nested array which we don't support yet, we should continue and Unmarshal
		// it to String. If the Type being set doesn't match what it actually should be,
		// it will be captured by validation in reconciler.
		// if failed to unmarshal to array, we will convert the value to string and marshal it to string
		var a []string
		if err := json.Unmarshal(value, &a); err == nil {
			paramValues.Type = ParamTypeArray
			paramValues.ArrayVal = a
			return nil
		}
	}
	if value[0] == '{' {
		// if failed to unmarshal to map, we will convert the value to string and marshal it to string
		var m map[string]string
		if err := json.Unmarshal(value, &m); err == nil {
			paramValues.Type = ParamTypeObject
			paramValues.ObjectVal = m
			return nil
		}
	}

	// By default we unmarshal to string
	paramValues.Type = ParamTypeString
	if err := json.Unmarshal(value, &paramValues.StringVal); err == nil {
		return nil
	}
	paramValues.StringVal = string(value)

	return nil
}

// MarshalJSON implements the json.Marshaller interface.
func (paramValues ParamValue) MarshalJSON() ([]byte, error) {
	switch paramValues.Type {
	case ParamTypeString:
		return json.Marshal(paramValues.StringVal)
	case ParamTypeArray:
		return json.Marshal(paramValues.ArrayVal)
	case ParamTypeObject:
		return json.Marshal(paramValues.ObjectVal)
	default:
		return []byte{}, fmt.Errorf("impossible ParamValues.Type: %q", paramValues.Type)
	}
}
