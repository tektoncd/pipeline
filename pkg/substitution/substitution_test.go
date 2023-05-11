/*
Copyright 2019 The Tekton Authors
	"github.com/tektoncd/pipeline/pkg/substitution"

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

package substitution_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

func TestValidateNoReferencesToUnknownVariables(t *testing.T) {
	type args struct {
		input  string
		prefix string
		vars   sets.String
	}
	for _, tc := range []struct {
		name          string
		args          args
		expectedError *apis.FieldError
	}{{
		name: "valid variable",
		args: args{
			input:  "--flag=$(inputs.params.baz)",
			prefix: "inputs.params",
			vars:   sets.NewString("baz"),
		},
		expectedError: nil,
	}, {
		name: "valid variable with double quote bracket",
		args: args{
			input:  "--flag=$(inputs.params[\"baz\"])",
			prefix: "inputs.params",
			vars:   sets.NewString("baz"),
		},
		expectedError: nil,
	}, {
		name: "valid variable with single quotebracket",
		args: args{
			input:  "--flag=$(inputs.params['baz'])",
			prefix: "inputs.params",
			vars:   sets.NewString("baz"),
		},
		expectedError: nil,
	}, {
		name: "valid variable with double quote bracket and dots",
		args: args{
			input:  "--flag=$(params[\"foo.bar.baz\"])",
			prefix: "params",
			vars:   sets.NewString("foo.bar.baz"),
		},
		expectedError: nil,
	}, {
		name: "valid variable with single quote bracket and dots",
		args: args{
			input:  "--flag=$(params['foo.bar.baz'])",
			prefix: "params",
			vars:   sets.NewString("foo.bar.baz"),
		},
		expectedError: nil,
	}, {
		name: "invalid variable with only dots referencing parameters",
		args: args{
			input:  "--flag=$(params.foo.bar.baz)",
			prefix: "params",
			vars:   sets.NewString("foo.bar.baz"),
		},
		expectedError: &apis.FieldError{
			Message: fmt.Sprintf(`Invalid referencing of parameters in "%s"! Only two dot-separated components after the prefix "%s" are allowed.`, "--flag=$(params.foo.bar.baz)", "params"),
			Paths:   []string{""},
		},
	}, {
		name: "valid variable with dots referencing resources",
		args: args{
			input:  "--flag=$(resources.inputs.foo.bar)",
			prefix: "resources.(?:inputs|outputs)",
			vars:   sets.NewString("foo"),
		},
		expectedError: nil,
	}, {
		name: "invalid variable with dots referencing resources",
		args: args{
			input:  "--flag=$(resources.inputs.foo.bar.baz)",
			prefix: "resources.(?:inputs|outputs)",
			vars:   sets.NewString("foo.bar"),
		},
		expectedError: &apis.FieldError{
			Message: fmt.Sprintf(`Invalid referencing of parameters in "%s"! Only two dot-separated components after the prefix "%s" are allowed.`, "--flag=$(resources.inputs.foo.bar.baz)", "resources.(?:inputs|outputs)"),
			Paths:   []string{""},
		},
	}, {
		name: "valid variable contains diffetent chars",
		args: args{
			input:  "--flag=$(inputs.params['ba-_9z'])",
			prefix: "inputs.params",
			vars:   sets.NewString("ba-_9z"),
		},
		expectedError: nil,
	}, {
		name: "valid variable uid",
		args: args{
			input:  "--flag=$(context.taskRun.uid)",
			prefix: "context.taskRun",
			vars:   sets.NewString("uid"),
		},
		expectedError: nil,
	}, {
		name: "multiple variables",
		args: args{
			input:  "--flag=$(inputs.params.baz) $(inputs.params.foo)",
			prefix: "inputs.params",
			vars:   sets.NewString("baz", "foo"),
		},
		expectedError: nil,
	}, {
		name: "valid usage of an individual attribute of an object param",
		args: args{
			input:  "--flag=$(params.objectParam.key1)",
			prefix: "params.objectParam",
			vars:   sets.NewString("key1", "key2"),
		},
		expectedError: nil,
	}, {
		name: "valid usage of multiple individual attributes of an object param",
		args: args{
			input:  "--flag=$(params.objectParam.key1) $(params.objectParam.key2)",
			prefix: "params.objectParam",
			vars:   sets.NewString("key1", "key2"),
		},
		expectedError: nil,
	}, {
		name: "different context and prefix",
		args: args{
			input:  "--flag=$(something.baz)",
			prefix: "something",
			vars:   sets.NewString("baz"),
		},
		expectedError: nil,
	}, {
		name: "undefined variable",
		args: args{
			input:  "--flag=$(inputs.params.baz)",
			prefix: "inputs.params",
			vars:   sets.NewString("foo"),
		},
		expectedError: &apis.FieldError{
			Message: `non-existent variable in "--flag=$(inputs.params.baz)"`,
			Paths:   []string{""},
		},
	}, {
		name: "undefined individual attributes of an object param",
		args: args{
			input:  "--flag=$(params.objectParam.key3)",
			prefix: "params.objectParam",
			vars:   sets.NewString("key1", "key2"),
		},
		expectedError: &apis.FieldError{
			Message: `non-existent variable in "--flag=$(params.objectParam.key3)"`,
			Paths:   []string{""},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := substitution.ValidateNoReferencesToUnknownVariables(tc.args.input, tc.args.prefix, tc.args.vars)

			if d := cmp.Diff(got, tc.expectedError, cmp.AllowUnexported(apis.FieldError{})); d != "" {
				t.Errorf("ValidateVariableP() error did not match expected error %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidateNoReferencesToProhibitedVariables(t *testing.T) {
	type args struct {
		input  string
		prefix string
		vars   sets.String
	}
	for _, tc := range []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name: "empty vars",
		args: args{
			input:  "--flag=$(params.foo)",
			prefix: "params",
			vars:   sets.NewString(),
		},
		wantErr: false,
	}, {
		name: "doesn't contain reference to a param",
		args: args{
			input:  "--flag=$(params.foo)",
			prefix: "params",
			vars:   sets.NewString("bar"),
		},
		wantErr: false,
	}, {
		name: "contains reference to a param",
		args: args{
			input:  "--flag=$(params.foo)",
			prefix: "params",
			vars:   sets.NewString("foo"),
		},
		wantErr: true,
	}, {
		name: "contains reference to an entire param",
		args: args{
			input:  "--flag=$(params.foo[*])",
			prefix: "params",
			vars:   sets.NewString("foo"),
		},
		wantErr: true,
	}, {
		name: "contains reference to an array param index",
		args: args{
			input:  "--flag=$(params.arrayParam[1])",
			prefix: "params",
			vars:   sets.NewString("arrayParam"),
		},
		wantErr: false,
	}, {
		name: "contains reference to an object param key",
		args: args{
			input:  "--flag=$(params.objectParam.key1)",
			prefix: "params",
			vars:   sets.NewString("objectParam"),
		},
		wantErr: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := substitution.ValidateNoReferencesToProhibitedVariables(tc.args.input, tc.args.prefix, tc.args.vars)
			if (gotErr != nil) != tc.wantErr {
				t.Errorf("wantErr was %t but got err %v", tc.wantErr, gotErr)
			}
		})
	}
}

func TestValidateValidateNoReferencesToEntireProhibitedVariables(t *testing.T) {
	type args struct {
		input  string
		prefix string
		vars   sets.String
	}
	for _, tc := range []struct {
		name          string
		args          args
		expectedError *apis.FieldError
	}{{
		name: "valid usage of an individual key of an object param",
		args: args{
			input:  "--flag=$(params.objectParam.key1)",
			prefix: "params",
			vars:   sets.NewString("objectParam"),
		},
		expectedError: nil,
	}, {
		name: "invalid regex",
		args: args{
			input:  "--flag=$(params.objectParam.key1)",
			prefix: `???`,
			vars:   sets.NewString("objectParam"),
		},
		expectedError: &apis.FieldError{
			Message: "extractEntireVariablesFromString failed : failed to parse regex pattern: error parsing regexp: invalid nested repetition operator: `???`",
			Paths:   []string{""},
		},
	}, {
		name: "invalid usage of an entire object param when providing values for strings",
		args: args{
			input:  "--flag=$(params.objectParam)",
			prefix: "params",
			vars:   sets.NewString("objectParam"),
		},
		expectedError: &apis.FieldError{
			Message: `variable type invalid in "--flag=$(params.objectParam)"`,
			Paths:   []string{""},
		},
	}, {
		name: "invalid usage of an entire object param using [*] when providing values for strings",
		args: args{
			input:  "--flag=$(params.objectParam[*])",
			prefix: "params",
			vars:   sets.NewString("objectParam"),
		},
		expectedError: &apis.FieldError{
			Message: `variable type invalid in "--flag=$(params.objectParam[*])"`,
			Paths:   []string{""},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := substitution.ValidateNoReferencesToEntireProhibitedVariables(tc.args.input, tc.args.prefix, tc.args.vars)

			if d := cmp.Diff(got, tc.expectedError, cmp.AllowUnexported(apis.FieldError{})); d != "" {
				t.Errorf("ValidateEntireVariableProhibitedP() error did not match expected error %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidateVariableReferenceIsIsolated(t *testing.T) {
	type args struct {
		input  string
		prefix string
		vars   sets.String
	}
	for _, tc := range []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name: "empty vars",
		args: args{
			input:  "--flag=$(params.foo)",
			prefix: "params",
			vars:   sets.NewString(),
		},
		wantErr: false,
	}, {
		name: "isolated variable",
		args: args{
			input:  "$(params.foo)",
			prefix: "params",
			vars:   sets.NewString("foo"),
		},
		wantErr: false,
	}, {
		name: "extra characters before",
		args: args{
			input:  "--flag=$(params.foo)",
			prefix: "params",
			vars:   sets.NewString("foo"),
		},
		wantErr: true,
	}, {
		name: "extra characters after",
		args: args{
			input:  "$(params.foo)-12345",
			prefix: "params",
			vars:   sets.NewString("foo"),
		},
		wantErr: true,
	}, {
		name: "isolated variable with array index",
		args: args{
			input:  "$(params.foo[1])",
			prefix: "params",
			vars:   sets.NewString("foo"),
		},
		wantErr: false,
	}, {
		name: "isolated variable with entire array",
		args: args{
			input:  "$(params.foo[*])",
			prefix: "params",
			vars:   sets.NewString("foo"),
		},
		wantErr: false,
	}, {
		name: "unknown variable isolated",
		args: args{
			input:  "$(params.foo)",
			prefix: "params",
			vars:   sets.NewString("bar"),
		},
		wantErr: false,
	}, {
		name: "unknown variable not isolated",
		args: args{
			input:  "1234-$(params.foo)",
			prefix: "params",
			vars:   sets.NewString("bar"),
		},
		wantErr: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := substitution.ValidateVariableReferenceIsIsolated(tc.args.input, tc.args.prefix, tc.args.vars)
			if (got != nil) != tc.wantErr {
				t.Errorf("wantErr was %t but got err %s", tc.wantErr, got)
			}
		})
	}
}

func TestApplyReplacements(t *testing.T) {
	type args struct {
		input        string
		replacements map[string]string
	}
	tests := []struct {
		name           string
		args           args
		expectedOutput string
	}{
		{
			name: "no replacements requested",
			args: args{
				input:        "this is a string",
				replacements: map[string]string{},
			},
			expectedOutput: "this is a string",
		},
		{
			name: "single replacement requested",
			args: args{
				input:        "this is $(a) string",
				replacements: map[string]string{"a": "not a"},
			},
			expectedOutput: "this is not a string",
		},
		{
			name: "single replacement requested multiple matches",
			args: args{
				input:        "this $(is) a string $(is) a string",
				replacements: map[string]string{"is": "foo"},
			},
			expectedOutput: "this foo a string foo a string",
		},
		{
			name: "multiple replacements requested",
			args: args{
				input:        "this $(is) a $(string) $(is) a $(string)",
				replacements: map[string]string{"is": "foo", "string": "sstring"},
			},
			expectedOutput: "this foo a sstring foo a sstring",
		},
		{
			name: "multiple replacements requested nothing replaced",
			args: args{
				input:        "this is a string",
				replacements: map[string]string{"this": "a", "evxasdd": "string"},
			},
			expectedOutput: "this is a string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualOutput := substitution.ApplyReplacements(tt.args.input, tt.args.replacements)
			if d := cmp.Diff(actualOutput, tt.expectedOutput); d != "" {
				t.Errorf("ApplyReplacements() output did not match expected value %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestNestedReplacements(t *testing.T) {
	replacements := map[string]string{
		// Foo should turn into barbar, which could then expand into bazbaz depending on how this is expanded
		"foo": "$(bar)$(bar)",
		"bar": "baz",
	}
	input := "$(foo) is cool"
	expected := "$(bar)$(bar) is cool"

	// Run this test a lot of times to ensure the behavior is deterministic
	for i := 0; i <= 1000; i++ {
		got := substitution.ApplyReplacements(input, replacements)
		if d := cmp.Diff(expected, got); d != "" {
			t.Errorf("ApplyReplacements() output did not match expected value %s", diff.PrintWantGot(d))
		}
	}
}

func TestApplyArrayReplacements(t *testing.T) {
	type args struct {
		input              string
		stringReplacements map[string]string
		arrayReplacements  map[string][]string
	}
	for _, tc := range []struct {
		name           string
		args           args
		expectedOutput []string
	}{{
		name: "no replacements requested",
		args: args{
			input:              "this is a string",
			stringReplacements: map[string]string{},
			arrayReplacements:  map[string][]string{},
		},
		expectedOutput: []string{"this is a string"},
	}, {
		name: "multiple replacements requested nothing replaced",
		args: args{
			input:              "this is a string",
			stringReplacements: map[string]string{"key": "replacement", "anotherkey": "foo"},
			arrayReplacements:  map[string][]string{"key2": {"replacement", "a"}, "key3": {"1", "2"}},
		},
		expectedOutput: []string{"this is a string"},
	}, {
		name: "multiple replacements only string replacement possible",
		args: args{
			input:              "$(string)rep$(lacement)$(string)",
			stringReplacements: map[string]string{"string": "word", "lacement": "lacements"},
			arrayReplacements:  map[string][]string{"ace": {"replacement", "a"}, "string": {"1", "2"}},
		},
		expectedOutput: []string{"wordreplacementsword"},
	}, {
		name: "array replacement",
		args: args{
			input:              "$(match)",
			stringReplacements: map[string]string{"string": "word", "lacement": "lacements"},
			arrayReplacements:  map[string][]string{"ace": {"replacement", "a"}, "match": {"1", "2"}},
		},
		expectedOutput: []string{"1", "2"},
	}, {
		name: "array star replacement",
		args: args{
			input:              "$(match[*])",
			stringReplacements: map[string]string{"string": "word", "lacement": "lacements"},
			arrayReplacements:  map[string][]string{"ace": {"replacement", "a"}, "match": {"1", "2"}},
		},
		expectedOutput: []string{"1", "2"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := substitution.ApplyArrayReplacements(tc.args.input, tc.args.stringReplacements, tc.args.arrayReplacements)
			if d := cmp.Diff(actualOutput, tc.expectedOutput); d != "" {
				t.Errorf("ApplyArrayReplacements() output did not match expected value %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestExtractParamsExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{{
		name:  "normal string",
		input: "hello world",
		want:  nil,
	}, {
		name:  "param reference",
		input: "$(params.paramName)",
		want:  nil,
	}, {
		name:  "param star reference",
		input: "$(params.paramName[*])",
		want:  nil,
	}, {
		name:  "param index reference",
		input: "$(params.paramName[1])",
		want:  []string{"$(params.paramName[1])"},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substitution.ExtractParamsExpressions(tt.input)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestExtractIntIndex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{{
		name:  "normal string",
		input: "hello world",
		want:  "",
	}, {
		name:  "param reference",
		input: "$(params.paramName)",
		want:  "",
	}, {
		name:  "param star reference",
		input: "$(params.paramName[*])",
		want:  "",
	}, {
		name:  "param index reference",
		input: "$(params.paramName[1])",
		want:  "[1]",
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substitution.ExtractIndexString(tt.input)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestTrimSquareBrackets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{{
		name:  "normal string",
		input: "hello world",
		want:  0,
	}, {
		name:  "star in square bracket",
		input: "[*]",
		want:  0,
	}, {
		name:  "index in square bracket",
		input: "[1]",
		want:  1,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := substitution.ExtractIndex(tt.input)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestStripStarVarSubExpression(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{{
		name:  "normal string",
		input: "hello world",
		want:  "hello world",
	}, {
		name:  "result reference",
		input: "$(tasks.task.results.result)",
		want:  "tasks.task.results.result",
	}, {
		name:  "result star reference",
		input: "$(tasks.task.results.result[*])",
		want:  "tasks.task.results.result",
	}, {
		name:  "result index reference",
		input: "$(tasks.task.results.result[1])",
		want:  "tasks.task.results.result[1]",
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substitution.StripStarVarSubExpression(tt.input)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestExtractVariablesFromString(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		prefix    string
		want      []string
		extracted bool
		err       string
	}{{
		name:      "complete match",
		s:         "--flag=$(inputs.params.baz)",
		prefix:    "inputs.params",
		want:      []string{"baz"},
		extracted: true,
		err:       "",
	}, {
		name:      "no match",
		s:         "--flag=$(inputs.params.baz)",
		prefix:    "outputs",
		want:      []string{},
		extracted: false,
		err:       "",
	}, {
		name:      "too many dots",
		s:         "--flag=$(inputs.params.foo.baz.bar)",
		prefix:    "inputs.params",
		want:      []string{""},
		extracted: true,
		err:       `Invalid referencing of parameters in "--flag=$(inputs.params.foo.baz.bar)"! Only two dot-separated components after the prefix "inputs.params" are allowed.`,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, extracted, err := substitution.ExtractVariablesFromString(tt.s, tt.prefix)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tt.extracted, extracted); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tt.err, err); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
