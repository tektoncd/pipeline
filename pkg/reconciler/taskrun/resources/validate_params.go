package resources

import (
	"fmt"

	pipelineErrors "github.com/tektoncd/pipeline/pkg/apis/pipeline/errors"
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
func ValidateOutOfBoundArrayParams(declarations v1.ParamSpecs, params v1.Params, arrayIndexingReferences sets.Set[string]) error {
	arrayParamLengths := declarations.ExtractDefaultParamArrayLengths()
	for k, v := range params.ExtractParamArrayLengths() {
		arrayParamLengths[k] = v
	}
	outofBoundParams := sets.Set[string]{}
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
		return pipelineErrors.WrapUserError(fmt.Errorf("non-existent param references:%v", sets.List(outofBoundParams)))
	}
	return nil
}

func validateStepHasStepActionParameters(stepParams v1.Params, stepActionDefaults []v1.ParamSpec) error {
	stepActionParams := sets.Set[string]{}
	requiredStepActionParams := []string{}
	for _, sa := range stepActionDefaults {
		stepActionParams.Insert(sa.Name)
		if sa.Default == nil {
			requiredStepActionParams = append(requiredStepActionParams, sa.Name)
		}
	}

	stepProvidedParams := sets.Set[string]{}
	extra := []string{}
	for _, sp := range stepParams {
		stepProvidedParams.Insert(sp.Name)
		if !stepActionParams.Has(sp.Name) {
			// Extra parameter that is not needed
			extra = append(extra, sp.Name)
		}
	}
	if len(extra) > 0 {
		return fmt.Errorf("extra params passed by Step to StepAction: %v", extra)
	}

	missing := []string{}

	for _, requiredParam := range requiredStepActionParams {
		if !stepProvidedParams.Has(requiredParam) {
			// Missing required param
			missing = append(missing, requiredParam)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("non-existent params in Step: %v", missing)
	}
	return nil
}
