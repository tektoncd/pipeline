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
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

const parameterSubstitution = `[_a-zA-Z][_a-zA-Z0-9.-]*(\[\*\])?`

const braceMatchingRegex = "(\\$(\\(%s\\.(?P<var>%s)\\)))"

// ValidateVariable makes sure all variables in the provided string are known
func ValidateVariable(name, value, prefix, locationName, path string, vars sets.String) *apis.FieldError {
	if vs, present := extractVariablesFromString(value, prefix); present {
		for _, v := range vs {
			v = strings.TrimSuffix(v, "[*]")
			if !vars.Has(v) {
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
	if vs, present := extractVariablesFromString(value, prefix); present {
		for _, v := range vs {
			v = strings.TrimSuffix(v, "[*]")
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

// ValidateVariableProhibited verifies that variables matching the relevant string expressions do not reference any of the names present in vars.
func ValidateVariableProhibited(name, value, prefix, locationName, path string, vars sets.String) *apis.FieldError {
	if vs, present := extractVariablesFromString(value, prefix); present {
		for _, v := range vs {
			v = strings.TrimSuffix(v, "[*]")
			if vars.Has(v) {
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
	if vs, present := extractVariablesFromString(value, prefix); present {
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

// ValidateVariableIsolated verifies that variables matching the relevant string expressions are completely isolated if present.
func ValidateVariableIsolated(name, value, prefix, locationName, path string, vars sets.String) *apis.FieldError {
	if vs, present := extractVariablesFromString(value, prefix); present {
		firstMatch, _ := extractExpressionFromString(value, prefix)
		for _, v := range vs {
			v = strings.TrimSuffix(v, "[*]")
			if vars.Has(v) {
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
	if vs, present := extractVariablesFromString(value, prefix); present {
		firstMatch, _ := extractExpressionFromString(value, prefix)
		for _, v := range vs {
			v = strings.TrimSuffix(v, "[*]")
			if vars.Has(v) {
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

// Extract a the first full string expressions found (e.g "$(input.params.foo)"). Return
// "" and false if nothing is found.
func extractExpressionFromString(s, prefix string) (string, bool) {
	pattern := fmt.Sprintf(braceMatchingRegex, prefix, parameterSubstitution)
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(s)
	if match == nil {
		return "", false
	}
	return match[0], true
}

func extractVariablesFromString(s, prefix string) ([]string, bool) {
	pattern := fmt.Sprintf(braceMatchingRegex, prefix, parameterSubstitution)
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return []string{}, false
	}
	vars := make([]string, len(matches))
	for i, match := range matches {
		groups := matchGroups(match, re)
		// foo -> foo
		// foo.bar -> foo
		// foo.bar.baz -> foo
		vars[i] = strings.SplitN(groups["var"], ".", 2)[0]
	}
	return vars, true
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
