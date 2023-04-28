/*
Copyright 2023 The Tekton Authors
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

package v1

import (
	"context"
	"fmt"
	"sort"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/strings/slices"
	"knative.dev/pkg/apis"
)

// Matrix is used to fan out Tasks in a Pipeline
type Matrix struct {
	// Params is a list of parameters used to fan out the pipelineTask
	// Params takes only `Parameters` of type `"array"`
	// Each array element is supplied to the `PipelineTask` by substituting `params` of type `"string"` in the underlying `Task`.
	// The names of the `params` in the `Matrix` must match the names of the `params` in the underlying `Task` that they will be substituting.
	// +listType=atomic
	Params Params `json:"params,omitempty"`

	// Include is a list of IncludeParams which allows passing in specific combinations of Parameters into the Matrix.
	// +optional
	// +listType=atomic
	Include IncludeParamsList `json:"include,omitempty"`
}

// IncludeParamsList is a list of IncludeParams which allows passing in specific combinations of Parameters into the Matrix.
type IncludeParamsList []IncludeParams

// IncludeParams allows passing in a specific combinations of Parameters into the Matrix.
type IncludeParams struct {
	// Name the specified combination
	Name string `json:"name,omitempty"`

	// Params takes only `Parameters` of type `"string"`
	// The names of the `params` must match the names of the `params` in the underlying `Task`
	// +listType=atomic
	Params Params `json:"params,omitempty"`
}

// Combination is a map, mainly defined to hold a single combination from a Matrix with key as param.Name and value as param.Value
type Combination map[string]string

// Combinations is a Combination list
type Combinations []Combination

// FanOut returns an list of params that represent combinations
func (m *Matrix) FanOut() []Params {
	var combinations, includeCombinations Combinations
	includeCombinations = m.getIncludeCombinations()
	if m.HasInclude() && !m.HasParams() {
		// If there are only Matrix Include Parameters return explicit combinations
		return includeCombinations.toParams()
	}
	// Generate combinations from Matrix Parameters
	for _, parameter := range m.Params {
		combinations = combinations.fanOutMatrixParams(parameter)
	}
	combinations.overwriteCombinations(includeCombinations)
	combinations = combinations.addNewCombinations(includeCombinations)
	return combinations.toParams()
}

// overwriteCombinations replaces any missing include params in the initial
// matrix params combinations by overwriting the initial combinations with the
// include combinations
func (cs Combinations) overwriteCombinations(ics Combinations) {
	for _, paramCombination := range cs {
		for _, includeCombination := range ics {
			if paramCombination.contains(includeCombination) {
				// overwrite the parameter name and value in existing combination
				// with the include combination
				for name, val := range includeCombination {
					paramCombination[name] = val
				}
			}
		}
	}
}

// addNewCombinations creates a new combination for any include parameter
// values that are missing entirely from the initial combinations and
// returns all combinations
func (cs Combinations) addNewCombinations(ics Combinations) Combinations {
	for _, includeCombination := range ics {
		if cs.shouldAddNewCombination(includeCombination) {
			cs = append(cs, includeCombination)
		}
	}
	return cs
}

// contains returns true if the include parameter name and value exists in combinations
func (c Combination) contains(includeCombination Combination) bool {
	for name, val := range includeCombination {
		if _, exist := c[name]; exist {
			if c[name] != val {
				return false
			}
		}
	}
	return true
}

// shouldAddNewCombination returns true if the include parameter name exists but the value is
// missing from combinations
func (cs Combinations) shouldAddNewCombination(includeCombination map[string]string) bool {
	if len(includeCombination) == 0 {
		return false
	}
	for _, paramCombination := range cs {
		for name, val := range includeCombination {
			if _, exist := paramCombination[name]; exist {
				if paramCombination[name] == val {
					return false
				}
			}
		}
	}
	return true
}

// toParams transforms Combinations from a slice of map[string]string to a slice of Params
// such that, these combinations can be directly consumed in creating taskRun/run object
func (cs Combinations) toParams() []Params {
	listOfParams := make([]Params, len(cs))
	for i := range cs {
		var params Params
		combination := cs[i]
		order, _ := combination.sortCombination()
		for _, key := range order {
			params = append(params, Param{
				Name:  key,
				Value: ParamValue{Type: ParamTypeString, StringVal: combination[key]},
			})
		}
		listOfParams[i] = params
	}
	return listOfParams
}

// fanOutMatrixParams generates new combinations based on Matrix Parameters.
func (cs Combinations) fanOutMatrixParams(param Param) Combinations {
	if len(cs) == 0 {
		return initializeCombinations(param)
	}
	return cs.distribute(param)
}

// getIncludeCombinations generates combinations based on Matrix Include Parameters
func (m *Matrix) getIncludeCombinations() Combinations {
	var combinations Combinations
	for i := range m.Include {
		includeParams := m.Include[i].Params
		newCombination := make(Combination)
		for _, param := range includeParams {
			newCombination[param.Name] = param.Value.StringVal
		}
		combinations = append(combinations, newCombination)
	}
	return combinations
}

// distribute generates a new Combination of Parameters by adding a new Parameter to an existing list of Combinations.
func (cs Combinations) distribute(param Param) Combinations {
	var expandedCombinations Combinations
	for _, value := range param.Value.ArrayVal {
		for _, combination := range cs {
			newCombination := make(Combination)
			maps.Copy(newCombination, combination)
			newCombination[param.Name] = value
			_, orderedCombination := newCombination.sortCombination()
			expandedCombinations = append(expandedCombinations, orderedCombination)
		}
	}
	return expandedCombinations
}

// initializeCombinations generates a new Combination based on the first Parameter in the Matrix.
func initializeCombinations(param Param) Combinations {
	var combinations Combinations
	for _, value := range param.Value.ArrayVal {
		combinations = append(combinations, Combination{param.Name: value})
	}
	return combinations
}

// sortCombination sorts the given Combination based on the Parameter names to produce a deterministic ordering
func (c Combination) sortCombination() ([]string, Combination) {
	sortedCombination := make(Combination, len(c))
	order := make([]string, 0, len(c))
	for key := range c {
		order = append(order, key)
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i] <= order[j]
	})
	for _, key := range order {
		sortedCombination[key] = c[key]
	}
	return order, sortedCombination
}

// CountCombinations returns the count of Combinations of Parameters generated from the Matrix in PipelineTask.
func (m *Matrix) CountCombinations() int {
	// Iterate over Matrix Parameters and compute count of all generated Combinations
	count := m.countGeneratedCombinationsFromParams()

	// Add any additional Combinations generated from Matrix Include Parameters
	count += m.countNewCombinationsFromInclude()

	return count
}

// countGeneratedCombinationsFromParams returns the count of Combinations of Parameters generated from the Matrix
// Parameters
func (m *Matrix) countGeneratedCombinationsFromParams() int {
	if !m.HasParams() {
		return 0
	}
	count := 1
	for _, param := range m.Params {
		count *= len(param.Value.ArrayVal)
	}
	return count
}

// countNewCombinationsFromInclude returns the count of Combinations of Parameters generated from the Matrix
// Include Parameters
func (m *Matrix) countNewCombinationsFromInclude() int {
	if !m.HasInclude() {
		return 0
	}
	if !m.HasParams() {
		return len(m.Include)
	}
	count := 0
	matrixParamMap := m.Params.extractParamMapArrVals()
	for _, include := range m.Include {
		for _, param := range include.Params {
			if val, exist := matrixParamMap[param.Name]; exist {
				// If the Matrix Include param values does not exist, a new Combination will be generated
				if !slices.Contains(val, param.Value.StringVal) {
					count++
				} else {
					break
				}
			}
		}
	}
	return count
}

// HasInclude returns true if the Matrix has Include Parameters
func (m *Matrix) HasInclude() bool {
	return m != nil && m.Include != nil && len(m.Include) > 0
}

// HasParams returns true if the Matrix has Parameters
func (m *Matrix) HasParams() bool {
	return m != nil && m.Params != nil && len(m.Params) > 0
}

// GetAllParams returns a list of all Matrix Parameters
func (m *Matrix) GetAllParams() Params {
	var params Params
	if m.HasParams() {
		params = append(params, m.Params...)
	}
	if m.HasInclude() {
		for _, include := range m.Include {
			params = append(params, include.Params...)
		}
	}
	return params
}

func (m *Matrix) validateCombinationsCount(ctx context.Context) (errs *apis.FieldError) {
	matrixCombinationsCount := m.CountCombinations()
	maxMatrixCombinationsCount := config.FromContextOrDefaults(ctx).Defaults.DefaultMaxMatrixCombinationsCount
	if matrixCombinationsCount > maxMatrixCombinationsCount {
		errs = errs.Also(apis.ErrOutOfBoundsValue(matrixCombinationsCount, 0, maxMatrixCombinationsCount, "matrix"))
	}
	return errs
}

// validateUniqueParams validates Matrix.Params for a unique list of params
// and a unique list of params in each Matrix.Include.Params specification
func (m *Matrix) validateUniqueParams() (errs *apis.FieldError) {
	if m != nil {
		if m.HasInclude() {
			for i, include := range m.Include {
				errs = errs.Also(include.Params.validateDuplicateParameters().ViaField(fmt.Sprintf("matrix.include[%d].params", i)))
			}
		}
		if m.HasParams() {
			errs = errs.Also(m.Params.validateDuplicateParameters().ViaField("matrix.params"))
		}
	}
	return errs
}

// validatePipelineParametersVariablesInMatrixParameters validates all pipeline parameter variables including Matrix.Params and Matrix.Include.Params
// that may contain the reference(s) to other params to make sure those references are used appropriately.
func (m *Matrix) validatePipelineParametersVariablesInMatrixParameters(prefix string, paramNames sets.String, arrayParamNames sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	if m.HasInclude() {
		for _, include := range m.Include {
			for idx, param := range include.Params {
				stringElement := param.Value.StringVal
				// Matrix Include Params must be of type string
				errs = errs.Also(validateStringVariable(stringElement, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaFieldIndex("", idx).ViaField("matrix.include.params", ""))
			}
		}
	}
	if m.HasParams() {
		for _, param := range m.Params {
			for idx, arrayElement := range param.Value.ArrayVal {
				// Matrix Params must be of type array
				errs = errs.Also(validateArrayVariable(arrayElement, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaFieldIndex("value", idx).ViaFieldKey("matrix.params", param.Name))
			}
		}
	}
	return errs
}

func (m *Matrix) validateParameterInOneOfMatrixOrParams(params []Param) (errs *apis.FieldError) {
	matrixParamNames := m.GetAllParams().ExtractNames()
	for _, param := range params {
		if matrixParamNames.Has(param.Name) {
			errs = errs.Also(apis.ErrMultipleOneOf("matrix["+param.Name+"]", "params["+param.Name+"]"))
		}
	}
	return errs
}
