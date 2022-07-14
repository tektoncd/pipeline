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

package config

import (
	"context"
)

// isSubstituted is used for associating the parameter substitution inside the context.Context.
type isSubstituted struct{}

// validatePropagatedVariables is used for deciding whether to validate or skip parameters and workspaces inside the contect.Context.
type validatePropagatedVariables string

// WithinSubstituted is used to note that it is calling within
// the context of a substitute variable operation.
func WithinSubstituted(ctx context.Context) context.Context {
	return context.WithValue(ctx, isSubstituted{}, true)
}

// IsSubstituted indicates that the variables have been substituted.
func IsSubstituted(ctx context.Context) bool {
	return ctx.Value(isSubstituted{}) != nil
}

// SkipValidationDueToPropagatedParametersAndWorkspaces sets the context to skip validation of parameters when embedded vs referenced to true or false.
func SkipValidationDueToPropagatedParametersAndWorkspaces(ctx context.Context, skip bool) context.Context {
	return context.WithValue(ctx, validatePropagatedVariables("ValidatePropagatedParameterVariablesAndWorkspaces"), !skip)
}

// ValidateParameterVariablesAndWorkspaces indicates if validation of paramater variables and workspaces should be conducted.
func ValidateParameterVariablesAndWorkspaces(ctx context.Context) bool {
	return ctx.Value(validatePropagatedVariables("ValidatePropagatedParameterVariablesAndWorkspaces")) == true
}
