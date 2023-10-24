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
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var validWhenOperators = []string{
	string(selection.In),
	string(selection.NotIn),
}

func (wes WhenExpressions) validate(ctx context.Context) *apis.FieldError {
	return wes.validateWhenExpressionsFields(ctx).ViaField("when")
}

func (wes WhenExpressions) validateWhenExpressionsFields(ctx context.Context) (errs *apis.FieldError) {
	for idx, we := range wes {
		errs = errs.Also(we.validateWhenExpressionFields(ctx).ViaIndex(idx))
	}
	return errs
}

func (we *WhenExpression) validateWhenExpressionFields(ctx context.Context) *apis.FieldError {
	if we.CEL != "" {
		if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableCELInWhenExpression {
			return apis.ErrGeneric("feature flag %s should be set to true to use CEL: %s in WhenExpression", config.EnableCELInWhenExpression, we.CEL)
		}
		if we.Input != "" || we.Operator != "" || len(we.Values) != 0 {
			return apis.ErrGeneric(fmt.Sprintf("cel and input+operator+values cannot be set in one WhenExpression: %v", we))
		}

		// We need to compile the CEL expression and check if it is a valid expression
		// note that at the validation webhook, Tekton's variables are not substituted,
		// so they need to be wrapped with single quotes.
		// e.g.  This is a valid CEL expression: '$(params.foo)' == 'foo';
		//       But this is not a valid expression since CEL cannot recognize: $(params.foo) == 'foo';
		//       This is not valid since we don't pass params to CEL's environment: params.foo == 'foo';
		env, _ := cel.NewEnv()
		_, iss := env.Compile(we.CEL)
		if iss.Err() != nil {
			return apis.ErrGeneric("invalid cel expression: %s with err: %s", we.CEL, iss.Err().Error())
		}
		return nil
	}

	if equality.Semantic.DeepEqual(we, &WhenExpression{}) || we == nil {
		return apis.ErrMissingField(apis.CurrentField)
	}
	if !sets.NewString(validWhenOperators...).Has(string(we.Operator)) {
		message := fmt.Sprintf("operator %q is not recognized. valid operators: %s", we.Operator, strings.Join(validWhenOperators, ","))
		return apis.ErrInvalidValue(message, apis.CurrentField)
	}
	if len(we.Values) == 0 {
		return apis.ErrInvalidValue("expecting non-empty values field", apis.CurrentField)
	}
	return nil
}

func (wes WhenExpressions) validatePipelineParametersVariables(prefix string, paramNames sets.String, arrayParamNames sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	for idx, we := range wes {
		errs = errs.Also(validateStringVariable(we.Input, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaField("input").ViaFieldIndex("when", idx))
		for _, val := range we.Values {
			// one of the values could be a reference to an array param, such as, $(params.foo[*])
			// extract the variable name from the pattern $(params.foo[*]), if the variable name matches with one of the array params
			// validate the param as an array variable otherwise, validate it as a string variable
			if arrayParamNames.Has(ArrayReference(val)) {
				errs = errs.Also(validateArrayVariable(val, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaField("values").ViaFieldIndex("when", idx))
			} else {
				errs = errs.Also(validateStringVariable(val, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaField("values").ViaFieldIndex("when", idx))
			}
		}
	}
	return errs
}
