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

package v1beta1

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

	// Include is a list of MatrixInclude which allows passing in specific combinations of Parameters into the Matrix.
	// Note that Include is in preview mode and not yet supported.
	// +optional
	// +listType=atomic
	Include []MatrixInclude `json:"include,omitempty"`
}

// MatrixInclude allows passing in a specific combinations of Parameters into the Matrix.
// Note this struct is in preview mode and not yet supported
type MatrixInclude struct {
	// Name the specified combination
	Name string `json:"name,omitempty"`

	// Params takes only `Parameters` of type `"string"`
	// The names of the `params` must match the names of the `params` in the underlying `Task`
	// +listType=atomic
	Params Params `json:"params,omitempty"`
}

// Combination is a map, mainly defined to hold a single combination from a Matrix with key as param.Name and value as param.Value
type Combination map[string]string

// Combinations is a list of combination
type Combinations []Combination

// ToParams transforms Combinations from a slice of map[string]string to a slice of Params
// such that, the combinations can be directly consumed in creating taskRun/run object
func (cs Combinations) ToParams() []Params {
	listOfParams := make([]Params, len(cs))
	for i := range cs {
		var params Params
		combination := cs[i]
		order, sortedCombination := combination.sortCombination()
		for _, key := range order {
			params = append(params, Param{
				Name:  key,
				Value: ParamValue{Type: ParamTypeString, StringVal: sortedCombination[key]},
			})
		}
		listOfParams[i] = params
	}
	return listOfParams
}

// fanOut generates a new combination based on a given Parameter in the Matrix.
func (cs Combinations) fanOut(param Param) Combinations {
	if len(cs) == 0 {
		return initializeCombinations(param)
	}
	return cs.distribute(param)
}

// distribute generates a new combination of Parameters by adding a new Parameter to an existing list of Combinations.
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

// initializeCombinations generates a new combination based on the first Parameter in the Matrix.
func initializeCombinations(param Param) Combinations {
	var combinations Combinations
	for _, value := range param.Value.ArrayVal {
		combinations = append(combinations, Combination{param.Name: value})
	}
	return combinations
}

// sortCombination sorts the given Combination based on the param names to produce a deterministic ordering
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

// FanOut produces combinations of Parameters of type String from a slice of Parameters of type Array.
func (m *Matrix) FanOut() Combinations {
	var combinations Combinations
	for _, parameter := range m.Params {
		combinations = combinations.fanOut(parameter)
	}
	return combinations
}

// CountCombinations returns the count of combinations of Parameters generated from the Matrix in PipelineTask.
func (m *Matrix) CountCombinations() int {
	// Iterate over matrix.params and compute count of all generated combinations
	count := m.countGeneratedCombinationsFromParams()

	// Add any additional combinations generated from matrix include params
	count += m.countNewCombinationsFromInclude()

	return count
}

// countGeneratedCombinationsFromParams returns the count of combinations of Parameters generated from the matrix
// parameters
func (m *Matrix) countGeneratedCombinationsFromParams() int {
	if !m.hasParams() {
		return 0
	}
	count := 1
	for _, param := range m.Params {
		count *= len(param.Value.ArrayVal)
	}
	return count
}

// countNewCombinationsFromInclude returns the count of combinations of Parameters generated from the matrix
// include parameters
func (m *Matrix) countNewCombinationsFromInclude() int {
	if !m.hasInclude() {
		return 0
	}
	if !m.hasParams() {
		return len(m.Include)
	}
	count := 0
	matrixParamMap := m.Params.extractParamMapArrVals()
	for _, include := range m.Include {
		for _, param := range include.Params {
			if val, exist := matrixParamMap[param.Name]; exist {
				// If the matrix include param values does not exist, a new combination will be generated
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

func (m *Matrix) hasInclude() bool {
	return m != nil && m.Include != nil && len(m.Include) > 0
}

func (m *Matrix) hasParams() bool {
	return m != nil && m.Params != nil && len(m.Params) > 0
}

func (m *Matrix) getParamNames() []string {
	var names []string
	if m.hasParams() {
		names = m.Params.extractNames()
	}
	if m.hasInclude() {
		for _, include := range m.Include {
			names = append(names, include.Params.extractNames()...)
		}
	}
	return names
}

func (m *Matrix) validateCombinationsCount(ctx context.Context) (errs *apis.FieldError) {
	matrixCombinationsCount := m.CountCombinations()
	maxMatrixCombinationsCount := config.FromContextOrDefaults(ctx).Defaults.DefaultMaxMatrixCombinationsCount
	if matrixCombinationsCount > maxMatrixCombinationsCount {
		errs = errs.Also(apis.ErrOutOfBoundsValue(matrixCombinationsCount, 0, maxMatrixCombinationsCount, "matrix"))
	}
	return errs
}

// validateParams validates the type of parameter
// for Matrix.Params and Matrix.Include.Params
// Matrix.Params must be of type array. Matrix.Include.Params must be of type string.
// validateParams also validates Matrix.Params for a unique list of params
// and a unique list of params in each Matrix.Include.Params specification
func (m *Matrix) validateParams() (errs *apis.FieldError) {
	if m != nil {
		if m.hasInclude() {
			for i, include := range m.Include {
				errs = errs.Also(include.Params.validateDuplicateParameters().ViaField(fmt.Sprintf("matrix.include[%d].params", i)))
				for _, param := range include.Params {
					if param.Value.Type != ParamTypeString {
						errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("parameters of type string only are allowed, but got param type %s", string(param.Value.Type)), "").ViaFieldKey("matrix.include.params", param.Name))
					}
				}
			}
		}
		if m.hasParams() {
			errs = errs.Also(m.Params.validateDuplicateParameters().ViaField("matrix.params"))
			for _, param := range m.Params {
				if param.Value.Type != ParamTypeArray {
					errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("parameters of type array only are allowed, but got param type %s", string(param.Value.Type)), "").ViaFieldKey("matrix.params", param.Name))
				}
			}
		}
	}
	return errs
}

// validatePipelineParametersVariablesInMatrixParameters validates all pipeline paramater variables including Matrix.Params and Matrix.Include.Params
// that may contain the reference(s) to other params to make sure those references are used appropriately.
func (m *Matrix) validatePipelineParametersVariablesInMatrixParameters(prefix string, paramNames sets.String, arrayParamNames sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	if m.hasInclude() {
		for _, include := range m.Include {
			for idx, param := range include.Params {
				stringElement := param.Value.StringVal
				// Matrix Include Params must be of type string
				errs = errs.Also(validateStringVariable(stringElement, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaFieldIndex("", idx).ViaField("matrix.include.params", ""))
			}
		}
	}
	if m.hasParams() {
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
	matrixParamNames := sets.NewString(m.getParamNames()...)
	for _, param := range params {
		if matrixParamNames.Has(param.Name) {
			errs = errs.Also(apis.ErrMultipleOneOf("matrix["+param.Name+"]", "params["+param.Name+"]"))
		}
	}
	return errs
}
