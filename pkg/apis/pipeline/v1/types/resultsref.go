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
