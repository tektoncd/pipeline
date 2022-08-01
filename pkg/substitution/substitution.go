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

// a struct that holds a pair of values extracted from a reference: variable name and operator
// example:
// - $(params.myString)    - name: "myString", operator: ""
// - $(params.myArray[2])  - name: "myArray", operator: "1"
// - $(params.myObject[*]) - name: "myObject", operator: "*"
type variableReference struct {
	name     string
	operator string
}

const (
	// nameSubstition is the regex pattern for variable names that can catch any names in a notation
	nameSubstition = `.*?`
	// operator is the regex pattern for the operator suffix in a reference string
	// - [i] int indexing to reference an array element
	// - [*] star operator to reference the whole array or object
	operator = `([0-9]+|\*)`

	// paramIndex will match all `$(params.paramName[int])` expressions
	paramIndexing = `\$\(params(\.[_a-zA-Z0-9.-]+|\[\'[_a-zA-Z0-9.-\/]+\'\]|\[\"[_a-zA-Z0-9.-\/]+\"\])\[[0-9]+\]\)`
	// intIndex will match all `[int]` expressions
	intIndex = `\[[0-9]+\]`
)

var (
	// .paramName - dot notation
	dotNotation = fmt.Sprintf(`\.(?P<var1>%s)`, nameSubstition)
	// ['paramName'] - bracket notation with single quote
	bracketNotationWithSingleQuote = fmt.Sprintf(`\['(?P<var2>%s)'\]`, nameSubstition)
	// ["paramName"] - bracket notation with double quote
	bracketNotationWithDoubleQuote = fmt.Sprintf(`\[\"(?P<var3>%s)\"\]`, nameSubstition)
	// one of the three notations with optional indexing as the suffix
	variableNotationPattern = fmt.Sprintf(`(%s|%s|%s)(\[(?P<opt>%s)\])?`, dotNotation, bracketNotationWithSingleQuote, bracketNotationWithDoubleQuote, operator)

	// full variable referencing regex: $(xy)
	// - x is the prefix i.e. params
	// - y is `variableNotationPattern` - one of three notations + optional indexing
	fullVariableReferenceRegex = `\$\(%s%s\)`

	// isolatedParamVariablePattern is a regex to check if a string is an isolated
	// reference to a param variable.
	// (An isolated reference means a string that only contains a reference to a param
	// variable but nothing else i.e. no extra characters, no reference to other variables.)
	// Some examples:
	// 	- Isolated reference: "$(params.myParam)", "$(params['myArray'][*])"
	// 	- Non-isolated reference: "the param name is $(params.myParam)", "the first param is $(params.first) and the second param is $(params["second"])"
	isolatedParamVariablePattern = fmt.Sprintf(fmt.Sprintf("^%s$", fullVariableReferenceRegex), "params", variableNotationPattern)
	isolatedParamVariableRegex   = regexp.MustCompile(isolatedParamVariablePattern)

	// operatorRegex is used to match `[int]` and `[*]`
	operatorRegex = regexp.MustCompile(fmt.Sprintf(`\[%s\]`, operator))

	// paramIndexingRegex will match all `$(params.paramName[int])` expressions
	paramIndexingRegex = regexp.MustCompile(paramIndexing)

	// intIndexRegex will match all `[int]` for param expression
	intIndexRegex = regexp.MustCompile(intIndex)
)

// ValidateVariable makes sure all variables in the provided string are known
func ValidateVariable(name, value, prefix, locationName, path string, vars sets.String) *apis.FieldError {
	if refs, _ := extractVariablesFromString(value, prefix); len(refs) > 0 {
		for _, v := range refs {
			if !vars.Has(v.name) {
				return &apis.FieldError{
					Message: fmt.Sprintf("non-existent variable in %q for %s %s", value, locationName, name),
					Paths:   []string{path + "." + name},
				}
			}
		}
	}
	return nil
}

// ValidateVariableP makes sure all variables for a parameter in the provided string are known
func ValidateVariableP(value, prefix string, vars sets.String) *apis.FieldError {
	if refs, errString := extractVariablesFromString(value, prefix); len(refs) > 0 {
		if errString != "" {
			return &apis.FieldError{
				Message: errString,
				Paths:   []string{""},
			}

		}
		for _, v := range refs {
			if !vars.Has(v.name) {
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

// ValidateVariableProhibited verifies that variables matching the relevant string expressions do not reference any of the names present in vars.
func ValidateVariableProhibited(name, value, prefix, locationName, path string, vars sets.String) *apis.FieldError {
	if refs, _ := extractVariablesFromString(value, prefix); len(refs) > 0 {
		for _, v := range refs {
			if vars.Has(v.name) {
				return &apis.FieldError{
					Message: fmt.Sprintf("variable type invalid in %q for %s %s", value, locationName, name),
					Paths:   []string{path + "." + name},
				}
			}
		}
	}
	return nil
}

// ValidateVariableProhibitedP verifies that variables for a parameter matching the relevant string expressions do not reference any of the names present in vars.
func ValidateVariableProhibitedP(value, prefix string, vars sets.String) *apis.FieldError {
	if refs, errString := extractVariablesFromString(value, prefix); len(refs) > 0 {
		if errString != "" {
			return &apis.FieldError{
				Message: errString,
				Paths:   []string{""},
			}
		}
		for _, v := range refs {
			if vars.Has(v.name) {
				if _, isIntIndex := parseIndex(v.operator); isIntIndex {
					continue
				}
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

// ValidateEntireVariableProhibitedP verifies that values of object type are not used as whole.
func ValidateEntireVariableProhibitedP(value, prefix string, vars sets.String) *apis.FieldError {
	vs, err := extractEntireVariablesFromString(value, prefix)
	if err != nil {
		return &apis.FieldError{
			Message: fmt.Sprintf("extractEntireVariablesFromString failed : %v", err),
			// Empty path is required to make the `ViaField`, … work
			Paths: []string{""},
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

	return nil
}

// ValidateVariableIsolated verifies that variables matching the relevant string expressions are completely isolated if present.
func ValidateVariableIsolated(name, value, prefix, locationName, path string, vars sets.String) *apis.FieldError {
	if refs, _ := extractVariablesFromString(value, prefix); len(refs) > 0 {
		matches := extractExpressionsFromString(value, prefix)
		firstMatch := ""
		if len(matches) > 0 {
			firstMatch = matches[0]
		}
		for _, v := range refs {
			if vars.Has(v.name) {
				if len(value) != len(firstMatch) {
					return &apis.FieldError{
						Message: fmt.Sprintf("variable is not properly isolated in %q for %s %s", value, locationName, name),
						Paths:   []string{path + "." + name},
					}
				}
			}
		}
	}
	return nil
}

// ValidateVariableIsolatedP verifies that variables matching the relevant string expressions are completely isolated if present.
func ValidateVariableIsolatedP(value, prefix string, vars sets.String) *apis.FieldError {
	if refs, errString := extractVariablesFromString(value, prefix); len(refs) > 0 {
		if errString != "" {
			return &apis.FieldError{
				Message: errString,
				Paths:   []string{""},
			}

		}
		matches := extractExpressionsFromString(value, prefix)
		firstMatch := ""
		if len(matches) > 0 {
			firstMatch = matches[0]
		}
		for _, v := range refs {
			if vars.Has(v.name) {
				if len(value) != len(firstMatch) {
					return &apis.FieldError{
						Message: fmt.Sprintf("variable is not properly isolated in %q", value),
						// Empty path is required to make the `ViaField`, … work
						Paths: []string{""},
					}
				}
			}
		}
	}
	return nil
}

// ValidateIsolatedArrayOrObjectRefInStringVariable checks if a string is an isolated reference to an array/object param as whole
// If that's the case, the variable name in that isolated reference will be validated against a set of available array/object param names.
func ValidateIsolatedArrayOrObjectRefInStringVariable(name, value, prefix string, vars sets.String) (isIsolated bool, errs *apis.FieldError) {
	if isIsolated := IsIsolatedArrayOrObjectParamRef(value); isIsolated {
		return true, ValidateVariableP(value, prefix, vars).ViaFieldKey(prefix, name)
	}

	return false, nil
}

// IsIsolatedArrayOrObjectParamRef checks if a string is an isolated reference to the whole array/object param with star operator
// (An isolated reference to an array/object param as whole means a string that only contains a single reference
// to an array/object param variable but nothing else)
// Examples:
// 	- isolated reference: "$(params['myArray'][*])", "$(params.myObject[*])"
// 	- non-isolated reference: "this whole array param $(params["myArray"][*]) shouldn't appear in a string"
func IsIsolatedArrayOrObjectParamRef(value string) bool {
	if isolatedParamVariableRegex.MatchString(value) && strings.HasSuffix(value, "[*])") {
		return true
	}

	return false
}

// Extract a the first full string expressions found (e.g "$(input.params.foo)"). Return
// "" and false if nothing is found.
func extractExpressionsFromString(s, prefix string) []string {
	pattern := fmt.Sprintf(fullVariableReferenceRegex, prefix, variableNotationPattern)
	re := regexp.MustCompile(pattern)
	return re.FindAllString(s, -1)
}

// extractVariablesFromString extracts both variable name and corresponding operator from a string
// example:
// 	Input: "first param $(params.myString), second param $(params["myArray"][2]), third param $(params.myObject[*])"
// 	Output:
// 		[("myString", ""), ("myArray", "2"), ("myObject", "*")], ""
func extractVariablesFromString(s, prefix string) ([]variableReference, string) {
	pattern := fmt.Sprintf(fullVariableReferenceRegex, prefix, variableNotationPattern)
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(s, -1)
	errString := ""
	if len(matches) == 0 {
		return []variableReference{}, ""
	}
	result := make([]variableReference, len(matches))
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
					return result, errString
				}
				result[i] = variableReference{
					name:     strings.SplitN(val, ".", 2)[0],
					operator: groups["opt"],
				}
				break
			}
			if val != "" {
				result[i] = variableReference{
					name:     val,
					operator: groups["opt"],
				}
				break
			}
		}
	}
	return result, errString
}

func extractEntireVariablesFromString(s, prefix string) ([]string, error) {
	pattern := fmt.Sprintf(fullVariableReferenceRegex, prefix, variableNotationPattern)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("Fail to parse regex pattern: %v", err)
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

// ApplyReplacements applies string replacements
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

// TrimTailOperator replaces all `[i]` and `[*]` to "".
func TrimTailOperator(s string) string {
	return operatorRegex.ReplaceAllString(s, "")
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

// parseIndex parses integer from a string.
// If the string is not numeric, the second returned value will be false.
func parseIndex(s string) (int, bool) {
	val, err := strconv.Atoi(s)
	if err != nil {
		return val, false
	}
	return val, true
}
