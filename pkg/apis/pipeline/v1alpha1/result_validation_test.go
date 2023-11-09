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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	"knative.dev/pkg/apis"
)

func TestResultsValidate(t *testing.T) {
	tests := []struct {
		name   string
		Result StepActionResult
	}{{
		name: "valid result type empty",
		Result: StepActionResult{
			Name:        "MY-RESULT",
			Description: "my great result",
		},
	}, {
		name: "valid result type string",
		Result: StepActionResult{
			Name:        "MY-RESULT",
			Type:        v1.ResultsTypeString,
			Description: "my great result",
		},
	}, {
		name: "valid result type array",
		Result: StepActionResult{
			Name:        "MY-RESULT",
			Type:        v1.ResultsTypeArray,
			Description: "my great result",
		},
	}, {
		name: "valid result type object",
		Result: StepActionResult{
			Name:        "MY-RESULT",
			Type:        v1.ResultsTypeObject,
			Description: "my great result",
			Properties:  map[string]v1.PropertySpec{"hello": {Type: v1.ParamTypeString}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if err := tt.Result.validate(ctx); err != nil {
				t.Errorf("TaskSpec.Validate() = %v", err)
			}
		})
	}
}

func TestResultsValidateError(t *testing.T) {
	tests := []struct {
		name          string
		Result        StepActionResult
		expectedError apis.FieldError
	}{{
		name: "invalid result type",
		Result: StepActionResult{
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
		Result: StepActionResult{
			Name:        "MY-RESULT",
			Type:        v1.ResultsTypeObject,
			Description: "my great result",
			Properties:  map[string]v1.PropertySpec{"hello": {Type: "wrong type"}},
		},
		expectedError: apis.FieldError{
			Message: "The value type specified for these keys [hello] is invalid, the type must be string",
			Paths:   []string{"MY-RESULT.properties"},
		},
	}, {
		name: "invalid object properties empty",
		Result: StepActionResult{
			Name:        "MY-RESULT",
			Type:        v1.ResultsTypeObject,
			Description: "my great result",
		},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"MY-RESULT.properties"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.Result.validate(context.Background())
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", tt.Result)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
