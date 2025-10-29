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

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/selection"
)

func TestAllowsExecution(t *testing.T) {
	tests := []struct {
		name            string
		whenExpressions WhenExpressions
		evaluatedCEL    map[string]bool
		expected        bool
	}{{
		name: "in expression",
		whenExpressions: WhenExpressions{
			{
				Input:    "foo",
				Operator: selection.In,
				Values:   []string{"foo", "bar"},
			},
		},
		expected: true,
	}, {
		name: "notin expression",
		whenExpressions: WhenExpressions{
			{
				Input:    "foobar",
				Operator: selection.NotIn,
				Values:   []string{"foobar"},
			},
		},
		expected: false,
	}, {
		name: "multiple expressions - false",
		whenExpressions: WhenExpressions{
			{
				Input:    "foobar",
				Operator: selection.In,
				Values:   []string{"foobar"},
			}, {
				Input:    "foo",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
		expected: false,
	}, {
		name: "multiple expressions - true",
		whenExpressions: WhenExpressions{
			{
				Input:    "foobar",
				Operator: selection.In,
				Values:   []string{"foobar"},
			}, {
				Input:    "foo",
				Operator: selection.NotIn,
				Values:   []string{"bar"},
			},
		},
		expected: true,
	}, {
		name: "CEL is true",
		whenExpressions: WhenExpressions{
			{
				CEL: "'foo'=='foo'",
			},
		},
		evaluatedCEL: map[string]bool{"'foo'=='foo'": true},
		expected:     true,
	}, {
		name: "CEL is false",
		whenExpressions: WhenExpressions{
			{
				CEL: "'foo'!='foo'",
			},
		},
		evaluatedCEL: map[string]bool{"'foo'!='foo'": false},
		expected:     false,
	},
		{
			name: "multiple expressions - 1. CEL is true 2. In Op is false, expect false",
			whenExpressions: WhenExpressions{
				{
					CEL: "'foo'=='foo'",
				},
				{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"bar"},
				},
			},
			evaluatedCEL: map[string]bool{"'foo'=='foo'": true},
			expected:     false,
		},
		{
			name: "multiple expressions - 1. CEL is true 2. CEL is false, expect false",
			whenExpressions: WhenExpressions{
				{
					CEL: "'foo'!='foo'",
				},
				{
					CEL: "'xxx'!='xxx'",
				},
			},
			evaluatedCEL: map[string]bool{"'foo'=='foo'": true, "'xxx'!='xxx'": false},
			expected:     false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.whenExpressions.AllowsExecution(tc.evaluatedCEL)
			if d := cmp.Diff(tc.expected, got); d != "" {
				t.Errorf("Error evaluating AllowsExecution() for When Expressions in test case %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReplaceWhenExpressionsVariables(t *testing.T) {
	tests := []struct {
		name            string
		whenExpressions WhenExpressions
		replacements    map[string]string
		expected        WhenExpressions
	}{{
		name: "params replacement in input",
		whenExpressions: WhenExpressions{
			{
				Input:    "$(params.foo)",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
		replacements: map[string]string{
			"params.foo": "bar",
		},
		expected: WhenExpressions{
			{
				Input:    "bar",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
	}, {
		name: "params replacement in values",
		whenExpressions: WhenExpressions{
			{
				Input:    "bar",
				Operator: selection.In,
				Values:   []string{"$(params.foo)"},
			},
		},
		replacements: map[string]string{
			"params.foo": "bar",
		},
		expected: WhenExpressions{
			{
				Input:    "bar",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
	}, {
		name: "results replacement in input",
		whenExpressions: WhenExpressions{
			{
				Input:    "$(tasks.aTask.results.foo)",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
		replacements: map[string]string{
			"tasks.aTask.results.foo": "bar",
		},
		expected: WhenExpressions{
			{
				Input:    "bar",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
	}, {
		name: "results replacement in values",
		whenExpressions: WhenExpressions{
			{
				Input:    "bar",
				Operator: selection.In,
				Values:   []string{"$(tasks.aTask.results.foo)"},
			},
		},
		replacements: map[string]string{
			"tasks.aTask.results.foo": "bar",
		},
		expected: WhenExpressions{
			{
				Input:    "bar",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
	}, {
		name: "replacements in multiple when expressions",
		whenExpressions: WhenExpressions{
			{
				Input:    "foo",
				Operator: selection.In,
				Values:   []string{"$(tasks.aTask.results.foo)"},
			}, {
				Input:    "$(params.bar)",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
		replacements: map[string]string{
			"tasks.aTask.results.foo": "foo",
			"params.bar":              "bar",
		},
		expected: WhenExpressions{
			{
				Input:    "foo",
				Operator: selection.In,
				Values:   []string{"foo"},
			}, {
				Input:    "bar",
				Operator: selection.In,
				Values:   []string{"bar"},
			},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.whenExpressions.ReplaceVariables(tc.replacements, nil)
			if d := cmp.Diff(tc.expected, got); d != "" {
				t.Errorf("Error evaluating When Expressions in test case %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyReplacements(t *testing.T) {
	tests := []struct {
		name              string
		original          *WhenExpression
		replacements      map[string]string
		arrayReplacements map[string][]string
		expected          *WhenExpression
	}{{
		name: "replace parameters variables",
		original: &WhenExpression{
			Input:    "$(params.path)",
			Operator: selection.In,
			Values:   []string{"$(params.branch)"},
		},
		replacements: map[string]string{
			"params.path":   "readme.md",
			"params.branch": "staging",
		},
		expected: &WhenExpression{
			Input:    "readme.md",
			Operator: selection.In,
			Values:   []string{"staging"},
		},
	}, {
		name: "replace results variables",
		original: &WhenExpression{
			Input:    "$(tasks.foo.results.bar)",
			Operator: selection.In,
			Values:   []string{"$(tasks.aTask.results.aResult)"},
		},
		replacements: map[string]string{
			"tasks.foo.results.bar":       "foobar",
			"tasks.aTask.results.aResult": "barfoo",
		},
		expected: &WhenExpression{
			Input:    "foobar",
			Operator: selection.In,
			Values:   []string{"barfoo"},
		},
	}, {
		name: "replace array results variables",
		original: &WhenExpression{
			Input:    "$(tasks.foo.results.bar)",
			Operator: selection.In,
			Values:   []string{"$(tasks.aTask.results.aResult[*])"},
		},
		replacements: map[string]string{
			"tasks.foo.results.bar": "foobar",
		},
		arrayReplacements: map[string][]string{
			"tasks.aTask.results.aResult": {"dev", "stage"},
		},
		expected: &WhenExpression{
			Input:    "foobar",
			Operator: selection.In,
			Values:   []string{"dev", "stage"},
		},
	}, {
		name: "invaliad array results replacements",
		original: &WhenExpression{
			Input:    "$(tasks.foo.results.bar)",
			Operator: selection.In,
			Values:   []string{"$(tasks.aTask.results.aResult[invalid])"},
		},
		replacements: map[string]string{
			"tasks.foo.results.bar":          "foobar",
			"tasks.aTask.results.aResult[*]": "barfoo",
		},
		arrayReplacements: map[string][]string{
			"tasks.aTask.results.aResult[*]": {"dev", "stage"},
		},
		expected: &WhenExpression{
			Input:    "foobar",
			Operator: selection.In,
			Values:   []string{"$(tasks.aTask.results.aResult[invalid])"},
		},
	}, {
		name: "replace array params",
		original: &WhenExpression{
			Input:    "$(params.path)",
			Operator: selection.In,
			Values:   []string{"$(params.branches[*])"},
		},
		replacements: map[string]string{
			"params.path": "readme.md",
		},
		arrayReplacements: map[string][]string{
			"params.branches": {"dev", "stage"},
		},
		expected: &WhenExpression{
			Input:    "readme.md",
			Operator: selection.In,
			Values:   []string{"dev", "stage"},
		},
	}, {
		name: "replace string and array params",
		original: &WhenExpression{
			Input:    "$(params.path)",
			Operator: selection.In,
			Values:   []string{"$(params.branches[*])", "$(params.matchPath)", "$(params.files[*])"},
		},
		replacements: map[string]string{
			"params.path":      "readme.md",
			"params.matchPath": "foo.txt",
		},
		arrayReplacements: map[string][]string{
			"params.branches": {"dev", "stage"},
			"params.files":    {"readme.md", "test.go"},
		},
		expected: &WhenExpression{
			Input:    "readme.md",
			Operator: selection.In,
			Values:   []string{"dev", "stage", "foo.txt", "readme.md", "test.go"},
		},
	}, {
		name: "replace input with empty results array",
		original: &WhenExpression{
			Input:    "$(tasks.foo.results.bar[*])",
			Operator: selection.In,
			Values:   []string{"main"},
		},
		arrayReplacements: map[string][]string{
			"tasks.foo.results.bar": {},
		},
		expected: &WhenExpression{
			Input:    "[]",
			Operator: selection.In,
			Values:   []string{"main"},
		},
	}, {
		name: "replace input with non-empty results array",
		original: &WhenExpression{
			Input:    "$(tasks.foo.results.bar[*])",
			Operator: selection.In,
			Values:   []string{"main"},
		},
		arrayReplacements: map[string][]string{
			"tasks.foo.results.bar": {"main", "devel"},
		},
		expected: &WhenExpression{
			Input:    `["main","devel"]`,
			Operator: selection.In,
			Values:   []string{"main"},
		},
	},
		{
			name: "replace input with empty params array",
			original: &WhenExpression{
				Input:    "$(params.branches[*])",
				Operator: selection.In,
				Values:   []string{"main"},
			},
			arrayReplacements: map[string][]string{
				"params.branches": {},
			},
			expected: &WhenExpression{
				Input:    "[]",
				Operator: selection.In,
				Values:   []string{"main"},
			},
		},
		{
			name: "replace input with non-empty params array",
			original: &WhenExpression{
				Input:    "$(params.branches[*])",
				Operator: selection.In,
				Values:   []string{"main"},
			},
			arrayReplacements: map[string][]string{
				"params.branches": {"main", "devel"},
			},
			expected: &WhenExpression{
				Input:    `["main","devel"]`,
				Operator: selection.In,
				Values:   []string{"main"},
			},
		}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.original.applyReplacements(tc.replacements, tc.arrayReplacements)
			if d := cmp.Diff(tc.expected, &got); d != "" {
				t.Errorf("Error applying replacements for When Expressions: %s", diff.PrintWantGot(d))
			}
		})
	}
}
