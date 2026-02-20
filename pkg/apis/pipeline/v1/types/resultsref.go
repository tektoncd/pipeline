/*
Copyright 2025 The Tekton Authors

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

package types

import (
	"regexp"
	"strings"
)

const (
	// TODO(#2462) use one regex across all substitutions
	// variableSubstitutionFormat matches format like $result.resultname, $result.resultname[int] and $result.resultname[*]
	variableSubstitutionFormat = `\$\([_a-zA-Z0-9.-]+(\.[_a-zA-Z0-9.-]+)*(\[([0-9]+|\*)\])?\)`
)

// VariableSubstitutionRegex is a regex to find all result matching substitutions
var VariableSubstitutionRegex = regexp.MustCompile(variableSubstitutionFormat)

func stripVarSubExpression(expression string) string {
	return strings.TrimSuffix(strings.TrimPrefix(expression, "$("), ")")
}

func validateString(value string) []string {
	expressions := VariableSubstitutionRegex.FindAllString(value, -1)
	if expressions == nil {
		return nil
	}
	var result []string
	for _, expression := range expressions {
		result = append(result, stripVarSubExpression(expression))
	}
	return result
}
