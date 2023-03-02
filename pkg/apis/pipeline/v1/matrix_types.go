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

	"github.com/tektoncd/pipeline/pkg/apis/config"
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

// FanOut produces combinations of Parameters of type String from a slice of Parameters of type Array.
func (m *Matrix) FanOut() []Params {
	var combinations []Params
	for _, parameter := range m.Params {
		combinations = fanOut(parameter, combinations)
	}
	return combinations
}

// fanOut generates a new combination based on a given Parameter in the Matrix.
func fanOut(param Param, combinations []Params) []Params {
	if len(combinations) == 0 {
		return initializeCombinations(param)
	}
	return distribute(param, combinations)
}

// distribute generates a new combination of Parameters by adding a new Parameter to an existing list of Combinations.
func distribute(param Param, combinations []Params) []Params {
	var expandedCombinations []Params
	for _, value := range param.Value.ArrayVal {
		for _, combination := range combinations {
			expandedCombinations = append(expandedCombinations, createCombination(param.Name, value, combination))
		}
	}
	return expandedCombinations
}

// initializeCombinations generates a new combination based on the first Parameter in the Matrix.
func initializeCombinations(param Param) []Params {
	var combinations []Params
	for _, value := range param.Value.ArrayVal {
		combinations = append(combinations, createCombination(param.Name, value, []Param{}))
	}
	return combinations
}

func createCombination(name string, value string, combination Params) Params {
	combination = append(combination, Param{
		Name:  name,
		Value: ParamValue{Type: ParamTypeString, StringVal: value},
	})
	return combination
}

// CountCombinations returns the count of combinations of Parameters generated from the Matrix in PipelineTask.
func (m *Matrix) CountCombinations() int {
	count := 0
	if m.hasParams() {
		count = 1
		for _, param := range m.Params {
			count *= len(param.Value.ArrayVal)
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

func (m *Matrix) validateCombinationsCount(ctx context.Context) (errs *apis.FieldError) {
	matrixCombinationsCount := m.CountCombinations()
	maxMatrixCombinationsCount := config.FromContextOrDefaults(ctx).Defaults.DefaultMaxMatrixCombinationsCount
	if matrixCombinationsCount > maxMatrixCombinationsCount {
		errs = errs.Also(apis.ErrOutOfBoundsValue(matrixCombinationsCount, 0, maxMatrixCombinationsCount, "matrix"))
	}
	return errs
}

// validateParamTypes validates the type of parameter
// for Matrix.Params and Matrix.Include.Params
// Matrix.Params must be of type array. Matrix.Include.Params must be of type string.
func (m *Matrix) validateParamTypes() (errs *apis.FieldError) {
	if m != nil {
		if m.hasInclude() {
			for _, include := range m.Include {
				for _, param := range include.Params {
					if param.Value.Type != ParamTypeString {
						errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("parameters of type string only are allowed, but got param type %s", string(param.Value.Type)), "").ViaFieldKey("matrix.include.params", param.Name))
					}
				}
			}
		}
		if m.hasParams() {
			for _, param := range m.Params {
				if param.Value.Type != ParamTypeArray {
					errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("parameters of type array only are allowed, but got param type %s", string(param.Value.Type)), "").ViaFieldKey("matrix.params", param.Name))
				}
			}
		}
	}
	return errs
}
