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

package v1beta1_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"knative.dev/pkg/apis"
)

func TestResultsValidate(t *testing.T) {
	tests := []struct {
		name   string
		Result v1beta1.TaskResult
	}{{
		name: "valid result type empty",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Description: "my great result",
		},
	}, {
		name: "valid result type string",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Type:        v1beta1.ResultsTypeString,
			Description: "my great result",
		},
	}, {
		name: "valid result type array",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Type:        v1beta1.ResultsTypeArray,
			Description: "my great result",
		},
	}, {
		name: "valid result type object",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Type:        v1beta1.ResultsTypeObject,
			Description: "my great result",
			Properties:  map[string]v1beta1.PropertySpec{"hello": {Type: v1beta1.ParamTypeString}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if err := tt.Result.Validate(ctx); err != nil {
				t.Errorf("TaskSpec.Validate() = %v", err)
			}
		})
	}
}

func TestResultsValidateError(t *testing.T) {
	tests := []struct {
		name          string
		Result        v1beta1.TaskResult
		expectedError apis.FieldError
	}{{
		name: "invalid result type in stable",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Type:        "wrong",
			Description: "my great result",
		},
		expectedError: apis.FieldError{
			Message: `invalid value: wrong`,
			Paths:   []string{"type"},
			Details: "type must be string",
		},
	}, {
		name: "invalid object properties type",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Type:        v1beta1.ResultsTypeObject,
			Description: "my great result",
			Properties:  map[string]v1beta1.PropertySpec{"hello": {Type: "wrong type"}},
		},
		expectedError: apis.FieldError{
			Message: "The value type specified for these keys [hello] is invalid, the type must be string",
			Paths:   []string{"MY-RESULT.properties"},
		},
	}, {
		name: "invalid object properties empty",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Type:        v1beta1.ResultsTypeObject,
			Description: "my great result",
		},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"MY-RESULT.properties"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := tt.Result.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", tt.Result)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResultsValidateValue(t *testing.T) {
	tests := []struct {
		name   string
		Result v1beta1.TaskResult
	}{{
		name: "valid result value",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Description: "my great result",
			Type:        v1beta1.ResultsTypeString,
			Value: &v1beta1.ParamValue{
				Type:      v1beta1.ParamTypeString,
				StringVal: "$(steps.step-name.results.resultName)",
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := config.ToContext(context.Background(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableStepActions: true,
				},
			})
			if err := tt.Result.Validate(ctx); err != nil {
				t.Errorf("TaskSpec.Validate() = %v", err)
			}
		})
	}
}

func TestResultsValidateValueError(t *testing.T) {
	tests := []struct {
		name              string
		Result            v1beta1.TaskResult
		enableStepActions bool
		expectedError     apis.FieldError
	}{{
		name: "enable-step-actions-not-enabled",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Description: "my great result",
			Type:        v1beta1.ResultsTypeString,
			Value: &v1beta1.ParamValue{
				Type:      v1beta1.ParamTypeString,
				StringVal: "$(steps.stepName.results.resultName)",
			},
		},
		enableStepActions: false,
		expectedError: apis.FieldError{
			Message: "feature flag enable-step-actions should be set to true to fetch Results from Steps using StepActions.",
		},
	}, {
		name: "invalid result value type array",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Description: "my great result",
			Type:        v1beta1.ResultsTypeArray,
			Value: &v1beta1.ParamValue{
				Type: v1beta1.ParamTypeArray,
			},
		},
		enableStepActions: true,
		expectedError: apis.FieldError{
			Message: `Invalid Type. Wanted string but got: "array"`,
			Paths:   []string{"MY-RESULT.type"},
		},
	}, {
		name: "invalid result value type object",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Description: "my great result",
			Type:        v1beta1.ResultsTypeObject,
			Properties:  map[string]v1beta1.PropertySpec{"hello": {Type: v1beta1.ParamTypeString}},
			Value: &v1beta1.ParamValue{
				Type: v1beta1.ParamTypeObject,
			},
		},
		enableStepActions: true,
		expectedError: apis.FieldError{
			Message: `Invalid Type. Wanted string but got: "object"`,
			Paths:   []string{"MY-RESULT.type"},
		},
	}, {
		name: "invalid result value format",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Description: "my great result",
			Type:        v1beta1.ResultsTypeString,
			Value: &v1beta1.ParamValue{
				Type:      v1beta1.ParamTypeString,
				StringVal: "not a valid format",
			},
		},
		enableStepActions: true,
		expectedError: apis.FieldError{
			Message: `Could not extract step name and result name. Expected value to look like $(steps.<stepName>.results.<resultName>) but got "not a valid format"`,
			Paths:   []string{"MY-RESULT.value"},
		},
	}, {
		name: "invalid string format invalid step name",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Description: "my great result",
			Type:        v1beta1.ResultsTypeString,
			Value: &v1beta1.ParamValue{
				Type:      v1beta1.ParamTypeString,
				StringVal: "$(steps.foo.foo.results.Bar)",
			},
		},
		enableStepActions: true,
		expectedError: apis.FieldError{
			Message: "invalid extracted step name \"foo.foo\"",
			Paths:   []string{"MY-RESULT.value"},
			Details: "stepName in $(steps.<stepName>.results.<resultName>) must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}, {
		name: "invalid string format invalid result name",
		Result: v1beta1.TaskResult{
			Name:        "MY-RESULT",
			Description: "my great result",
			Type:        v1beta1.ResultsTypeString,
			Value: &v1beta1.ParamValue{
				Type:      v1beta1.ParamTypeString,
				StringVal: "$(steps.foo.results.-bar)",
			},
		},
		enableStepActions: true,
		expectedError: apis.FieldError{
			Message: "invalid extracted result name \"-bar\"",
			Paths:   []string{"MY-RESULT.value"},
			Details: fmt.Sprintf("resultName in $(steps.<stepName>.results.<resultName>) must consist of alphanumeric characters, '-', '_', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my-name',  or 'my_name', regex used for validation is '%s')", v1beta1.ResultNameFormat),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := config.ToContext(context.Background(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableStepActions: tt.enableStepActions,
				},
			})
			err := tt.Result.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", tt.Result)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
