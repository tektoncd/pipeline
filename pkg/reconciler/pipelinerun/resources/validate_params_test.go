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

package resources_test

import (
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resources "github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateParamTypesMatching_Valid(t *testing.T) {
	stringValue := *v1.NewStructuredValues("stringValue")
	arrayValue := *v1.NewStructuredValues("arrayValue", "arrayValue")

	for _, tc := range []struct {
		name        string
		description string
		pp          []v1.ParamSpec
		prp         []v1.Param
	}{{
		name: "proper param types",
		pp: []v1.ParamSpec{
			{Name: "correct-type-1", Type: v1.ParamTypeString},
			{Name: "correct-type-2", Type: v1.ParamTypeArray},
		},
		prp: v1.Params{
			{Name: "correct-type-1", Value: stringValue},
			{Name: "correct-type-2", Value: arrayValue},
		},
	}, {
		name: "no params to get wrong",
		pp:   []v1.ParamSpec{},
		prp:  v1.Params{},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ps := &v1.PipelineSpec{Params: tc.pp}
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
				Spec:       v1.PipelineRunSpec{Params: tc.prp},
			}

			if err := resources.ValidateParamTypesMatching(ps, pr); err != nil {
				t.Errorf("Pipeline.Validate() returned error: %v", err)
			}
		})
	}
}

func TestValidateParamTypesMatching_Invalid(t *testing.T) {
	stringValue := *v1.NewStructuredValues("stringValue")
	arrayValue := *v1.NewStructuredValues("arrayValue", "arrayValue")

	for _, tc := range []struct {
		name        string
		description string
		pp          []v1.ParamSpec
		prp         []v1.Param
	}{{
		name: "string-array mismatch",
		pp: []v1.ParamSpec{
			{Name: "correct-type-1", Type: v1.ParamTypeString},
			{Name: "correct-type-2", Type: v1.ParamTypeArray},
			{Name: "incorrect-type", Type: v1.ParamTypeString},
		},
		prp: v1.Params{
			{Name: "correct-type-1", Value: stringValue},
			{Name: "correct-type-2", Value: arrayValue},
			{Name: "incorrect-type", Value: arrayValue},
		},
	}, {
		name: "array-string mismatch",
		pp: []v1.ParamSpec{
			{Name: "correct-type-1", Type: v1.ParamTypeString},
			{Name: "correct-type-2", Type: v1.ParamTypeArray},
			{Name: "incorrect-type", Type: v1.ParamTypeArray},
		},
		prp: v1.Params{
			{Name: "correct-type-1", Value: stringValue},
			{Name: "correct-type-2", Value: arrayValue},
			{Name: "incorrect-type", Value: stringValue},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ps := &v1.PipelineSpec{Params: tc.pp}
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
				Spec:       v1.PipelineRunSpec{Params: tc.prp},
			}

			if err := resources.ValidateParamTypesMatching(ps, pr); err == nil {
				t.Errorf("Expected to see error when validating PipelineRun/Pipeline param types but saw none")
			}
		})
	}
}

func TestValidateRequiredParametersProvided_Valid(t *testing.T) {
	stringValue := *v1.NewStructuredValues("stringValue")
	arrayValue := *v1.NewStructuredValues("arrayValue", "arrayValue")

	for _, tc := range []struct {
		name        string
		description string
		pp          v1.ParamSpecs
		prp         v1.Params
	}{{
		name: "required string params provided",
		pp: []v1.ParamSpec{
			{Name: "required-string-param", Type: v1.ParamTypeString},
		},
		prp: v1.Params{
			{Name: "required-string-param", Value: stringValue},
		},
	}, {
		name: "required array params provided",
		pp: []v1.ParamSpec{
			{Name: "required-array-param", Type: v1.ParamTypeArray},
		},
		prp: v1.Params{
			{Name: "required-array-param", Value: arrayValue},
		},
	}, {
		name: "string params provided in default",
		pp: []v1.ParamSpec{
			{Name: "string-param", Type: v1.ParamTypeString, Default: &stringValue},
		},
		prp: v1.Params{
			{Name: "another-string-param", Value: stringValue},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := resources.ValidateRequiredParametersProvided(&tc.pp, &tc.prp); err != nil {
				t.Errorf("Didn't expect to see error when validating valid PipelineRun parameters but got: %v", err)
			}
		})
	}
}

func TestValidateRequiredParametersProvided_Invalid(t *testing.T) {
	stringValue := *v1.NewStructuredValues("stringValue")
	arrayValue := *v1.NewStructuredValues("arrayValue", "arrayValue")

	for _, tc := range []struct {
		name        string
		description string
		pp          v1.ParamSpecs
		prp         v1.Params
	}{{
		name: "required string param missing",
		pp: []v1.ParamSpec{
			{Name: "required-string-param", Type: v1.ParamTypeString},
		},
		prp: v1.Params{
			{Name: "another-string-param", Value: stringValue},
		},
	}, {
		name: "required array param missing",
		pp: []v1.ParamSpec{
			{Name: "required-array-param", Type: v1.ParamTypeArray},
		},
		prp: v1.Params{
			{Name: "another-array-param", Value: arrayValue},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := resources.ValidateRequiredParametersProvided(&tc.pp, &tc.prp); err == nil {
				t.Errorf("Expected to see error when validating invalid PipelineRun parameters but saw none")
			}
		})
	}
}

func TestValidateObjectParamRequiredKeys_Invalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		pp   []v1.ParamSpec
		prp  v1.Params
	}{{
		name: "miss all required keys",
		pp: []v1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
			},
		},
		prp: v1.Params{
			{
				Name: "an-object-param",
				Value: *v1.NewObject(map[string]string{
					"foo": "val1",
				})},
		},
	}, {
		name: "miss one of the required keys",
		pp: []v1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
			},
		},
		prp: v1.Params{
			{
				Name: "an-object-param",
				Value: *v1.NewObject(map[string]string{
					"key1": "foo",
				})},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := resources.ValidateObjectParamRequiredKeys(tc.pp, tc.prp); err == nil {
				t.Errorf("Expected to see error when validating invalid object parameter keys but saw none")
			}
		})
	}
}

func TestValidateObjectParamRequiredKeys_Valid(t *testing.T) {
	for _, tc := range []struct {
		name string
		pp   []v1.ParamSpec
		prp  v1.Params
	}{{
		name: "some keys are provided by default, and the rest are provided in value",
		pp: []v1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
				Default: &v1.ParamValue{
					Type: v1.ParamTypeObject,
					ObjectVal: map[string]string{
						"key1": "val1",
					},
				},
			},
		},
		prp: v1.Params{
			{
				Name: "an-object-param",
				Value: *v1.NewObject(map[string]string{
					"key2": "val2",
				})},
		},
	}, {
		name: "all keys are provided with a value",
		pp: []v1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
			},
		},
		prp: v1.Params{
			{
				Name: "an-object-param",
				Value: *v1.NewObject(map[string]string{
					"key1": "val1",
					"key2": "val2",
				})},
		},
	}, {
		name: "extra keys are provided",
		pp: []v1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
			},
		},
		prp: v1.Params{
			{
				Name: "an-object-param",
				Value: *v1.NewObject(map[string]string{
					"key1": "val1",
					"key2": "val2",
					"key3": "val3",
				})},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := resources.ValidateObjectParamRequiredKeys(tc.pp, tc.prp); err != nil {
				t.Errorf("Didn't expect to see error when validating invalid object parameter keys but got: %v", err)
			}
		})
	}
}

// ValidateMatrixPipelineParameterTypes tests that a pipeline task with
// a matrix has the correct parameter types after any replacements are made
func TestValidatePipelineParameterTypes(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		state    resources.PipelineRunState
		wantErrs string
	}{{
		desc: "parameters in matrix are arrays",
		state: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name: "task",
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name: "foobar", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
					}, {
						Name: "barfoo", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"bar", "foo"}}}},
				},
			},
		}},
	}, {
		desc: "parameters in matrix are strings",
		state: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name: "task",
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name: "foo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "foo"},
					}, {
						Name: "bar", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "bar"},
					}}},
			},
		}},
		wantErrs: "parameters of type array only are allowed, but param foo has type string",
	}, {
		desc: "parameters in include matrix are strings",
		state: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name: "task",
				Matrix: &v1.Matrix{
					Include: v1.IncludeParamsList{{
						Name: "build-1",
						Params: v1.Params{{
							Name: "foo", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "foo"},
						}, {
							Name: "bar", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "bar"}}},
					}}},
			},
		}},
	}, {
		desc: "parameters in include matrix are arrays",
		state: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name: "task",
				Matrix: &v1.Matrix{
					Include: v1.IncludeParamsList{{
						Name: "build-1",
						Params: v1.Params{{
							Name: "foobar", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
						}, {
							Name: "barfoo", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"bar", "foo"}}}},
					}}},
			},
		}},
		wantErrs: "parameters of type string only are allowed, but param foobar has type array",
	}, {
		desc: "parameters in include matrix are objects",
		state: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name: "task",
				Matrix: &v1.Matrix{
					Include: v1.IncludeParamsList{{
						Name: "build-1",
						Params: v1.Params{{
							Name: "barfoo", Value: v1.ParamValue{Type: v1.ParamTypeObject, ObjectVal: map[string]string{
								"url":    "$(params.myObject.non-exist-key)",
								"commit": "$(params.myString)",
							}},
						}, {
							Name: "foobar", Value: v1.ParamValue{Type: v1.ParamTypeObject, ObjectVal: map[string]string{
								"url":    "$(params.myObject.non-exist-key)",
								"commit": "$(params.myString)",
							}},
						}},
					}}},
			},
		}},
		wantErrs: "parameters of type string only are allowed, but param barfoo has type object",
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			err := resources.ValidateParameterTypesInMatrix(tc.state)
			if (err != nil) && err.Error() != tc.wantErrs {
				t.Errorf("expected err: %s, but got err %s", tc.wantErrs, err)
			}
			if tc.wantErrs == "" && err != nil {
				t.Errorf("got unexpected error: %v", err)
			}
		})
	}
}
