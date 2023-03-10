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
	"strconv"
	"strings"
)

// ResultRef is a type that represents a reference to a task run result
type ResultRef struct {
	PipelineTask string `json:"pipelineTask"`
	Result       string `json:"result"`
	ResultsIndex int    `json:"resultsIndex"`
	Property     string `json:"property"`
}

const (
	resultExpressionFormat = "tasks.<taskName>.results.<resultName>"
	// Result expressions of the form <resultName>.<attribute> will be treated as object results.
	// If a string result name contains a dot, brackets should be used to differentiate it from an object result.
	// https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md#collisions-with-builtin-variable-replacement
	objectResultExpressionFormat = "tasks.<taskName>.results.<objectResultName>.<individualAttribute>"
	// ResultTaskPart Constant used to define the "tasks" part of a pipeline result reference
	ResultTaskPart = "tasks"
	// ResultFinallyPart Constant used to define the "finally" part of a pipeline result reference
	ResultFinallyPart = "finally"
	// ResultResultPart Constant used to define the "results" part of a pipeline result reference
	ResultResultPart = "results"
	// TODO(#2462) use one regex across all substitutions
	// variableSubstitutionFormat matches format like $result.resultname, $result.resultname[int] and $result.resultname[*]
	variableSubstitutionFormat = `\$\([_a-zA-Z0-9.-]+(\.[_a-zA-Z0-9.-]+)*(\[([0-9]+|\*)\])?\)`
	// exactVariableSubstitutionFormat matches strings that only contain a single reference to result or param variables, but nothing else
	// i.e. `$(result.resultname)` is a match, but `foo $(result.resultname)` is not.
	exactVariableSubstitutionFormat = `^\$\([_a-zA-Z0-9.-]+(\.[_a-zA-Z0-9.-]+)*(\[([0-9]+|\*)\])?\)$`
	// arrayIndexing will match all `[int]` and `[*]` for parseExpression
	arrayIndexing = `\[([0-9])*\*?\]`
	// ResultNameFormat Constant used to define the regex Result.Name should follow
	ResultNameFormat = `^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`
)

// VariableSubstitutionRegex is a regex to find all result matching substitutions
var VariableSubstitutionRegex = regexp.MustCompile(variableSubstitutionFormat)
var exactVariableSubstitutionRegex = regexp.MustCompile(exactVariableSubstitutionFormat)
var resultNameFormatRegex = regexp.MustCompile(ResultNameFormat)

// arrayIndexingRegex is used to match `[int]` and `[*]`
var arrayIndexingRegex = regexp.MustCompile(arrayIndexing)

// NewResultRefs extracts all ResultReferences from a param or a pipeline result.
// If the ResultReference can be extracted, they are returned. Expressions which are not
// results are ignored.
func NewResultRefs(expressions []string) []*ResultRef {
	var resultRefs []*ResultRef
	for _, expression := range expressions {
		pipelineTask, result, index, property, err := parseExpression(expression)
		// If the expression isn't a result but is some other expression,
		// parseExpression will return an error, in which case we just skip that expression,
		// since although it's not a result ref, it might be some other kind of reference
		if err == nil {
			resultRefs = append(resultRefs, &ResultRef{
				PipelineTask: pipelineTask,
				Result:       result,
				ResultsIndex: index,
				Property:     property,
			})
		}
	}
	return resultRefs
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

// looksLikeResultRef attempts to check if the given string looks like it contains any
// result references. Returns true if it does, false otherwise
func looksLikeResultRef(expression string) bool {
	subExpressions := strings.Split(expression, ".")
	return len(subExpressions) >= 4 && (subExpressions[0] == ResultTaskPart || subExpressions[0] == ResultFinallyPart) && subExpressions[2] == ResultResultPart
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
	case ParamTypeObject:
		// object type
		for _, value := range param.Value.ObjectVal {
			allExpressions = append(allExpressions, validateString(value)...)
		}
	default:
		return nil, false
	}
	return allExpressions, len(allExpressions) != 0
}

// GetVarSubstitutionExpressionsForPipelineResult extracts all the value between "$(" and ")"" for a pipeline result
func GetVarSubstitutionExpressionsForPipelineResult(result PipelineResult) ([]string, bool) {
	allExpressions := validateString(result.Value.StringVal)
	for _, v := range result.Value.ArrayVal {
		allExpressions = append(allExpressions, validateString(v)...)
	}
	for _, v := range result.Value.ObjectVal {
		allExpressions = append(allExpressions, validateString(v)...)
	}
	return allExpressions, len(allExpressions) != 0
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

func stripVarSubExpression(expression string) string {
	return strings.TrimSuffix(strings.TrimPrefix(expression, "$("), ")")
}

// parseExpression parses "task name", "result name", "array index" (iff it's an array result) and "object key name" (iff it's an object result)
// Valid Example 1:
// - Input: tasks.myTask.results.aStringResult
// - Output: "myTask", "aStringResult", -1, "", nil
// Valid Example 2:
// - Input: tasks.myTask.results.anObjectResult.key1
// - Output: "myTask", "anObjectResult", 0, "key1", nil
// Valid Example 3:
// - Input: tasks.myTask.results.anArrayResult[1]
// - Output: "myTask", "anArrayResult", 1, "", nil
// Invalid Example 1:
// - Input: tasks.myTask.results.resultName.foo.bar
// - Output: "", "", 0, "", error
// TODO: may use regex for each type to handle possible reference formats
func parseExpression(substitutionExpression string) (string, string, int, string, error) {
	if looksLikeResultRef(substitutionExpression) {
		subExpressions := strings.Split(substitutionExpression, ".")
		// For string result: tasks.<taskName>.results.<stringResultName>
		// For array result: tasks.<taskName>.results.<arrayResultName>[index]
		if len(subExpressions) == 4 {
			resultName, stringIdx := ParseResultName(subExpressions[3])
			if stringIdx != "" {
				intIdx, _ := strconv.Atoi(stringIdx)
				return subExpressions[1], resultName, intIdx, "", nil
			}
			return subExpressions[1], resultName, 0, "", nil
		} else if len(subExpressions) == 5 {
			// For object type result: tasks.<taskName>.results.<objectResultName>.<individualAttribute>
			return subExpressions[1], subExpressions[3], 0, subExpressions[4], nil
		}
	}

	return "", "", 0, "", fmt.Errorf("must be one of the form 1). %q; 2). %q", resultExpressionFormat, objectResultExpressionFormat)
}

// ParseResultName parse the input string to extract resultName and result index.
// Array indexing:
// Input:  anArrayResult[1]
// Output: anArrayResult, "1"
// Array star reference:
// Input:  anArrayResult[*]
// Output: anArrayResult, "*"
func ParseResultName(resultName string) (string, string) {
	stringIdx := strings.TrimSuffix(strings.TrimPrefix(arrayIndexingRegex.FindString(resultName), "["), "]")
	resultName = arrayIndexingRegex.ReplaceAllString(resultName, "")
	return resultName, stringIdx
}

// PipelineTaskResultRefs walks all the places a result reference can be used
// in a PipelineTask and returns a list of any references that are found.
func PipelineTaskResultRefs(pt *PipelineTask) []*ResultRef {
	refs := []*ResultRef{}
	for _, p := range pt.extractAllParams() {
		expressions, _ := GetVarSubstitutionExpressionsForParam(p)
		refs = append(refs, NewResultRefs(expressions)...)
	}
	for _, whenExpression := range pt.WhenExpressions {
		expressions, _ := whenExpression.GetVarSubstitutionExpressions()
		refs = append(refs, NewResultRefs(expressions)...)
	}
	return refs
}
