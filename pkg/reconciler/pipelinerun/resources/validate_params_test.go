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

package resources

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
)

func TestValidateParamTypesMatching_Valid(t *testing.T) {
	stringValue := *v1beta1.NewStructuredValues("stringValue")
	arrayValue := *v1beta1.NewStructuredValues("arrayValue", "arrayValue")

	for _, tc := range []struct {
		name        string
		description string
		pp          []v1beta1.ParamSpec
		prp         []v1beta1.Param
	}{{
		name: "proper param types",
		pp: []v1beta1.ParamSpec{
			{Name: "correct-type-1", Type: v1beta1.ParamTypeString},
			{Name: "correct-type-2", Type: v1beta1.ParamTypeArray},
		},
		prp: []v1beta1.Param{
			{Name: "correct-type-1", Value: stringValue},
			{Name: "correct-type-2", Value: arrayValue},
		},
	}, {
		name: "no params to get wrong",
		pp:   []v1beta1.ParamSpec{},
		prp:  []v1beta1.Param{},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ps := &v1beta1.PipelineSpec{Params: tc.pp}
			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
				Spec:       v1beta1.PipelineRunSpec{Params: tc.prp},
			}

			if err := ValidateParamTypesMatching(ps, pr); err != nil {
				t.Errorf("Pipeline.Validate() returned error: %v", err)
			}
		})
	}
}

func TestValidateParamTypesMatching_Invalid(t *testing.T) {
	stringValue := *v1beta1.NewStructuredValues("stringValue")
	arrayValue := *v1beta1.NewStructuredValues("arrayValue", "arrayValue")

	for _, tc := range []struct {
		name        string
		description string
		pp          []v1beta1.ParamSpec
		prp         []v1beta1.Param
	}{{
		name: "string-array mismatch",
		pp: []v1beta1.ParamSpec{
			{Name: "correct-type-1", Type: v1beta1.ParamTypeString},
			{Name: "correct-type-2", Type: v1beta1.ParamTypeArray},
			{Name: "incorrect-type", Type: v1beta1.ParamTypeString},
		},
		prp: []v1beta1.Param{
			{Name: "correct-type-1", Value: stringValue},
			{Name: "correct-type-2", Value: arrayValue},
			{Name: "incorrect-type", Value: arrayValue},
		},
	}, {
		name: "array-string mismatch",
		pp: []v1beta1.ParamSpec{
			{Name: "correct-type-1", Type: v1beta1.ParamTypeString},
			{Name: "correct-type-2", Type: v1beta1.ParamTypeArray},
			{Name: "incorrect-type", Type: v1beta1.ParamTypeArray},
		},
		prp: []v1beta1.Param{
			{Name: "correct-type-1", Value: stringValue},
			{Name: "correct-type-2", Value: arrayValue},
			{Name: "incorrect-type", Value: stringValue},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ps := &v1beta1.PipelineSpec{Params: tc.pp}
			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
				Spec:       v1beta1.PipelineRunSpec{Params: tc.prp},
			}

			if err := ValidateParamTypesMatching(ps, pr); err == nil {
				t.Errorf("Expected to see error when validating PipelineRun/Pipeline param types but saw none")
			}
		})
	}
}

func TestValidateRequiredParametersProvided_Valid(t *testing.T) {
	stringValue := *v1beta1.NewStructuredValues("stringValue")
	arrayValue := *v1beta1.NewStructuredValues("arrayValue", "arrayValue")

	for _, tc := range []struct {
		name        string
		description string
		pp          []v1beta1.ParamSpec
		prp         []v1beta1.Param
	}{{
		name: "required string params provided",
		pp: []v1beta1.ParamSpec{
			{Name: "required-string-param", Type: v1beta1.ParamTypeString},
		},
		prp: []v1beta1.Param{
			{Name: "required-string-param", Value: stringValue},
		},
	}, {
		name: "required array params provided",
		pp: []v1beta1.ParamSpec{
			{Name: "required-array-param", Type: v1beta1.ParamTypeArray},
		},
		prp: []v1beta1.Param{
			{Name: "required-array-param", Value: arrayValue},
		},
	}, {
		name: "string params provided in default",
		pp: []v1beta1.ParamSpec{
			{Name: "string-param", Type: v1beta1.ParamTypeString, Default: &stringValue},
		},
		prp: []v1beta1.Param{
			{Name: "another-string-param", Value: stringValue},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRequiredParametersProvided(&tc.pp, &tc.prp); err != nil {
				t.Errorf("Didn't expect to see error when validating valid PipelineRun parameters but got: %v", err)
			}
		})
	}
}

func TestValidateRequiredParametersProvided_Invalid(t *testing.T) {
	stringValue := *v1beta1.NewStructuredValues("stringValue")
	arrayValue := *v1beta1.NewStructuredValues("arrayValue", "arrayValue")

	for _, tc := range []struct {
		name        string
		description string
		pp          []v1beta1.ParamSpec
		prp         []v1beta1.Param
	}{{
		name: "required string param missing",
		pp: []v1beta1.ParamSpec{
			{Name: "required-string-param", Type: v1beta1.ParamTypeString},
		},
		prp: []v1beta1.Param{
			{Name: "another-string-param", Value: stringValue},
		},
	}, {
		name: "required array param missing",
		pp: []v1beta1.ParamSpec{
			{Name: "required-array-param", Type: v1beta1.ParamTypeArray},
		},
		prp: []v1beta1.Param{
			{Name: "another-array-param", Value: arrayValue},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRequiredParametersProvided(&tc.pp, &tc.prp); err == nil {
				t.Errorf("Expected to see error when validating invalid PipelineRun parameters but saw none")
			}
		})
	}
}

func TestValidateObjectParamRequiredKeys_Invalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		pp   []v1beta1.ParamSpec
		prp  []v1beta1.Param
	}{{
		name: "miss all required keys",
		pp: []v1beta1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
			},
		},
		prp: []v1beta1.Param{
			{
				Name: "an-object-param",
				Value: *v1beta1.NewObject(map[string]string{
					"foo": "val1",
				})},
		},
	}, {
		name: "miss one of the required keys",
		pp: []v1beta1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
			},
		},
		prp: []v1beta1.Param{
			{
				Name: "an-object-param",
				Value: *v1beta1.NewObject(map[string]string{
					"key1": "foo",
				})},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateObjectParamRequiredKeys(tc.pp, tc.prp); err == nil {
				t.Errorf("Expected to see error when validating invalid object parameter keys but saw none")
			}
		})
	}
}

func TestValidateObjectParamRequiredKeys_Valid(t *testing.T) {
	for _, tc := range []struct {
		name string
		pp   []v1beta1.ParamSpec
		prp  []v1beta1.Param
	}{{
		name: "some keys are provided by default, and the rest are provided in value",
		pp: []v1beta1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
				Default: &v1beta1.ParamValue{
					Type: v1beta1.ParamTypeObject,
					ObjectVal: map[string]string{
						"key1": "val1",
					},
				},
			},
		},
		prp: []v1beta1.Param{
			{
				Name: "an-object-param",
				Value: *v1beta1.NewObject(map[string]string{
					"key2": "val2",
				})},
		},
	}, {
		name: "all keys are provided with a value",
		pp: []v1beta1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
			},
		},
		prp: []v1beta1.Param{
			{
				Name: "an-object-param",
				Value: *v1beta1.NewObject(map[string]string{
					"key1": "val1",
					"key2": "val2",
				})},
		},
	}, {
		name: "extra keys are provided",
		pp: []v1beta1.ParamSpec{
			{
				Name: "an-object-param",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {Type: "string"},
					"key2": {Type: "string"},
				},
			},
		},
		prp: []v1beta1.Param{
			{
				Name: "an-object-param",
				Value: *v1beta1.NewObject(map[string]string{
					"key1": "val1",
					"key2": "val2",
					"key3": "val3",
				})},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateObjectParamRequiredKeys(tc.pp, tc.prp); err != nil {
				t.Errorf("Didn't expect to see error when validating invalid object parameter keys but got: %v", err)
			}
		})
	}
}

func TestValidateParamArrayIndex_valid(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	cfg.FeatureFlags.EnableAPIFields = config.BetaAPIFields
	ctx = config.ToContext(ctx, cfg)
	for _, tt := range []struct {
		name     string
		original v1beta1.PipelineSpec
		params   []v1beta1.Param
	}{{
		name: "single parameter",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[1])")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("$(params.second-param[0])")},
					{Name: "first-task-third-param", Value: *v1beta1.NewStructuredValues("static value")},
				},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
	}, {
		name: "single parameter with when expression",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "$(params.first-param[1])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[0])"},
				}},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
	}, {
		name: "pipeline parameter nested inside task parameter",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("$(input.workspace.$(params.first-param[0]))")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("$(input.workspace.$(params.second-param[1]))")},
				},
			}},
		},
		params: nil, // no parameter values.
	}, {
		name: "array parameter",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default", "array", "value")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("firstelement", "$(params.first-param)")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("firstelement", "$(params.second-param[0])")},
				},
			}},
		},
		params: []v1beta1.Param{
			{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "array")},
		},
	}, {
		name: "parameter evaluation with final tasks",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Finally: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[0])")},
					{Name: "final-task-second-param", Value: *v1beta1.NewStructuredValues("$(params.second-param[1])")},
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(params.first-param[0])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[1])"},
				}},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
	}, {
		name: "parameter evaluation with both tasks and final tasks",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[0])")},
					{Name: "final-task-second-param", Value: *v1beta1.NewStructuredValues("$(params.second-param[1])")},
				},
			}},
			Finally: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[0])")},
					{Name: "final-task-second-param", Value: *v1beta1.NewStructuredValues("$(params.second-param[1])")},
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(params.first-param[0])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[1])"},
				}},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
	}, {
		name: "parameter references with bracket notation and special characters",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first.param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second/param", Type: v1beta1.ParamTypeArray},
				{Name: "third.param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "fourth/param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues(`$(params["first.param"][0])`)},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues(`$(params["second/param"][0])`)},
					{Name: "first-task-third-param", Value: *v1beta1.NewStructuredValues(`$(params['third.param'][1])`)},
					{Name: "first-task-fourth-param", Value: *v1beta1.NewStructuredValues(`$(params['fourth/param'][1])`)},
					{Name: "first-task-fifth-param", Value: *v1beta1.NewStructuredValues("static value")},
				},
			}},
		},
		params: []v1beta1.Param{
			{Name: "second/param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")},
			{Name: "fourth/param", Value: *v1beta1.NewStructuredValues("fourth-value", "fourth-value-again")},
		},
	}, {
		name: "single parameter in workspace subpath",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[0])")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("static value")},
				},
				Workspaces: []v1beta1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "$(params.second-param[1])",
					},
				},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
	},
	} {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := &v1beta1.PipelineRun{
				Spec: v1beta1.PipelineRunSpec{
					Params: tt.params,
				},
			}
			err := ValidateParamArrayIndex(ctx, &tt.original, run)
			if err != nil {
				t.Errorf("ValidateParamArrayIndex() got err %s", err)
			}
		})
	}
}

func TestValidateParamArrayIndex_invalid(t *testing.T) {
	for _, tt := range []struct {
		name      string
		original  v1beta1.PipelineSpec
		params    []v1beta1.Param
		apifields string
		expected  error
	}{{
		name: "single parameter reference out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[2])")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("$(params.second-param[2])")},
					{Name: "first-task-third-param", Value: *v1beta1.NewStructuredValues("static value")},
				},
			}},
		},
		params:   []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
		expected: fmt.Errorf("non-existent param references:[$(params.first-param[2]) $(params.second-param[2])]"),
	}, {
		name: "single parameter reference with when expression out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "$(params.first-param[2])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[2])"},
				}},
			}},
		},
		params:   []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
		expected: fmt.Errorf("non-existent param references:[$(params.first-param[2]) $(params.second-param[2])]"),
	}, {
		name: "pipeline parameter reference nested inside task parameter out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("$(input.workspace.$(params.first-param[2]))")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("$(input.workspace.$(params.second-param[2]))")},
				},
			}},
		},
		params:   nil, // no parameter values.
		expected: fmt.Errorf("non-existent param references:[$(params.first-param[2]) $(params.second-param[2])]"),
	}, {
		name: "array parameter reference out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default", "array", "value")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("firstelement", "$(params.first-param[3])")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("firstelement", "$(params.second-param[4])")},
				},
			}},
		},
		params: []v1beta1.Param{
			{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "array")},
		},
		expected: fmt.Errorf("non-existent param references:[$(params.first-param[3]) $(params.second-param[4])]"),
	}, {
		name: "object parameter reference out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default", "array", "value")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewObject(map[string]string{
						"val1": "$(params.first-param[4])",
						"val2": "$(params.second-param[4])",
					})},
				},
			}},
		},
		params: []v1beta1.Param{
			{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "array")},
		},
		expected: fmt.Errorf("non-existent param references:[$(params.first-param[4]) $(params.second-param[4])]"),
	}, {
		name: "parameter evaluation with final tasks reference out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Finally: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[2])")},
					{Name: "final-task-second-param", Value: *v1beta1.NewStructuredValues("$(params.second-param[2])")},
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(params.first-param[0])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[1])"},
				}},
			}},
		},
		params:   []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
		expected: fmt.Errorf("non-existent param references:[$(params.first-param[2]) $(params.second-param[2])]"),
	}, {
		name: "parameter evaluation with both tasks and final tasks reference out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[2])")},
					{Name: "final-task-second-param", Value: *v1beta1.NewStructuredValues("$(params.second-param[2])")},
				},
			}},
			Finally: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[3])")},
					{Name: "final-task-second-param", Value: *v1beta1.NewStructuredValues("$(params.second-param[3])")},
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(params.first-param[2])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[2])"},
				}},
			}},
		},
		params:   []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
		expected: fmt.Errorf("non-existent param references:[$(params.first-param[2]) $(params.first-param[3]) $(params.second-param[2]) $(params.second-param[3])]"),
	}, {
		name: "parameter references with bracket notation and special characters reference out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first.param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second/param", Type: v1beta1.ParamTypeArray},
				{Name: "third.param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "fourth/param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues(`$(params["first.param"][2])`)},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues(`$(params["second/param"][2])`)},
					{Name: "first-task-third-param", Value: *v1beta1.NewStructuredValues(`$(params['third.param'][2])`)},
					{Name: "first-task-fourth-param", Value: *v1beta1.NewStructuredValues(`$(params['fourth/param'][2])`)},
					{Name: "first-task-fifth-param", Value: *v1beta1.NewStructuredValues("static value")},
				},
			}},
		},
		params: []v1beta1.Param{
			{Name: "second/param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")},
			{Name: "fourth/param", Value: *v1beta1.NewStructuredValues("fourth-value", "fourth-value-again")},
		},
		expected: fmt.Errorf("non-existent param references:[$(params[\"first.param\"][2]) $(params[\"second/param\"][2]) $(params['fourth/param'][2]) $(params['third.param'][2])]"),
	}, {
		name: "single parameter in workspace subpath reference out of bound",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[2])")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("static value")},
				},
				Workspaces: []v1beta1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "$(params.second-param[3])",
					},
				},
			}},
		},
		params:   []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
		expected: fmt.Errorf("non-existent param references:[$(params.first-param[2]) $(params.second-param[3])]"),
	}, {
		name: "alpha gate not enabled",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewStructuredValues("$(params.first-param[2])")},
					{Name: "first-task-second-param", Value: *v1beta1.NewStructuredValues("static value")},
				},
				Workspaces: []v1beta1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "$(params.second-param[3])",
					},
				},
			}},
		},
		params:    []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewStructuredValues("second-value", "second-value-again")}},
		apifields: "stable",
		expected:  fmt.Errorf(`indexing into array param %s requires "enable-api-fields" feature gate to be "alpha" or "beta"`, "$(params.first-param[2])"),
	},
	} {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.apifields != "stable" {
				ctx = config.ToContext(ctx, &config.Config{FeatureFlags: &config.FeatureFlags{EnableAPIFields: "beta"}})
			}
			run := &v1beta1.PipelineRun{
				Spec: v1beta1.PipelineRunSpec{
					Params: tt.params,
				},
			}
			err := ValidateParamArrayIndex(ctx, &tt.original, run)
			if d := cmp.Diff(tt.expected.Error(), err.Error()); d != "" {
				t.Errorf("ValidateParamArrayIndex() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
