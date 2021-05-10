/*
Copyright 2020 The Tekton Authors

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
	"github.com/tektoncd/pipeline/pkg/substitution"
	"k8s.io/apimachinery/pkg/selection"
)

// WhenExpression allows a PipelineTask to declare expressions to be evaluated before the Task is run
// to determine whether the Task should be executed or skipped
type WhenExpression struct {
	// Input is the string for guard checking which can be a static input or an output from a parent Task
	Input string `json:"input"`

	// DeprecatedInput for backwards compatibility with <v0.17
	// it is the string for guard checking which can be a static input or an output from a parent Task
	// +optional
	DeprecatedInput string `json:"Input,omitempty"`

	// Operator that represents an Input's relationship to the values
	Operator selection.Operator `json:"operator"`

	// DeprecatedOperator for backwards compatibility with <v0.17
	// it represents a DeprecatedInput's relationship to the DeprecatedValues
	// +optional
	DeprecatedOperator selection.Operator `json:"Operator,omitempty"`

	// Values is an array of strings, which is compared against the input, for guard checking
	// It must be non-empty
	Values []string `json:"values"`

	// DeprecatedValues for backwards compatibility with <v0.17
	// it represents a DeprecatedInput's relationship to the DeprecatedValues
	// +optional
	DeprecatedValues []string `json:"Values,omitempty"`
}

// GetInput returns the input string for guard checking
// based on DeprecatedInput (<v0.17) and Input
func (we *WhenExpression) GetInput() string {
	if we.Input == "" {
		return we.DeprecatedInput
	}
	return we.Input
}

// GetOperator returns the relationship between input and values
// based on DeprecatedOperator (<v0.17) and Operator
func (we *WhenExpression) GetOperator() selection.Operator {
	if we.Operator == "" {
		return we.DeprecatedOperator
	}
	return we.Operator
}

// GetValues returns an array of strings which is compared against the input
// based on DeprecatedValues (<v0.17) and Values
func (we *WhenExpression) GetValues() []string {
	if we.Values == nil {
		return we.DeprecatedValues
	}
	return we.Values
}

func (we *WhenExpression) isInputInValues() bool {
	values := we.GetValues()
	for i := range values {
		if values[i] == we.GetInput() {
			return true
		}
	}
	return false
}

func (we *WhenExpression) isTrue() bool {
	if we.GetOperator() == selection.In {
		return we.isInputInValues()
	}
	// selection.NotIn
	return !we.isInputInValues()
}

func (we *WhenExpression) hasVariable() bool {
	if _, hasVariable := we.GetVarSubstitutionExpressions(); hasVariable {
		return true
	}
	return false
}

func (we *WhenExpression) applyReplacements(replacements map[string]string) WhenExpression {
	replacedInput := substitution.ApplyReplacements(we.GetInput(), replacements)

	var replacedValues []string
	for _, val := range we.GetValues() {
		replacedValues = append(replacedValues, substitution.ApplyReplacements(val, replacements))
	}

	return WhenExpression{Input: replacedInput, Operator: we.GetOperator(), Values: replacedValues}
}

// GetVarSubstitutionExpressions extracts all the values between "$(" and ")" in a When Expression
func (we *WhenExpression) GetVarSubstitutionExpressions() ([]string, bool) {
	var allExpressions []string
	allExpressions = append(allExpressions, validateString(we.GetInput())...)
	for _, value := range we.GetValues() {
		allExpressions = append(allExpressions, validateString(value)...)
	}
	return allExpressions, len(allExpressions) != 0
}

// WhenExpressions are used to specify whether a Task should be executed or skipped
// All of them need to evaluate to True for a guarded Task to be executed.
type WhenExpressions []WhenExpression

// AllowsExecution evaluates an Input's relationship to an array of Values, based on the Operator,
// to determine whether all the When Expressions are True. If they are all True, the guarded Task is
// executed, otherwise it is skipped.
func (wes WhenExpressions) AllowsExecution() bool {
	for _, we := range wes {
		if !we.isTrue() {
			return false
		}
	}
	return true
}

// HaveVariables indicates whether When Expressions contains variables, such as Parameters
// or Results in the Inputs or Values.
func (wes WhenExpressions) HaveVariables() bool {
	for _, we := range wes {
		if we.hasVariable() {
			return true
		}
	}
	return false
}

// ReplaceWhenExpressionsVariables interpolates variables, such as Parameters and Results, in
// the Input and Values.
func (wes WhenExpressions) ReplaceWhenExpressionsVariables(replacements map[string]string) WhenExpressions {
	replaced := wes
	for i := range wes {
		replaced[i] = wes[i].applyReplacements(replacements)
	}
	return replaced
}
