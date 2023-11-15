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
	"regexp"
	"strings"

	"github.com/tektoncd/pipeline/pkg/internal/resultref"
)

// ResultRef is a type that represents a reference to a task run result
type ResultRef struct {
	PipelineTask string `json:"pipelineTask"`
	Result       string `json:"result"`
	ResultsIndex *int   `json:"resultsIndex"`
	Property     string `json:"property"`
}

const (
	// ResultTaskPart Constant used to define the "tasks" part of a pipeline result reference
	// retained because of backwards compatibility
	ResultTaskPart = resultref.ResultTaskPart
	// ResultFinallyPart Constant used to define the "finally" part of a pipeline result reference
	// retained because of backwards compatibility
	ResultFinallyPart = resultref.ResultFinallyPart
	// ResultResultPart Constant used to define the "results" part of a pipeline result reference
	// retained because of backwards compatibility
	ResultResultPart = resultref.ResultResultPart
	// TODO(#2462) use one regex across all substitutions
	// variableSubstitutionFormat matches format like $result.resultname, $result.resultname[int] and $result.resultname[*]
	variableSubstitutionFormat = `\$\([_a-zA-Z0-9.-]+(\.[_a-zA-Z0-9.-]+)*(\[([0-9]+|\*)\])?\)`
	// exactVariableSubstitutionFormat matches strings that only contain a single reference to result or param variables, but nothing else
	// i.e. `$(result.resultname)` is a match, but `foo $(result.resultname)` is not.
	exactVariableSubstitutionFormat = `^\$\([_a-zA-Z0-9.-]+(\.[_a-zA-Z0-9.-]+)*(\[([0-9]+|\*)\])?\)$`
	// ResultNameFormat Constant used to define the regex Result.Name should follow
	ResultNameFormat = `^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`
)

// VariableSubstitutionRegex is a regex to find all result matching substitutions
var VariableSubstitutionRegex = regexp.MustCompile(variableSubstitutionFormat)
var exactVariableSubstitutionRegex = regexp.MustCompile(exactVariableSubstitutionFormat)
var resultNameFormatRegex = regexp.MustCompile(ResultNameFormat)

// NewResultRefs extracts all ResultReferences from a param or a pipeline result.
// If the ResultReference can be extracted, they are returned. Expressions which are not
// results are ignored.
func NewResultRefs(expressions []string) []*ResultRef {
	var resultRefs []*ResultRef
	for _, expression := range expressions {
		pr, err := resultref.ParseTaskExpression(expression)
		// If the expression isn't a result but is some other expression,
		// parseExpression will return an error, in which case we just skip that expression,
		// since although it's not a result ref, it might be some other kind of reference
		if err == nil {
			resultRefs = append(resultRefs, &ResultRef{
				PipelineTask: pr.ResourceName,
				Result:       pr.ResultName,
				ResultsIndex: pr.ArrayIdx,
				Property:     pr.ObjectKey,
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
		if resultref.LooksLikeResultRef(expression) {
			return true
		}
	}
	return false
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

// ParseResultName parse the input string to extract resultName and result index.
// Array indexing:
// Input:  anArrayResult[1]
// Output: anArrayResult, "1"
// Array star reference:
// Input:  anArrayResult[*]
// Output: anArrayResult, "*"
func ParseResultName(resultName string) (string, string) {
	return resultref.ParseResultName(resultName)
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
