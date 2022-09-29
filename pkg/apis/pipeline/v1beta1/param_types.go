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

package v1beta1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/substitution"
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

// setDefaultsForProperties sets default type for PropertySpec (string) if it's not specified
func (pp *ParamSpec) setDefaultsForProperties() {
	for key, propertySpec := range pp.Properties {
		if propertySpec.Type == "" {
			pp.Properties[key] = PropertySpec{Type: ParamTypeString}
		}
	}
}

// ResourceParam declares a string value to use for the parameter called Name, and is used in
// the specific context of PipelineResources.
type ResourceParam = resource.ResourceParam

// Param declares an ParamValues to use for the parameter called name.
type Param struct {
	Name  string     `json:"name"`
	Value ParamValue `json:"value"`
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
	Type      ParamType `json:"type"` // Represents the stored type of ParamValues.
	StringVal string    `json:"stringVal"`
	// +listType=atomic
	ArrayVal  []string          `json:"arrayVal"`
	ObjectVal map[string]string `json:"objectVal"`
}

// ArrayOrString is deprecated, this is to keep backward compatibility
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

// ApplyReplacements applyes replacements for ParamValues type
func (paramValues *ParamValue) ApplyReplacements(stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) {
	switch paramValues.Type {
	case ParamTypeArray:
		newArrayVal := []string{}
		for _, v := range paramValues.ArrayVal {
			newArrayVal = append(newArrayVal, substitution.ApplyArrayReplacements(v, stringReplacements, arrayReplacements)...)
		}
		paramValues.ArrayVal = newArrayVal
	case ParamTypeObject:
		newObjectVal := map[string]string{}
		for k, v := range paramValues.ObjectVal {
			newObjectVal[k] = substitution.ApplyReplacements(v, stringReplacements)
		}
		paramValues.ObjectVal = newObjectVal
	default:
		paramValues.applyOrCorrect(stringReplacements, arrayReplacements, objectReplacements)
	}
}

// applyOrCorrect deals with string param whose value can be string literal or a reference to a string/array/object param/result.
// If the value of paramValues is a reference to array or object, the type will be corrected from string to array/object.
func (paramValues *ParamValue) applyOrCorrect(stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) {
	stringVal := paramValues.StringVal

	// if the stringVal is a string literal or a string that mixed with var references
	// just do the normal string replacement
	if !exactVariableSubstitutionRegex.MatchString(stringVal) {
		paramValues.StringVal = substitution.ApplyReplacements(paramValues.StringVal, stringReplacements)
		return
	}

	// trim the head "$(" and the tail ")" or "[*])"
	// i.e. get "params.name" from "$(params.name)" or "$(params.name[*])"
	trimedStringVal := substitution.StripStarVarSubExpression(stringVal)

	// if the stringVal is a reference to a string param
	if _, ok := stringReplacements[trimedStringVal]; ok {
		paramValues.StringVal = substitution.ApplyReplacements(paramValues.StringVal, stringReplacements)
	}

	// if the stringVal is a reference to an array param, we need to change the type other than apply replacement
	if _, ok := arrayReplacements[trimedStringVal]; ok {
		paramValues.StringVal = ""
		paramValues.ArrayVal = substitution.ApplyArrayReplacements(stringVal, stringReplacements, arrayReplacements)
		paramValues.Type = ParamTypeArray
	}

	// if the stringVal is a reference an object param, we need to change the type other than apply replacement
	if _, ok := objectReplacements[trimedStringVal]; ok {
		paramValues.StringVal = ""
		paramValues.ObjectVal = objectReplacements[trimedStringVal]
		paramValues.Type = ParamTypeObject
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

// ArrayReference returns the name of the parameter from array parameter reference
// returns arrayParam from $(params.arrayParam[*])
func ArrayReference(a string) string {
	return strings.TrimSuffix(strings.TrimPrefix(a, "$("+ParamsPrefix+"."), "[*])")
}

// validatePipelineParametersVariablesInTaskParameters validates param value that
// may contain the reference(s) to other params to make sure those references are used appropriately.
func validatePipelineParametersVariablesInTaskParameters(params []Param, prefix string, paramNames sets.String, arrayParamNames sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	taskParamNames := sets.NewString()
	for i, param := range params {
		if taskParamNames.Has(param.Name) {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("params names must be unique, the same param: %s is defined multiple times at", param.Name), fmt.Sprintf("params[%d].name", i)))
		}
		switch param.Value.Type {
		case ParamTypeArray:
			for idx, arrayElement := range param.Value.ArrayVal {
				errs = errs.Also(validateArrayVariable(arrayElement, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaFieldIndex("value", idx).ViaFieldKey("params", param.Name))
			}
		case ParamTypeObject:
			for key, val := range param.Value.ObjectVal {
				errs = errs.Also(validateStringVariable(val, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaFieldKey("properties", key).ViaFieldKey("params", param.Name))
			}
		default:
			errs = errs.Also(validateParamStringValue(param, prefix, paramNames, arrayParamNames, objectParamNameKeys))
		}
		taskParamNames.Insert(param.Name)
	}
	return errs
}

// validatePipelineParametersVariablesInMatrixParameters validates matrix param value
// that may contain the reference(s) to other params to make sure those references are used appropriately.
func validatePipelineParametersVariablesInMatrixParameters(matrix []Param, prefix string, paramNames sets.String, arrayParamNames sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	for _, param := range matrix {
		for idx, arrayElement := range param.Value.ArrayVal {
			errs = errs.Also(validateArrayVariable(arrayElement, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaFieldIndex("value", idx).ViaFieldKey("matrix", param.Name))
		}
	}
	return errs
}

func validateParametersInTaskMatrix(matrix *Matrix) (errs *apis.FieldError) {
	if matrix != nil {
		for _, param := range matrix.Params {
			if param.Value.Type != ParamTypeArray {
				errs = errs.Also(apis.ErrInvalidValue("parameters of type array only are allowed in matrix", "").ViaFieldKey("matrix", param.Name))
			}
		}
	}
	return errs
}

func validateParameterInOneOfMatrixOrParams(matrix *Matrix, params []Param) (errs *apis.FieldError) {
	matrixParameterNames := sets.NewString()
	if matrix != nil {
		for _, param := range matrix.Params {
			matrixParameterNames.Insert(param.Name)
		}
	}
	for _, param := range params {
		if matrixParameterNames.Has(param.Name) {
			errs = errs.Also(apis.ErrMultipleOneOf("matrix["+param.Name+"]", "params["+param.Name+"]"))
		}
	}
	return errs
}

// validateParamStringValue validates the param value field of string type
// that may contain references to other isolated array/object params other than string param.
func validateParamStringValue(param Param, prefix string, paramNames sets.String, arrayVars sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	stringValue := param.Value.StringVal

	// if the provided param value is an isolated reference to the whole array/object, we just check if the param name exists.
	isIsolated, errs := substitution.ValidateWholeArrayOrObjectRefInStringVariable(param.Name, stringValue, prefix, paramNames)
	if isIsolated {
		return errs
	}

	// if the provided param value is string literal and/or contains multiple variables
	// valid example: "$(params.myString) and another $(params.myObject.key1)"
	// invalid example: "$(params.myString) and another $(params.myObject[*])"
	return validateStringVariable(stringValue, prefix, paramNames, arrayVars, objectParamNameKeys).ViaFieldKey("params", param.Name)
}

// validateStringVariable validates the normal string fields that can only accept references to string param or individual keys of object param
func validateStringVariable(value, prefix string, stringVars sets.String, arrayVars sets.String, objectParamNameKeys map[string][]string) *apis.FieldError {
	errs := substitution.ValidateVariableP(value, prefix, stringVars)
	errs = errs.Also(validateObjectVariable(value, prefix, objectParamNameKeys))
	return errs.Also(substitution.ValidateVariableProhibitedP(value, prefix, arrayVars))
}

func validateArrayVariable(value, prefix string, stringVars sets.String, arrayVars sets.String, objectParamNameKeys map[string][]string) *apis.FieldError {
	errs := substitution.ValidateVariableP(value, prefix, stringVars)
	errs = errs.Also(validateObjectVariable(value, prefix, objectParamNameKeys))
	return errs.Also(substitution.ValidateVariableIsolatedP(value, prefix, arrayVars))
}

func validateObjectVariable(value, prefix string, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	objectNames := sets.NewString()
	for objectParamName, keys := range objectParamNameKeys {
		objectNames.Insert(objectParamName)
		errs = errs.Also(substitution.ValidateVariableP(value, fmt.Sprintf("%s\\.%s", prefix, objectParamName), sets.NewString(keys...)))
	}

	return errs.Also(substitution.ValidateEntireVariableProhibitedP(value, prefix, objectNames))
}
