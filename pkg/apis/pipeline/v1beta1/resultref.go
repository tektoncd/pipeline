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

package v1beta1

import (
	"fmt"
	"regexp"
	"strings"
)

// ResultRef is a type that represents a reference to a task run result
type ResultRef struct {
	PipelineTask string
	Result       string
}

const (
	resultExpressionFormat = "tasks.<taskName>.results.<resultName>"
	// ResultTaskPart Constant used to define the "tasks" part of a pipeline result reference
	ResultTaskPart = "tasks"
	// ResultResultPart Constant used to define the "results" part of a pipeline result reference
	ResultResultPart           = "results"
	variableSubstitutionFormat = `\$\([A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\)`
)

var variableSubstitutionRegex = regexp.MustCompile(variableSubstitutionFormat)

// NewResultRefs extracts all ResultReferences from a param or a pipeline result.
// If the ResultReference can be extracted, they are returned. Otherwise an error is returned
func NewResultRefs(expressions []string) ([]*ResultRef, error) {
	var resultRefs []*ResultRef
	for _, expression := range expressions {
		pipelineTask, result, err := parseExpression(expression)
		if err != nil {
			return nil, fmt.Errorf("Invalid result reference expression: %v", err)
		}
		resultRefs = append(resultRefs, &ResultRef{
			PipelineTask: pipelineTask,
			Result:       result,
		})
	}
	return resultRefs, nil
}

// LooksLikeContainsResultRefs attempts to check if param or a pipeline result looks like it contains any
// result references.
// This is useful if we want to make sure the param looks like a ResultReference before
// performing strict validation
func LooksLikeContainsResultRefs(expressions []string) bool {
	for _, expression := range expressions {
		if looksLikeResultRef(expression) {
			return true
		}
	}
	return false
}

func looksLikeResultRef(expression string) bool {
	return strings.HasPrefix(expression, "task") && strings.Contains(expression, ".result")
}

// GetVarSubstitutionExpressionsForParam extracts all the value between "$(" and ")"" for a parameter
func GetVarSubstitutionExpressionsForParam(param Param) ([]string, bool) {
	var allExpressions []string
	switch param.Value.Type {
	case ParamTypeArray:
		// array type
		for _, value := range param.Value.ArrayVal {
			allExpressions = append(allExpressions, validateString(value)...)
		}
	case ParamTypeString:
		// string type
		allExpressions = append(allExpressions, validateString(param.Value.StringVal)...)
	default:
		return nil, false
	}
	return allExpressions, len(allExpressions) != 0
}

// GetVarSubstitutionExpressionsForPipelineResult extracts all the value between "$(" and ")"" for a pipeline result
func GetVarSubstitutionExpressionsForPipelineResult(result PipelineResult) ([]string, bool) {
	allExpressions := validateString(result.Value)
	return allExpressions, len(allExpressions) != 0
}

func validateString(value string) []string {
	expressions := variableSubstitutionRegex.FindAllString(value, -1)
	if expressions == nil {
		return nil
	}
	var result []string
	for _, expression := range expressions {
		result = append(result, stripVarSubExpression(expression))
	}
	return result
}

func stripVarSubExpression(expression string) string {
	return strings.TrimSuffix(strings.TrimPrefix(expression, "$("), ")")
}

func parseExpression(substitutionExpression string) (string, string, error) {
	subExpressions := strings.Split(substitutionExpression, ".")
	if len(subExpressions) != 4 || subExpressions[0] != ResultTaskPart || subExpressions[2] != ResultResultPart {
		return "", "", fmt.Errorf("Must be of the form %q", resultExpressionFormat)
	}
	return subExpressions[1], subExpressions[3], nil
}
