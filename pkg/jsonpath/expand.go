/*
Copyright 2020 The Tekton Authors
	"github.com/tektoncd/pipeline/pkg/substitution"

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

package jsonpath

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

var (
	// expandRE captures the strings $$ (e.g. escaped dollar-sign) and those enclosed in $() (e.g. Tekton expression)
	// This regex currently will only accept one-level of filter-expression e.g. nested (...) expressions but not (...(...))
	expandRE = regexp.MustCompile(`\$\$|\$\((?:[^()]|\([^()]*\))+\)`)
	// the list of valid Tekton JSONPath prefixes from the Kube library. Any other prefixes are treated as literals
	expressionPrefix = regexp.MustCompile(`^[$.[@'"]`)
)

// createExpression takes a Tekton expression of the form $(...) and turns it into a hopefully valid Kubernets JSONPath expression
func createExpression(variable string) string {
	expression := strings.TrimSuffix(strings.TrimPrefix(variable, "$("), ")")
	// we will dollar dot-prefix all expressions that don't already have a valid prefix so they are resolved relative to the root
	if len(expression) != 0 && !expressionPrefix.MatchString(expression) {
		expression = "$." + expression
	}
	return "{" + expression + "}"
}

// expandVariable is the function that directly interacts with the Kubernetes JSONPath support
// Note: we currently do not support {range}
func expandVariable(variable string, context interface{}) ([]interface{}, error) {
	j := jsonpath.New("").AllowMissingKeys(false)

	expr := createExpression(variable)
	if err := j.Parse(expr); err != nil {
		return nil, err
	}

	jResults, err := j.FindResults(context)
	if err != nil {
		return nil, err
	}

	// we do not support multiple JSONPath "root" results currently (e.g. {range})
	if len(jResults) != 1 {
		return nil, fmt.Errorf("multiple results: %v", len(jResults))
	}

	// we do support JSONPath "lists" though...
	result := make([]interface{}, len(jResults[0]))
	for i := range result {
		result[i] = jResults[0][i].Interface()
	}
	return result, nil
}

// expandStringAsList expands a string that contains JSONPath expressions
// this variation is used directly when expanding into an array as all items in the JSONPath result list are appended
// if the input string consists of a single JSONPath expression the expansion returns a JSONPath result list
// if the input string is made up of a mixture of string literals or multiple expressions the expansion return the string
// concatenation of the constituent parts stringified using json marshalling for all non-string parts
func expandStringAsList(input string, context interface{}) ([]interface{}, error) {
	match := expandRE.FindString(input)
	if match == "" {
		return []interface{}{input}, nil
	}
	// if the input consists of a single JSONPath expression, return the JSONPath result list
	if input == match && match != "$$" {
		expanded, err := expandVariable(match, context)
		// if there is a problem we return the original string (consistent with Kubernetes container env expansion)
		if err != nil {
			return []interface{}{}, err
		}
		return expanded, nil
	}
	var expandError error = nil
	expandedTemplate := expandRE.ReplaceAllStringFunc(input, func(match string) string {
		// escape double-dollars as a single dollar
		if match == "$$" {
			return "$"
		}
		expanded, err := expandVariable(match, context)
		if err != nil {
			if expandError == nil {
				expandError = err
			}
			return match
		}

		// if the array or object star expansion is empty, the replacement is the empty string
		if len(expanded) == 0 {
			return ""
		}

		// unless expanded in the context of an array we only want the first list item
		// at some point we might allow {range} to process JSONPath lists
		result := expanded[0]

		// strings are replaced as is to eliminate the extra quotes from json marshalling
		s, isString := result.(string)
		if isString {
			return s
		}

		// all other types are json marshalled
		b, err := json.Marshal(result)
		if err != nil {
			if expandError == nil {
				expandError = err
			}
			return match
		}
		return string(b)
	})
	if expandError != nil {
		return nil, expandError
	}
	return []interface{}{expandedTemplate}, nil
}

// expandString expands a string that contains JSONPath expressions
// this is the non-array context variation and only the first value from expandStringAsList is returned
func expandString(input string, context interface{}) (interface{}, error) {
	expanded, err := expandStringAsList(input, context)
	if err != nil {
		return nil, err
	}
	// if the array or object star expansion is empty, return an empty string
	if len(expanded) == 0 {
		return "", nil
	}
	return expanded[0], nil
}

// expandArray walks each array element and expands it if possible
// if the expansion returns a JSONPath result list inserts all list members!
func expandArray(input []interface{}, context interface{}) ([]interface{}, error) {
	var result = make([]interface{}, 0, len(input))
	for _, v := range input {
		switch t := v.(type) {
		case string:
			// all items in the JSONPath list are appended when called in the context of an array
			expanded, err := expandStringAsList(t, context)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded...)
		case []interface{}:
			expanded, err := expandArray(t, context)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded)
		case map[string]interface{}:
			expanded, err := expandObject(t, context)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded)
		default:
			result = append(result, t)
		}
	}
	return result, nil
}

// expandObject walks each map entry and expands its value if possible
func expandObject(input map[string]interface{}, context interface{}) (map[string]interface{}, error) {
	var result = make(map[string]interface{}, len(input))
	var err error
	for k, v := range input {
		switch t := v.(type) {
		case string:
			result[k], err = expandString(t, context)
		case []interface{}:
			result[k], err = expandArray(t, context)
		case map[string]interface{}:
			result[k], err = expandObject(t, context)
		default:
			result[k] = v
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Expand transforms json by walking the incoming input looking for Tekton expressions and translating into json output
// using a supplied context. Tekton expressions are represented using $(...) where the body is a Kubernetes JSONPath expression.
// see https://goessner.net/articles/JsonPath/index.html and https://kubernetes.io/docs/reference/kubectl/jsonpath/
// Note: "range" and "end" operators are not supported and curly brackets have no special semantics are treated as string literals
//
// Similar to CSS and XPath, JSONPath has the flexibility to return multiple results of any JSON type and expansion is handled
// differently depending on if the expression container is a scalar (e.g. object field or top-level), or vector (e.g. an array).
// 1) If expanded into a field-type the "first element" of the JSONPath result "replaces" the expression value
// 2) If expanded into an array-type "all elements" are appended at the expression's index
// Note: Tekton uses Kubernetes expansion semantics, so the expression string is returned if there is no matching results
//
// In addition to simple JSONPath expression expansion Tekton supports string templating. Tekton expressions expanded as part of a
// a larger string or a string containing multiple expressions are output as text. "string" values are output as is without
// surrounding quotes all other types are JSON Marshalled using goland encoding/json rules. To force a JSONPath result to use
// string templating add an empty string literal expression to your original expression eg. $('')$(original.expression.to.stringify)
func Expand(input interface{}, context interface{}) (interface{}, error) {
	switch t := input.(type) {
	case string:
		return expandString(t, context)
	case []interface{}:
		return expandArray(t, context)
	case map[string]interface{}:
		return expandObject(t, context)
	default:
		return input, nil
	}
}
