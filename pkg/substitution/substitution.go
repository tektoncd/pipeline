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

package substitution

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

const (
	parameterSubstitution = `.*?(\[\*\])?`

	// braceMatchingRegex is a regex for parameter references including dot notation, bracket notation with single and double quotes.
	braceMatchingRegex = "(\\$(\\(%s(\\.(?P<var1>%s)|\\[\"(?P<var2>%s)\"\\]|\\['(?P<var3>%s)'\\])\\)))"
	// arrayIndexing will match all `[int]` and `[*]` for parseExpression
	arrayIndexing = `\[([0-9])*\*?\]`
	// paramIndex will match all `$(params.paramName[int])` expressions
	paramIndexing = `\$\(params(\.[_a-zA-Z0-9.-]+|\[\'[_a-zA-Z0-9.-\/]+\'\]|\[\"[_a-zA-Z0-9.-\/]+\"\])\[[0-9]+\]\)`
	// intIndex will match all `[int]` expressions
	intIndex = `\[[0-9]+\]`
)

// arrayIndexingRegex is used to match `[int]` and `[*]`
var arrayIndexingRegex = regexp.MustCompile(arrayIndexing)

// paramIndexingRegex will match all `$(params.paramName[int])` expressions
var paramIndexingRegex = regexp.MustCompile(paramIndexing)

// intIndexRegex will match all `[int]` for param expression
var intIndexRegex = regexp.MustCompile(intIndex)

// ValidateNoReferencesToUnknownVariables returns an error if the input string contains references to unknown variables
// Inputs:
// - value: a string containing a reference to a variable that can be substituted, e.g. "echo $(params.foo)"
// - prefix: the prefix of the substitutable variable, e.g. "params" or "context.pipeline"
// - vars: names of known variables
func ValidateNoReferencesToUnknownVariables(value, prefix string, vars sets.String) *apis.FieldError {
	if vs, present, errString := ExtractVariablesFromString(value, prefix); present {
		if errString != "" {
			return &apis.FieldError{
				Message: errString,
				Paths:   []string{""},
			}
		}
		for _, v := range vs {
			v = TrimArrayIndex(v)
			if !vars.Has(v) {
				return &apis.FieldError{
					Message: fmt.Sprintf("non-existent variable in %q", value),
					// Empty path is required to make the `ViaField`, … work
					Paths: []string{""},
				}
			}
		}
	}
	return nil
}

// ValidateNoReferencesToProhibitedVariables returns an error if the input string contains any references to any variables in vars,
// except for array indexing references.
//
// Inputs:
// - value: a string containing a reference to a variable that can be substituted, e.g. "echo $(params.foo)"
// - prefix: the prefix of the substitutable variable, e.g. "params" or "context.pipeline"
// - vars: names of known variables
func ValidateNoReferencesToProhibitedVariables(value, prefix string, vars sets.String) *apis.FieldError {
	if vs, present, errString := ExtractVariablesFromString(value, prefix); present {
		if errString != "" {
			return &apis.FieldError{
				Message: errString,
				Paths:   []string{""},
			}
		}
		for _, v := range vs {
			v = strings.TrimSuffix(v, "[*]")
			if vars.Has(v) {
				return &apis.FieldError{
					Message: fmt.Sprintf("variable type invalid in %q", value),
					// Empty path is required to make the `ViaField`, … work
					Paths: []string{""},
				}
			}
		}
	}
	return nil
}

// ValidateNoReferencesToEntireProhibitedVariables returns an error if the input string contains any whole array/object references
// to any variables in vars. References to array indexes or object keys are permitted.
//
// Inputs:
// - value: a string containing a reference to a variable that can be substituted, e.g. "echo $(params.foo)"
// - prefix: the prefix of the substitutable variable, e.g. "params" or "context.pipeline"
// - vars: names of known variables
func ValidateNoReferencesToEntireProhibitedVariables(value, prefix string, vars sets.String) *apis.FieldError {
	paths := []string{""} // Empty path is required to make the `ViaField`, … work
	vs, err := extractEntireVariablesFromString(value, prefix)
	if err != nil {
		return &apis.FieldError{
			Message: fmt.Sprintf("extractEntireVariablesFromString failed : %v", err),
			Paths:   paths,
		}
	}

	for _, v := range vs {
		v = strings.TrimSuffix(v, "[*]")
		if vars.Has(v) {
			return &apis.FieldError{
				Message: fmt.Sprintf("variable type invalid in %q", value),
				Paths:   paths,
			}
		}
	}

	return nil
}

// ValidateVariableReferenceIsIsolated returns an error if the input string contains characters in addition to references to known parameters.
// For example, if "foo" is a known parameter, a value of "foo: $(params.foo)" returns an error, but a value of "$(params.foo)" does not.
// Inputs:
// - value: a string containing a reference to a variable that can be substituted, e.g. "echo $(params.foo)"
// - prefix: the prefix of the substitutable variable, e.g. "params" or "context.pipeline"
// - vars: names of known variables
func ValidateVariableReferenceIsIsolated(value, prefix string, vars sets.String) *apis.FieldError {
	paths := []string{""} // Empty path is required to make the `ViaField`, … work
	if vs, present, errString := ExtractVariablesFromString(value, prefix); present {
		if errString != "" {
			return &apis.FieldError{
				Message: errString,
				Paths:   paths,
			}
		}
		firstMatch, err := extractExpressionFromString(value, prefix)
		if err != nil {
			return &apis.FieldError{
				Message: err.Error(),
				Paths:   paths,
			}
		}
		for _, v := range vs {
			v = strings.TrimSuffix(v, "[*]")
			if vars.Has(v) {
				if len(value) != len(firstMatch) {
					return &apis.FieldError{
						Message: fmt.Sprintf("variable is not properly isolated in %q", value),
						Paths:   paths,
					}
				}
			}
		}
	}
	return nil
}

// ValidateWholeArrayOrObjectRefInStringVariable validates if a single string field uses references to the whole array/object appropriately
// valid example: "$(params.myObject[*])"
// invalid example: "$(params.name-not-exist[*])"
func ValidateWholeArrayOrObjectRefInStringVariable(name, value, prefix string, vars sets.String) (isIsolated bool, errs *apis.FieldError) {
	nameSubstitution := `[_a-zA-Z0-9.-]+\[\*\]`

	// a regex to check if the stringValue is an isolated reference to the whole array/object param without extra string literal.
	isolatedVariablePattern := fmt.Sprintf(fmt.Sprintf("^%s$", braceMatchingRegex), prefix, nameSubstitution, nameSubstitution, nameSubstitution)
	isolatedVariableRegex, err := regexp.Compile(isolatedVariablePattern)
	if err != nil {
		return false, &apis.FieldError{
			Message: fmt.Sprint("Fail to parse the regex: ", err),
			Paths:   []string{fmt.Sprintf("%s.%s", prefix, name)},
		}
	}

	if isolatedVariableRegex.MatchString(value) {
		return true, ValidateNoReferencesToUnknownVariables(value, prefix, vars).ViaFieldKey(prefix, name)
	}

	return false, nil
}

// extract a the first full string expressions found (e.g "$(input.params.foo)").
// Returns "" if nothing is found.
func extractExpressionFromString(s, prefix string) (string, error) {
	pattern := fmt.Sprintf(braceMatchingRegex, prefix, parameterSubstitution, parameterSubstitution, parameterSubstitution)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	match := re.FindStringSubmatch(s)
	if match == nil {
		return "", nil
	}
	return match[0], nil
}

// ExtractVariablesFromString extracts variables from an input string s with the given prefix via regex matching.
// It returns a slice of strings which contains the extracted variables, a bool flag to indicate if matches were found
// and the error string if the referencing of parameters is invalid.
// If the string does not contain the input prefix then the output will contain a slice of strings with length 0.
func ExtractVariablesFromString(s, prefix string) ([]string, bool, string) {
	pattern := fmt.Sprintf(braceMatchingRegex, prefix, parameterSubstitution, parameterSubstitution, parameterSubstitution)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, false, ""
	}
	matches := re.FindAllStringSubmatch(s, -1)
	errString := ""
	// Input string does not contain the prefix and therefore not matches are found.
	if len(matches) == 0 {
		return []string{}, false, ""
	}
	vars := make([]string, len(matches))
	for i, match := range matches {
		groups := matchGroups(match, re)
		for j, v := range []string{"var1", "var2", "var3"} {
			val := groups[v]
			// If using the dot notation, the number of dot-separated components is restricted up to 2.
			// Valid Examples:
			//  - extract "aString" from <prefix>.aString
			//  - extract "anObject" from <prefix>.anObject.key
			// Invalid Examples:
			//  - <prefix>.foo.bar.baz....
			if j == 0 && strings.Contains(val, ".") {
				if len(strings.Split(val, ".")) > 2 {
					errString = fmt.Sprintf(`Invalid referencing of parameters in "%s"! Only two dot-separated components after the prefix "%s" are allowed.`, s, prefix)
					return vars, true, errString
				}
				vars[i] = strings.SplitN(val, ".", 2)[0]
				break
			}
			if val != "" {
				vars[i] = val
				break
			}
		}
	}
	return vars, true, errString
}

// extractEntireVariablesFromString returns any references to entire array or object params in s with the given prefix
func extractEntireVariablesFromString(s, prefix string) ([]string, error) {
	pattern := fmt.Sprintf(braceMatchingRegex, prefix, parameterSubstitution, parameterSubstitution, parameterSubstitution)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse regex pattern: %w", err)
	}

	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return []string{}, nil
	}
	vars := make([]string, len(matches))
	for i, match := range matches {
		groups := matchGroups(match, re)
		// foo -> foo
		// foo.bar -> foo.bar
		// foo.bar.baz -> foo.bar.baz
		for _, v := range []string{"var1", "var2", "var3"} {
			val := groups[v]
			if val != "" {
				vars[i] = val
				break
			}
		}
	}
	return vars, nil
}

func matchGroups(matches []string, pattern *regexp.Regexp) map[string]string {
	groups := make(map[string]string)
	for i, name := range pattern.SubexpNames()[1:] {
		groups[name] = matches[i+1]
	}
	return groups
}

// ApplyReplacements returns a string with references to parameters replaced,
// based on the mapping provided in replacements.
// For example, if the input string is "foo: $(params.foo)", and replacements maps "params.foo" to "bar",
// the output would be "foo: bar".
func ApplyReplacements(in string, replacements map[string]string) string {
	replacementsList := []string{}
	for k, v := range replacements {
		replacementsList = append(replacementsList, fmt.Sprintf("$(%s)", k), v)
	}
	// strings.Replacer does all replacements in one pass, preventing multiple replacements
	// See #2093 for an explanation on why we need to do this.
	replacer := strings.NewReplacer(replacementsList...)
	return replacer.Replace(in)
}

// ApplyArrayReplacements takes an input string, and output an array of strings related to possible arrayReplacements. If there aren't any
// areas where the input can be split up via arrayReplacements, then just return an array with a single element,
// which is ApplyReplacements(in, replacements).
func ApplyArrayReplacements(in string, stringReplacements map[string]string, arrayReplacements map[string][]string) []string {
	for k, v := range arrayReplacements {
		stringToReplace := fmt.Sprintf("$(%s)", k)

		// If the input string matches a replacement's key (without padding characters), return the corresponding array.
		// Note that the webhook should prevent all instances where this could evaluate to false.
		if (strings.Count(in, stringToReplace) == 1) && len(in) == len(stringToReplace) {
			return v
		}

		// same replace logic for star array expressions
		starStringtoReplace := fmt.Sprintf("$(%s[*])", k)
		if (strings.Count(in, starStringtoReplace) == 1) && len(in) == len(starStringtoReplace) {
			return v
		}
	}

	// Otherwise return a size-1 array containing the input string with standard stringReplacements applied.
	return []string{ApplyReplacements(in, stringReplacements)}
}

// TrimArrayIndex replaces all `[i]` and `[*]` to "".
func TrimArrayIndex(s string) string {
	return arrayIndexingRegex.ReplaceAllString(s, "")
}

// ExtractParamsExpressions will find all  `$(params.paramName[int])` expressions
func ExtractParamsExpressions(s string) []string {
	return paramIndexingRegex.FindAllString(s, -1)
}

// ExtractIndexString will find the leftmost match of `[int]`
func ExtractIndexString(s string) string {
	return intIndexRegex.FindString(s)
}

// ExtractIndex will extract int from `[int]`
func ExtractIndex(s string) (int, error) {
	return strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]"))
}

// StripStarVarSubExpression strips "$(target[*])"" to get "target"
func StripStarVarSubExpression(s string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(s, "$("), ")"), "[*]")
}
