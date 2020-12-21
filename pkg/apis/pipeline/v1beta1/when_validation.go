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
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/substitution"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var validWhenOperators = []string{
	string(selection.In),
	string(selection.NotIn),
}

func (wes WhenExpressions) validate() *apis.FieldError {
	errs := wes.validateWhenExpressionsFields().ViaField("when")
	return errs.Also(wes.validateTaskResultsVariables().ViaField("when"))
}

func (wes WhenExpressions) validateWhenExpressionsFields() (errs *apis.FieldError) {
	for idx, we := range wes {
		errs = errs.Also(we.validateWhenExpressionFields().ViaIndex(idx))
	}
	return errs
}

func (we *WhenExpression) validateWhenExpressionFields() *apis.FieldError {
	if equality.Semantic.DeepEqual(we, &WhenExpression{}) || we == nil {
		return apis.ErrMissingField(apis.CurrentField)
	}
	if !sets.NewString(validWhenOperators...).Has(string(we.GetOperator())) {
		message := fmt.Sprintf("operator %q is not recognized. valid operators: %s", we.GetOperator(), strings.Join(validWhenOperators, ","))
		return apis.ErrInvalidValue(message, apis.CurrentField)
	}
	if len(we.GetValues()) == 0 {
		return apis.ErrInvalidValue("expecting non-empty values field", apis.CurrentField)
	}
	return we.validateWhenExpressionsDuplicateFields()
}

func (we *WhenExpression) validateWhenExpressionsDuplicateFields() *apis.FieldError {
	if we.Input != "" && we.DeprecatedInput != "" {
		return apis.ErrMultipleOneOf("input", "Input")
	}
	if we.Operator != "" && we.DeprecatedOperator != "" {
		return apis.ErrMultipleOneOf("operator", "Operator")
	}
	if we.Values != nil && we.DeprecatedValues != nil {
		return apis.ErrMultipleOneOf("values", "Values")
	}
	return nil
}

func (wes WhenExpressions) validateTaskResultsVariables() *apis.FieldError {
	for idx, we := range wes {
		expressions, ok := we.GetVarSubstitutionExpressions()
		if ok {
			if LooksLikeContainsResultRefs(expressions) {
				expressions = filter(expressions, looksLikeResultRef)
				resultRefs := NewResultRefs(expressions)
				if len(expressions) != len(resultRefs) {
					message := fmt.Sprintf("expected all of the expressions %v to be result expressions but only %v were", expressions, resultRefs)
					return apis.ErrInvalidValue(message, apis.CurrentField).ViaIndex(idx)
				}
			}
		}
	}
	return nil
}

func (wes WhenExpressions) validatePipelineParametersVariables(prefix string, paramNames sets.String, arrayParamNames sets.String) (errs *apis.FieldError) {
	for idx, we := range wes {
		errs = errs.Also(validateStringVariable(we.GetInput(), prefix, paramNames, arrayParamNames).ViaField("input").ViaFieldIndex("when", idx))
		for _, val := range we.GetValues() {
			errs = errs.Also(validateStringVariable(val, prefix, paramNames, arrayParamNames).ViaField("values").ViaFieldIndex("when", idx))
		}
	}
	return errs
}
func validateStringVariable(value, prefix string, stringVars sets.String, arrayVars sets.String) *apis.FieldError {
	errs := substitution.ValidateVariableP(value, prefix, stringVars)
	return errs.Also(substitution.ValidateVariableProhibitedP(value, prefix, arrayVars))
}
