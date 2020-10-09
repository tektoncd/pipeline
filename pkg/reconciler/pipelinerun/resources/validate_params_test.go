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
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateParamTypesMatching_Valid(t *testing.T) {

	stringValue := *v1beta1.NewArrayOrString("stringValue")
	arrayValue := *v1beta1.NewArrayOrString("arrayValue", "arrayValue")

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

	stringValue := *v1beta1.NewArrayOrString("stringValue")
	arrayValue := *v1beta1.NewArrayOrString("arrayValue", "arrayValue")

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

	stringValue := *v1beta1.NewArrayOrString("stringValue")
	arrayValue := *v1beta1.NewArrayOrString("arrayValue", "arrayValue")

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

	stringValue := *v1beta1.NewArrayOrString("stringValue")
	arrayValue := *v1beta1.NewArrayOrString("arrayValue", "arrayValue")

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
