package resources

import (
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidateParamArrayIndex validates if the param reference to an array param is out of bound.
// error is returned when the array indexing reference is out of bound of the array param
// e.g. if a param reference of $(params.array-param[2]) and the array param is of length 2.
// - `params` are params from taskrun.
// - `ts` contains params declarations and references to array params.
func ValidateParamArrayIndex(ts *v1.TaskSpec, params v1.Params) error {
	return ValidateOutOfBoundArrayParams(ts.Params, params, ts.GetIndexingReferencesToArrayParams())
}

// ValidateOutOfBoundArrayParams returns an error if the array indexing params are out of bounds,
// based on the param declarations, the parameters passed in at runtime, and the indexing references
// to array params from a task or pipeline spec.
// Example of arrayIndexingReferences: ["$(params.a-array-param[1])", "$(params.b-array-param[2])"]
func ValidateOutOfBoundArrayParams(declarations v1.ParamSpecs, params v1.Params, arrayIndexingReferences sets.String) error {
	arrayParamLengths := declarations.ExtractDefaultParamArrayLengths()
	for k, v := range params.ExtractParamArrayLengths() {
		arrayParamLengths[k] = v
	}
	outofBoundParams := sets.String{}
	for val := range arrayIndexingReferences {
		indexString := substitution.ExtractIndexString(val)
		idx, _ := substitution.ExtractIndex(indexString)
		// this will extract the param name from reference
		// e.g. $(params.a-array-param[1]) -> a-array-param
		paramName, _, _ := substitution.ExtractVariablesFromString(substitution.TrimArrayIndex(val), "params")

		if paramLength, ok := arrayParamLengths[paramName[0]]; ok {
			if idx >= paramLength {
				outofBoundParams.Insert(val)
			}
		}
	}
	if outofBoundParams.Len() > 0 {
		return fmt.Errorf("non-existent param references:%v", outofBoundParams.List())
	}
	return nil
}
