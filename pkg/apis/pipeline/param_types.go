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
