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
	"k8s.io/apimachinery/pkg/selection"
)

// WhenExpression allows a PipelineTask to declare expressions to be evaluated before the Task is run
// to determine whether the Task should be executed or skipped
type WhenExpression struct {
	// Input is the string for guard checking which can be a static input or an output from a parent Task
	Input string `json:"input,omitempty"`
	// Operator that represents an Input's relationship to the values
	Operator selection.Operator `json:"operator,omitempty"`
	// Values is an array of strings, which is compared against the input, for guard checking
	// It must be non-empty
	Values []string `json:"values,omitempty"`
}

// GetVarSubstitutionExpressionsForWhenExpression extracts all the values between "$(" and ")"" for a when expression
func (we *WhenExpression) GetVarSubstitutionExpressionsForWhenExpression() ([]string, bool) {
	var allExpressions []string
	allExpressions = append(allExpressions, validateString(we.Input)...)
	for _, value := range we.Values {
		allExpressions = append(allExpressions, validateString(value)...)
	}
	return allExpressions, len(allExpressions) != 0
}

func (we *WhenExpression) valuesContainsInput() bool {
	for i := range we.Values {
		if we.Values[i] == we.Input {
			return true
		}
	}
	return false
}

func (we *WhenExpression) isTrue() bool {
	if we.Operator == selection.In {
		return we.valuesContainsInput()
	}
	return !we.valuesContainsInput()
}

func (we *WhenExpression) hasVariable() bool {
	if _, hasVariable := we.GetVarSubstitutionExpressionsForWhenExpression(); hasVariable {
		return true
	}
	return false
}

func (we *WhenExpression) applyReplacements(replacements map[string]string) WhenExpression {
	replacedInput := ApplyReplacements(we.Input, replacements)

	var replacedValues []string
	for _, val := range we.Values {
		replacedValues = append(replacedValues, ApplyReplacements(val, replacements))
	}

	return WhenExpression{Input: replacedInput, Operator: we.Operator, Values: replacedValues}
}

// WhenExpressions is an array of When Expressions which are used to specify whether a Task should be executed or skipped.
type WhenExpressions []WhenExpression

// AreTrue indicates whether the When Expression are true.
func (wes WhenExpressions) AreTrue() bool {
	for _, we := range wes {
		if isTrue := we.isTrue(); !isTrue {
			return false
		}
	}
	return true
}

// HaveVariables indicates whether When Expressions have variables in the Inputs or Values.
func (wes WhenExpressions) HaveVariables() bool {
	for _, we := range wes {
		if hasVariable := we.hasVariable(); !hasVariable {
			return false
		}
	}
	return true
}

// ReplaceWhenExpressionsVariables interpolates variables in When Expressions.
func (wes WhenExpressions) ReplaceWhenExpressionsVariables(replacements map[string]string) WhenExpressions {
	var replaced []WhenExpression
	for _, we := range wes {
		replaced = append(replaced, we.applyReplacements(replacements))
	}
	return replaced
}

// WhenExpressionEvaluationResult is the result from evaluating a When Expression.
type WhenExpressionEvaluationResult struct {
	// Expression is the expression, made up of Input, Operator and Values.
	Expression *WhenExpression `json:"expression,omitempty"`
	// IsTrue is a boolean from evaluating the Expression.
	IsTrue bool `json:"result,omitempty"`
}

// WhenExpressionsEvaluationStatus are the results from evaluating When Expressions.
type WhenExpressionsEvaluationStatus struct {
	// EvaluationResults are the results from evaluating When Expressions.
	EvaluationResults []*WhenExpressionEvaluationResult `json:"evaluationResults,omitempty"`
	// Executed is a boolean that represents an AND of all results from evaluating When Expressions.
	Executed bool `json:"execute,omitempty"`
}

// GetWhenExpressionsStatus evaluates When Expressions and produces Evaluation Results.
func (wes WhenExpressions) GetWhenExpressionsStatus() WhenExpressionsEvaluationStatus {
	var evaluationResults []*WhenExpressionEvaluationResult

	execute := true
	for _, we := range wes {
		isTrue := we.isTrue()
		evaluationResults = append(evaluationResults, &WhenExpressionEvaluationResult{Expression: &we, IsTrue: isTrue})
		execute = execute && isTrue
	}

	return WhenExpressionsEvaluationStatus{EvaluationResults: evaluationResults, Executed: execute}
}
