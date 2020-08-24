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

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var validWhenOperators = []string{
	string(selection.In),
	string(selection.NotIn),
}

// Validate looks at the provided When Expression and makes sure it's valid.
// Specifically, ensures that all the fields are included, operators are valid,
// and values array is non-empty.
func (whenExpression *WhenExpression) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(whenExpression, &WhenExpression{}) || whenExpression == nil {
		return apis.ErrMissingField(apis.CurrentField)
	}
	if err := validateWhenExpressionOperator(&whenExpression.Operator); err != nil {
		return err
	}
	if err := validateWhenExpressionValues(&whenExpression.Values); err != nil {
		return err
	}
	return nil
}

func validateWhenExpressionOperator(operator *selection.Operator) *apis.FieldError {
	if !sets.NewString(validWhenOperators...).Has(string(*operator)) {
		message := fmt.Sprintf("operator %q is not recognized. valid operators: %s", *operator, strings.Join(validWhenOperators, ","))
		return apis.ErrInvalidValue(message, "spec.task.when")
	}
	return nil
}

func validateWhenExpressionValues(values *[]string) *apis.FieldError {
	if len(*values) == 0 {
		return apis.ErrInvalidValue("expecting non-empty values field", "spec.task.when")
	}
	return nil
}

func (whenExpressions WhenExpressions) validateReferencesToTaskResults() error {
	for _, we := range whenExpressions {
		expressions, ok := we.GetVarSubstitutionExpressionsForWhenExpression()
		if ok {
			if LooksLikeContainsResultRefs(expressions) {
				expressions = filter(expressions, looksLikeResultRef)
				resultRefs := NewResultRefs(expressions)
				if len(expressions) != len(resultRefs) {
					return fmt.Errorf("expected all of the expressions %v to be result expressions but only %v were", expressions, resultRefs)
				}
			}
		}
	}
	return nil
}

func (whenExpressions WhenExpressions) validateReferencesToParameters(prefix string, paramNames sets.String, arrayParamNames sets.String) *apis.FieldError {
	for _, whenExpression := range whenExpressions {
		if err := validatePipelineStringVariable(fmt.Sprintf("whenInput[%s]", whenExpression.Input), whenExpression.Input, prefix, paramNames, arrayParamNames); err != nil {
			return err
		}
		for _, whenExpressionValue := range whenExpression.Values {
			if err := validatePipelineStringVariable(fmt.Sprintf("whenValue[%s]", whenExpressionValue), whenExpressionValue, prefix, paramNames, arrayParamNames); err != nil {
				return err
			}
		}
	}
	return nil
}
