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

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/substitution"
)

// exactVariableSubstitutionFormat matches strings that only contain a single reference to result or param variables, but nothing else
// i.e. `$(result.resultname)` is a match, but `foo $(result.resultname)` is not.
const exactVariableSubstitutionFormat = `^\$\([_a-zA-Z0-9.-]+(\.[_a-zA-Z0-9.-]+)*(\[([0-9]+|\*)\])?\)$`

var exactVariableSubstitutionRegex = regexp.MustCompile(exactVariableSubstitutionFormat)

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
	Default *ArrayOrString `json:"default,omitempty"`
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

// Param declares an ArrayOrString to use for the parameter called name.
type Param struct {
	Name  string        `json:"name"`
	Value ArrayOrString `json:"value"`
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

// ArrayOrString is modeled after IntOrString in kubernetes/apimachinery:

// ArrayOrString is a type that can hold a single string or string array.
// Used in JSON unmarshalling so that a single JSON field can accept
// either an individual string or an array of strings.
// TODO (@chuangw6): This struct will be renamed or be embedded in a new struct to take into
// consideration the object case after the community reaches an agreement on it.
type ArrayOrString struct {
	Type      ParamType `json:"type"` // Represents the stored type of ArrayOrString.
	StringVal string    `json:"stringVal"`
	// +listType=atomic
	ArrayVal  []string          `json:"arrayVal"`
	ObjectVal map[string]string `json:"objectVal"`
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (arrayOrString *ArrayOrString) UnmarshalJSON(value []byte) error {
	// ArrayOrString is used for Results Value as well, the results can be any kind of
	// data so we need to check if it is empty.
	if len(value) == 0 {
		arrayOrString.Type = ParamTypeString
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
			arrayOrString.Type = ParamTypeArray
			arrayOrString.ArrayVal = a
			return nil
		}
	}
	if value[0] == '{' {
		// if failed to unmarshal to map, we will convert the value to string and marshal it to string
		var m map[string]string
		if err := json.Unmarshal(value, &m); err == nil {
			arrayOrString.Type = ParamTypeObject
			arrayOrString.ObjectVal = m
			return nil
		}
	}

	// By default we unmarshal to string
	arrayOrString.Type = ParamTypeString
	if err := json.Unmarshal(value, &arrayOrString.StringVal); err == nil {
		return nil
	}
	arrayOrString.StringVal = string(value)

	return nil
}

// MarshalJSON implements the json.Marshaller interface.
func (arrayOrString ArrayOrString) MarshalJSON() ([]byte, error) {
	switch arrayOrString.Type {
	case ParamTypeString:
		return json.Marshal(arrayOrString.StringVal)
	case ParamTypeArray:
		return json.Marshal(arrayOrString.ArrayVal)
	case ParamTypeObject:
		return json.Marshal(arrayOrString.ObjectVal)
	default:
		return []byte{}, fmt.Errorf("impossible ArrayOrString.Type: %q", arrayOrString.Type)
	}
}

// ApplyReplacements applyes replacements for ArrayOrString type
func (arrayOrString *ArrayOrString) ApplyReplacements(stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) {
	switch arrayOrString.Type {
	case ParamTypeArray:
		var newArrayVal []string
		for _, v := range arrayOrString.ArrayVal {
			newArrayVal = append(newArrayVal, substitution.ApplyArrayReplacements(v, stringReplacements, arrayReplacements)...)
		}
		arrayOrString.ArrayVal = newArrayVal
	case ParamTypeObject:
		newObjectVal := map[string]string{}
		for k, v := range arrayOrString.ObjectVal {
			newObjectVal[k] = substitution.ApplyReplacements(v, stringReplacements)
		}
		arrayOrString.ObjectVal = newObjectVal
	default:
		arrayOrString.applyOrCorrect(stringReplacements, arrayReplacements, objectReplacements)
	}
}

// applyOrCorrect deals with string param whose value can be string literal or a reference to a string/array/object param/result.
// If the value of arrayOrString is a reference to array or object, the type will be corrected from string to array/object.
func (arrayOrString *ArrayOrString) applyOrCorrect(stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) {
	stringVal := arrayOrString.StringVal

	// if the stringVal is a string literal or a string that mixed with var references
	// just do the normal string replacement
	if !exactVariableSubstitutionRegex.MatchString(stringVal) {
		arrayOrString.StringVal = substitution.ApplyReplacements(arrayOrString.StringVal, stringReplacements)
		return
	}

	// trim the head "$(" and the tail ")" or "[*])"
	// i.e. get "params.name" from "$(params.name)" or "$(params.name[*])"
	trimedStringVal := StripStarVarSubExpression(stringVal)

	// if the stringVal is a reference to a string param
	if _, ok := stringReplacements[trimedStringVal]; ok {
		arrayOrString.StringVal = substitution.ApplyReplacements(arrayOrString.StringVal, stringReplacements)
	}

	// if the stringVal is a reference to an array param, we need to change the type other than apply replacement
	if _, ok := arrayReplacements[trimedStringVal]; ok {
		arrayOrString.StringVal = ""
		arrayOrString.ArrayVal = substitution.ApplyArrayReplacements(stringVal, stringReplacements, arrayReplacements)
		arrayOrString.Type = ParamTypeArray
	}

	// if the stringVal is a reference an object param, we need to change the type other than apply replacement
	if _, ok := objectReplacements[trimedStringVal]; ok {
		arrayOrString.StringVal = ""
		arrayOrString.ObjectVal = objectReplacements[trimedStringVal]
		arrayOrString.Type = ParamTypeObject
	}
}

// StripStarVarSubExpression strips "$(target[*])"" to get "target"
func StripStarVarSubExpression(s string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(s, "$("), ")"), "[*]")
}

// NewArrayOrString creates an ArrayOrString of type ParamTypeString or ParamTypeArray, based on
// how many inputs are given (>1 input will create an array, not string).
func NewArrayOrString(value string, values ...string) *ArrayOrString {
	if len(values) > 0 {
		return &ArrayOrString{
			Type:     ParamTypeArray,
			ArrayVal: append([]string{value}, values...),
		}
	}
	return &ArrayOrString{
		Type:      ParamTypeString,
		StringVal: value,
	}
}

// NewObject creates an ArrayOrString of type ParamTypeObject using the provided key-value pairs
func NewObject(pairs map[string]string) *ArrayOrString {
	return &ArrayOrString{
		Type:      ParamTypeObject,
		ObjectVal: pairs,
	}
}

// ArrayReference returns the name of the parameter from array parameter reference
// returns arrayParam from $(params.arrayParam[*])
func ArrayReference(a string) string {
	return strings.TrimSuffix(strings.TrimPrefix(a, "$("+ParamsPrefix+"."), "[*])")
}
