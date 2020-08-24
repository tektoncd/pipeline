/*
Copyright 2020 The Tekton Authors

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
	"testing"

	"k8s.io/apimachinery/pkg/selection"
)

func TestWhenExpression_Valid(t *testing.T) {
	tests := []struct {
		name           string
		whenExpression WhenExpression
	}{{
		name: "valid operator - in - and values",
		whenExpression: WhenExpression{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
	}, {
		name: "valid operator - in - and values",
		whenExpression: WhenExpression{
			Input:    "bar",
			Operator: selection.NotIn,
			Values:   []string{"foo"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.whenExpression.Validate(); err != nil {
				t.Errorf("WhenExpression.Validate() returned an error for valid when expression: %s, %s", tt.name, tt.whenExpression)
			}
		})
	}
}

func TestWhenExpression_Invalid(t *testing.T) {
	tests := []struct {
		name           string
		whenExpression WhenExpression
	}{{
		name: "invalid operator - exists",
		whenExpression: WhenExpression{
			Input:    "foo",
			Operator: selection.Exists,
			Values:   []string{"foo"},
		},
	}, {
		name: "empty values",
		whenExpression: WhenExpression{
			Input:    "bar",
			Operator: selection.NotIn,
			Values:   []string{},
		},
	}, {
		name: "missing values",
		whenExpression: WhenExpression{
			Input:    "bar",
			Operator: selection.NotIn,
		},
	}, {
		name: "missing operator",
		whenExpression: WhenExpression{
			Input:  "bar",
			Values: []string{"foo"},
		},
	}, {
		name:           "missing expression",
		whenExpression: WhenExpression{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.whenExpression.Validate(); err == nil {
				t.Errorf("WhenExpression.Validate() did not return an error for invalid when expression: %s, %s", tt.name, tt.whenExpression)
			}
		})
	}
}
