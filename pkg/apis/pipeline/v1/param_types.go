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
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/substitution"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/strings/slices"
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
	// Enum declares a set of allowed param input values for tasks/pipelines that can be validated.
	// If Enum is not set, no input validation is performed for the param.
	// +optional
	Enum []string `json:"enum,omitempty"`
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

// setDefaultsForProperties sets default type for PropertySpec (string) if it's not specified
func (pp *ParamSpec) setDefaultsForProperties() {
	for key, propertySpec := range pp.Properties {
		if propertySpec.Type == "" {
			pp.Properties[key] = PropertySpec{Type: ParamTypeString}
		}
	}
}

// GetNames returns all the names of the declared parameters
func (ps ParamSpecs) GetNames() []string {
	var names []string
	for _, p := range ps {
		names = append(names, p.Name)
	}
	return names
}

// SortByType splits the input params into string params, array params, and object params, in that order
func (ps ParamSpecs) SortByType() (ParamSpecs, ParamSpecs, ParamSpecs) {
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

// ValidateNoDuplicateNames returns an error if any of the params have the same name
func (ps ParamSpecs) ValidateNoDuplicateNames() *apis.FieldError {
	var errs *apis.FieldError
	names := ps.GetNames()
	for dup := range findDups(names) {
		errs = errs.Also(apis.ErrGeneric("parameter appears more than once", "").ViaFieldKey("params", dup))
	}
	return errs
}

// validateParamEnum validates feature flag, duplication and allowed types for Param Enum
func (ps ParamSpecs) validateParamEnums(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	for _, p := range ps {
		if len(p.Enum) == 0 {
			continue
		}
		if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableParamEnum {
			errs = errs.Also(errs, apis.ErrGeneric(fmt.Sprintf("feature flag `%s` should be set to true to use Enum", config.EnableParamEnum), "").ViaKey(p.Name))
		}
		if p.Type != ParamTypeString {
			errs = errs.Also(apis.ErrGeneric("enum can only be set with string type param", "").ViaKey(p.Name))
		}
		for dup := range findDups(p.Enum) {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("parameter enum value %v appears more than once", dup), "").ViaKey(p.Name))
		}
		if p.Default != nil && p.Default.StringVal != "" {
			if !slices.Contains(p.Enum, p.Default.StringVal) {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("param default value %v not in the enum list", p.Default.StringVal), "").ViaKey(p.Name))
			}
		}
	}
	return errs
}

// findDups returns the duplicate element in the given slice
func findDups(vals []string) sets.String {
	seen := sets.String{}
	dups := sets.String{}
	for _, val := range vals {
		if seen.Has(val) {
			dups.Insert(val)
		}
		seen.Insert(val)
	}
	return dups
}

// Param declares an ParamValues to use for the parameter called name.
type Param struct {
	Name  string     `json:"name"`
	Value ParamValue `json:"value"`
}

// GetVarSubstitutionExpressions extracts all the value between "$(" and ")"" for a Parameter
func (p Param) GetVarSubstitutionExpressions() ([]string, bool) {
	var allExpressions []string
	switch p.Value.Type {
	case ParamTypeArray:
		// array type
		for _, value := range p.Value.ArrayVal {
			allExpressions = append(allExpressions, validateString(value)...)
		}
	case ParamTypeString:
		// string type
		allExpressions = append(allExpressions, validateString(p.Value.StringVal)...)
	case ParamTypeObject:
		// object type
		for _, value := range p.Value.ObjectVal {
			allExpressions = append(allExpressions, validateString(value)...)
		}
	default:
		return nil, false
	}
	return allExpressions, len(allExpressions) != 0
}

// ExtractNames returns a set of unique names
func (ps Params) ExtractNames() sets.String {
	names := sets.String{}
	for _, p := range ps {
		names.Insert(p.Name)
	}
	return names
}

func (ps Params) extractValues() []string {
	pvs := []string{}
	for i := range ps {
		pvs = append(pvs, ps[i].Value.StringVal)
		pvs = append(pvs, ps[i].Value.ArrayVal...)
		for _, v := range ps[i].Value.ObjectVal {
			pvs = append(pvs, v)
		}
	}
	return pvs
}

// extractParamMapArrVals creates a param map with the key: param.Name and
// val: param.Value.ArrayVal
func (ps Params) extractParamMapArrVals() map[string][]string {
	paramsMap := make(map[string][]string)
	for _, p := range ps {
		paramsMap[p.Name] = p.Value.ArrayVal
	}
	return paramsMap
}

// ParseTaskandResultName parses "task name", "result name" from a Matrix Context Variable
// Valid Example 1:
// - Input: tasks.myTask.matrix.length
// - Output: "myTask", ""
// Valid Example 2:
// - Input: tasks.myTask.matrix.ResultName.length
// - Output: "myTask", "ResultName"
func (p Param) ParseTaskandResultName() (string, string) {
	if expressions, ok := p.GetVarSubstitutionExpressions(); ok {
		for _, expression := range expressions {
			subExpressions := strings.Split(expression, ".")
			pipelineTaskName := subExpressions[1]
			if len(subExpressions) == 4 {
				return pipelineTaskName, ""
			} else if len(subExpressions) == 5 {
				resultName := subExpressions[3]
				return pipelineTaskName, resultName
			}
		}
	}
	return "", ""
}

// Params is a list of Param
type Params []Param

// ExtractParamArrayLengths extract and return the lengths of all array params
// Example of returned value: {"a-array-params": 2,"b-array-params": 2 }
func (ps Params) ExtractParamArrayLengths() map[string]int {
	// Collect all array params
	arrayParamsLengths := make(map[string]int)

	// Collect array params lengths from params
	for _, p := range ps {
		if p.Value.Type == ParamTypeArray {
			arrayParamsLengths[p.Name] = len(p.Value.ArrayVal)
		}
	}
	return arrayParamsLengths
}

// validateDuplicateParameters checks if a parameter with the same name is defined more than once
func (ps Params) validateDuplicateParameters() (errs *apis.FieldError) {
	taskParamNames := sets.NewString()
	for i, param := range ps {
		if taskParamNames.Has(param.Name) {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("parameter names must be unique,"+
				" the parameter \"%s\" is also defined at", param.Name), fmt.Sprintf("[%d].name", i)))
		}
		taskParamNames.Insert(param.Name)
	}
	return errs
}

// ReplaceVariables applies string, array and object replacements to variables in Params
func (ps Params) ReplaceVariables(stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) Params {
	params := ps.DeepCopy()
	for i := range params {
		params[i].Value.ApplyReplacements(stringReplacements, arrayReplacements, objectReplacements)
	}
	return params
}

// ExtractDefaultParamArrayLengths extract and return the lengths of all array params
// Example of returned value: {"a-array-params": 2,"b-array-params": 2 }
func (ps ParamSpecs) ExtractDefaultParamArrayLengths() map[string]int {
	// Collect all array params
	arrayParamsLengths := make(map[string]int)

	// Collect array params lengths from defaults
	for _, p := range ps {
		if p.Default != nil {
			if p.Default.Type == ParamTypeArray {
				arrayParamsLengths[p.Name] = len(p.Default.ArrayVal)
			}
		}
	}
	return arrayParamsLengths
}

// extractArrayIndexingParamRefs takes a string of the form `foo-$(params.array-param[1])-bar` and extracts the portions of the string that reference an element in an array param.
// For example, for the string “foo-$(params.array-param[1])-bar-$(params.other-array-param[2])-$(params.string-param)`,
// it would return ["$(params.array-param[1])", "$(params.other-array-param[2])"].
func extractArrayIndexingParamRefs(paramReference string) []string {
	l := []string{}
	list := substitution.ExtractArrayIndexingParamsExpressions(paramReference)
	for _, val := range list {
		indexString := substitution.ExtractIndexString(val)
		if indexString != "" {
			l = append(l, val)
		}
	}
	return l
}

// extractParamRefsFromSteps get all array indexing references from steps
func extractParamRefsFromSteps(steps []Step) []string {
	paramsRefs := []string{}
	for _, step := range steps {
		paramsRefs = append(paramsRefs, step.Script)
		container := step.ToK8sContainer()
		paramsRefs = append(paramsRefs, extractParamRefsFromContainer(container)...)
	}
	return paramsRefs
}

// extractParamRefsFromStepTemplate get all array indexing references from StepsTemplate
func extractParamRefsFromStepTemplate(stepTemplate *StepTemplate) []string {
	if stepTemplate == nil {
		return nil
	}
	container := stepTemplate.ToK8sContainer()
	return extractParamRefsFromContainer(container)
}

// extractParamRefsFromSidecars get all array indexing references from sidecars
func extractParamRefsFromSidecars(sidecars []Sidecar) []string {
	paramsRefs := []string{}
	for _, s := range sidecars {
		paramsRefs = append(paramsRefs, s.Script)
		container := s.ToK8sContainer()
		paramsRefs = append(paramsRefs, extractParamRefsFromContainer(container)...)
	}
	return paramsRefs
}

// extractParamRefsFromVolumes get all array indexing references from volumes
func extractParamRefsFromVolumes(volumes []corev1.Volume) []string {
	paramsRefs := []string{}
	for i, v := range volumes {
		paramsRefs = append(paramsRefs, v.Name)
		if v.VolumeSource.ConfigMap != nil {
			paramsRefs = append(paramsRefs, v.ConfigMap.Name)
			for _, item := range v.ConfigMap.Items {
				paramsRefs = append(paramsRefs, item.Key)
				paramsRefs = append(paramsRefs, item.Path)
			}
		}
		if v.VolumeSource.Secret != nil {
			paramsRefs = append(paramsRefs, v.Secret.SecretName)
			for _, item := range v.Secret.Items {
				paramsRefs = append(paramsRefs, item.Key)
				paramsRefs = append(paramsRefs, item.Path)
			}
		}
		if v.PersistentVolumeClaim != nil {
			paramsRefs = append(paramsRefs, v.PersistentVolumeClaim.ClaimName)
		}
		if v.Projected != nil {
			for _, s := range volumes[i].Projected.Sources {
				if s.ConfigMap != nil {
					paramsRefs = append(paramsRefs, s.ConfigMap.Name)
				}
				if s.Secret != nil {
					paramsRefs = append(paramsRefs, s.Secret.Name)
				}
				if s.ServiceAccountToken != nil {
					paramsRefs = append(paramsRefs, s.ServiceAccountToken.Audience)
				}
			}
		}
		if v.CSI != nil {
			if v.CSI.NodePublishSecretRef != nil {
				paramsRefs = append(paramsRefs, v.CSI.NodePublishSecretRef.Name)
			}
			if v.CSI.VolumeAttributes != nil {
				for _, value := range v.CSI.VolumeAttributes {
					paramsRefs = append(paramsRefs, value)
				}
			}
		}
	}
	return paramsRefs
}

// extractParamRefsFromContainer get all array indexing references from container
func extractParamRefsFromContainer(c *corev1.Container) []string {
	paramsRefs := []string{}
	paramsRefs = append(paramsRefs, c.Name)
	paramsRefs = append(paramsRefs, c.Image)
	paramsRefs = append(paramsRefs, string(c.ImagePullPolicy))
	paramsRefs = append(paramsRefs, c.Args...)

	for ie, e := range c.Env {
		paramsRefs = append(paramsRefs, e.Value)
		if c.Env[ie].ValueFrom != nil {
			if e.ValueFrom.SecretKeyRef != nil {
				paramsRefs = append(paramsRefs, e.ValueFrom.SecretKeyRef.LocalObjectReference.Name)
				paramsRefs = append(paramsRefs, e.ValueFrom.SecretKeyRef.Key)
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				paramsRefs = append(paramsRefs, e.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name)
				paramsRefs = append(paramsRefs, e.ValueFrom.ConfigMapKeyRef.Key)
			}
		}
	}

	for _, e := range c.EnvFrom {
		paramsRefs = append(paramsRefs, e.Prefix)
		if e.ConfigMapRef != nil {
			paramsRefs = append(paramsRefs, e.ConfigMapRef.LocalObjectReference.Name)
		}
		if e.SecretRef != nil {
			paramsRefs = append(paramsRefs, e.SecretRef.LocalObjectReference.Name)
		}
	}

	paramsRefs = append(paramsRefs, c.WorkingDir)
	paramsRefs = append(paramsRefs, c.Command...)

	for _, v := range c.VolumeMounts {
		paramsRefs = append(paramsRefs, v.Name)
		paramsRefs = append(paramsRefs, v.MountPath)
		paramsRefs = append(paramsRefs, v.SubPath)
	}
	return paramsRefs
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
	case ParamTypeString:
		fallthrough
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
func validatePipelineParametersVariablesInTaskParameters(params Params, prefix string, paramNames sets.String, arrayParamNames sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	errs = errs.Also(params.validateDuplicateParameters()).ViaField("params")
	for _, param := range params {
		switch param.Value.Type {
		case ParamTypeArray:
			for idx, arrayElement := range param.Value.ArrayVal {
				errs = errs.Also(validateArrayVariable(arrayElement, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaFieldIndex("value", idx).ViaFieldKey("params", param.Name))
			}
		case ParamTypeObject:
			for key, val := range param.Value.ObjectVal {
				errs = errs.Also(validateStringVariable(val, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaFieldKey("properties", key).ViaFieldKey("params", param.Name))
			}
		case ParamTypeString:
			fallthrough
		default:
			errs = errs.Also(validateParamStringValue(param, prefix, paramNames, arrayParamNames, objectParamNameKeys))
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
	errs := substitution.ValidateNoReferencesToUnknownVariables(value, prefix, stringVars)
	errs = errs.Also(validateObjectVariable(value, prefix, objectParamNameKeys))
	return errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(value, prefix, arrayVars))
}

func validateArrayVariable(value, prefix string, stringVars sets.String, arrayVars sets.String, objectParamNameKeys map[string][]string) *apis.FieldError {
	errs := substitution.ValidateNoReferencesToUnknownVariables(value, prefix, stringVars)
	errs = errs.Also(validateObjectVariable(value, prefix, objectParamNameKeys))
	return errs.Also(substitution.ValidateVariableReferenceIsIsolated(value, prefix, arrayVars))
}

func validateObjectVariable(value, prefix string, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	objectNames := sets.NewString()
	for objectParamName, keys := range objectParamNameKeys {
		objectNames.Insert(objectParamName)
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(value, fmt.Sprintf("%s\\.%s", prefix, objectParamName), sets.NewString(keys...)))
	}

	return errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(value, prefix, objectNames))
}
