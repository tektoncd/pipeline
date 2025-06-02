/*
Copyright 2022 The Tekton Authors

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
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"k8s.io/apimachinery/pkg/selection"
)

func TestWhenExpressions_Valid(t *testing.T) {
	tests := []struct {
		name string
		wes  WhenExpressions
	}{{
		name: "valid operator - In - and values",
		wes: []WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo"},
		}},
	}, {
		name: "valid operator - NotIn - and values",
		wes: []WhenExpression{{
			Input:    "foo",
			Operator: selection.NotIn,
			Values:   []string{"bar"},
		}},
	}, {
		wes: []WhenExpression{{
			Input:    "$(tasks.a-task.results.output)",
			Operator: selection.In,
			Values:   []string{"bar"},
		}},
	}, {
		wes: []WhenExpression{{ // missing Input defaults to empty string
			Operator: selection.In,
			Values:   []string{""},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.wes.validate(t.Context()); err != nil {
				t.Errorf("WhenExpressions.validate() returned an error for valid when expressions: %s", tt.wes)
			}
		})
	}
}

func TestWhenExpressions_Invalid(t *testing.T) {
	tests := []struct {
		name string
		wes  WhenExpressions
	}{{
		name: "invalid operator - exists",
		wes: []WhenExpression{{
			Input:    "foo",
			Operator: selection.Exists,
			Values:   []string{"foo"},
		}},
	}, {
		name: "invalid values - empty",
		wes: []WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{},
		}},
	}, {
		name: "missing Operator",
		wes: []WhenExpression{{
			Input:  "foo",
			Values: []string{"foo"},
		}},
	}, {
		name: "missing Values",
		wes: []WhenExpression{{
			Input:    "foo",
			Operator: selection.NotIn,
		}},
	}, {
		name: "missing when expression",
		wes:  []WhenExpression{{}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.wes.validate(t.Context()); err == nil {
				t.Errorf("WhenExpressions.validate() did not return error for invalid when expressions: %s, %s", tt.wes, err)
			}
		})
	}
}

func TestCELinWhenExpressions_Valid(t *testing.T) {
	ctx := config.ToContext(t.Context(), &config.Config{
		FeatureFlags: &config.FeatureFlags{
			EnableCELInWhenExpression: true,
		},
	})
	tests := []struct {
		name string
		wes  WhenExpressions
	}{{
		name: "valid operator - Equal",
		wes: []WhenExpression{{
			CEL: " 'foo' == 'foo' ",
		}},
	}, {
		name: "valid operator - NotEqual",
		wes: []WhenExpression{{
			CEL: " 'foo' != 'foo' ",
		}},
	}, {
		name: "valid operator - In",
		wes: []WhenExpression{{
			CEL: "'foo' in ['foo', 'bar']",
		}},
	}, {
		name: "valid operator - NotIn",
		wes: []WhenExpression{{
			CEL: "!('foo' in ['foo', 'bar'])",
		}},
	}, {
		name: "valid regex expression",
		wes: []WhenExpression{{
			CEL: "'release/v1'.matches('release/.*')",
		}},
	}, {
		name: "valid variable reference syntax",
		wes: []WhenExpression{{
			CEL: "'$(params.foo)' in ['foo', 'bar']",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.wes.validate(ctx); err != nil {
				t.Errorf("WhenExpressions.validate() returned an error: %s for valid when expressions: %s", err, tt.wes)
			}
		})
	}
}

func TestCELWhenExpressions_Invalid(t *testing.T) {
	tests := []struct {
		name                      string
		wes                       WhenExpressions
		enableCELInWhenExpression bool
	}{{
		name: "feature flag not set",
		wes: []WhenExpression{{
			CEL: " 'foo' == 'foo' ",
		}},
		enableCELInWhenExpression: false,
	}, {
		name: "variable reference should be wrapped with single quotes",
		wes: []WhenExpression{{
			CEL: " $(params.foo) == 'foo' ",
		}},
		enableCELInWhenExpression: true,
	}, {
		name: "variables not declared in environment",
		wes: []WhenExpression{{
			CEL: " params.foo == 'foo' ",
		}},
		enableCELInWhenExpression: true,
	}, {
		name: "CEL should not coexist with input+operator+values",
		wes: []WhenExpression{{
			CEL:      "'foo' != 'foo'",
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo"},
		}},
		enableCELInWhenExpression: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := config.ToContext(t.Context(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableCELInWhenExpression: tt.enableCELInWhenExpression,
				},
			})
			if err := tt.wes.validate(ctx); err == nil {
				t.Errorf("WhenExpressions.validate() did not return error for invalid when expressions: %s", tt.wes)
			}
		})
	}
}
