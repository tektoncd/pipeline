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

package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

// ParamsPrefix is the prefix used in $(...) expressions referring to parameters
const ParamsPrefix = "params"

// ParamSpec defines arbitrary parameters needed beyond typed inputs (such as
// resources). Parameter values are provided by users as inputs on a TaskRun
// or PipelineRun.
type ParamSpec struct {
	// Name declares the name by which a parameter is referenced.
	Name string `json:"name"`
	// Type is the user-specified type of the parameter. The possible types
	// are currently "string", "array" and "object", and "string" is the default.
	// +optional
	Type ParamType `json:"type,omitempty"`
	// Description is a user-facing description of the parameter that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`
	// Properties is the JSON Schema properties to support key-value pairs parameter.
	// +optional
	Properties map[string]PropertySpec `json:"properties,omitempty"`
	// Default is the value a parameter takes if no input value is supplied. If
	// default is set, a Task may be executed without a supplied value for the
	// parameter.
	// +optional
	Default *ParamValue `json:"default,omitempty"`
}

// ParamSpecs is a list of ParamSpec
type ParamSpecs []ParamSpec

// PropertySpec defines the struct for object keys
type PropertySpec struct {
	Type ParamType `json:"type,omitempty"`
}

// SetDefaults set the default type
func (pp *ParamSpec) SetDefaults(context.Context) {
	if pp == nil {
		return
	}

	// Propagate inferred type to the parent ParamSpec's type, and default type to the PropertySpec's type
	// The sequence to look at is type in ParamSpec -> properties -> type in default -> array/string/object value in default
	// If neither `properties` or `default` section is provided, ParamTypeString will be the default type.
	switch {
	case pp.Type != "":
		// If param type is provided by the author, do nothing but just set default type for PropertySpec in case `properties` section is provided.
		pp.setDefaultsForProperties()
	case pp.Properties != nil:
		pp.Type = ParamTypeObject
		// Also set default type for PropertySpec
		pp.setDefaultsForProperties()
	case pp.Default == nil:
		// ParamTypeString is the default value (when no type can be inferred from the default value)
		pp.Type = ParamTypeString
	case pp.Default.Type != "":
		pp.Type = pp.Default.Type
	case pp.Default.ArrayVal != nil:
		pp.Type = ParamTypeArray
	case pp.Default.ObjectVal != nil:
		pp.Type = ParamTypeObject
	default:
		pp.Type = ParamTypeString
	}
}

// getNames returns all the names of the declared parameters
func (ps ParamSpecs) getNames() []string {
	var names []string
	for _, p := range ps {
		names = append(names, p.Name)
	}
	return names
}

// sortByType splits the input params into string params, array params, and object params, in that order
func (ps ParamSpecs) sortByType() (ParamSpecs, ParamSpecs, ParamSpecs) {
	var stringParams, arrayParams, objectParams ParamSpecs
	for _, p := range ps {
		switch p.Type {
		case ParamTypeArray:
			arrayParams = append(arrayParams, p)
		case ParamTypeObject:
			objectParams = append(objectParams, p)
		case ParamTypeString:
			fallthrough
		default:
			stringParams = append(stringParams, p)
		}
	}
	return stringParams, arrayParams, objectParams
}

// validateNoDuplicateNames returns an error if any of the params have the same name
func (ps ParamSpecs) validateNoDuplicateNames() *apis.FieldError {
	names := ps.getNames()
	seen := sets.String{}
	dups := sets.String{}
	var errs *apis.FieldError
	for _, n := range names {
		if seen.Has(n) {
			dups.Insert(n)
		}
		seen.Insert(n)
	}
	for n := range dups {
		errs = errs.Also(apis.ErrGeneric("parameter appears more than once", "").ViaFieldKey("params", n))
	}
	return errs
}

// setDefaultsForProperties sets default type for PropertySpec (string) if it's not specified
func (pp *ParamSpec) setDefaultsForProperties() {
	for key, propertySpec := range pp.Properties {
		if propertySpec.Type == "" {
			pp.Properties[key] = PropertySpec{Type: ParamTypeString}
		}
	}
}

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

// ParamValue is a type that can hold a single string or string array.
// Used in JSON unmarshalling so that a single JSON field can accept
// either an individual string or an array of strings.
type ParamValue struct {
	Type      ParamType // Represents the stored type of ParamValues.
	StringVal string
	// +listType=atomic
	ArrayVal  []string
	ObjectVal map[string]string
}

// ArrayOrString is deprecated, this is to keep backward compatibility
//
// Deprecated: Use ParamValue instead.
type ArrayOrString = ParamValue

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

// NewStructuredValues creates an ParamValues of type ParamTypeString or ParamTypeArray, based on
// how many inputs are given (>1 input will create an array, not string).
func NewStructuredValues(value string, values ...string) *ParamValue {
	if len(values) > 0 {
		return &ParamValue{
			Type:     ParamTypeArray,
			ArrayVal: append([]string{value}, values...),
		}
	}
	return &ParamValue{
		Type:      ParamTypeString,
		StringVal: value,
	}
}

// NewArrayOrString is the deprecated, this is to keep backward compatibility
var NewArrayOrString = NewStructuredValues

// NewObject creates an ParamValues of type ParamTypeObject using the provided key-value pairs
func NewObject(pairs map[string]string) *ParamValue {
	return &ParamValue{
		Type:      ParamTypeObject,
		ObjectVal: pairs,
	}
}
