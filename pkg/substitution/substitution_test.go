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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

func TestValidateVariables(t *testing.T) {
	type args struct {
		input        string
		prefix       string
		locationName string
		path         string
		vars         sets.String
	}
	for _, tc := range []struct {
		name          string
		args          args
		expectedError *apis.FieldError
	}{{
		name: "valid variable",
		args: args{
			input:        "--flag=$(inputs.params.baz)",
			prefix:       "inputs.params",
			locationName: "step",
			path:         "taskspec.steps",
			vars:         sets.NewString("baz"),
		},
		expectedError: nil,
	}, {
		name: "valid variable uid",
		args: args{
			input:        "--flag=$(context.taskRun.uid)",
			prefix:       "context.taskRun",
			locationName: "step",
			path:         "taskspec.steps",
			vars:         sets.NewString("uid"),
		},
		expectedError: nil,
	}, {
		name: "multiple variables",
		args: args{
			input:        "--flag=$(inputs.params.baz) $(input.params.foo)",
			prefix:       "inputs.params",
			locationName: "step",
			path:         "taskspec.steps",
			vars:         sets.NewString("baz", "foo"),
		},
		expectedError: nil,
	}, {
		name: "different context and prefix",
		args: args{
			input:        "--flag=$(something.baz)",
			prefix:       "something",
			locationName: "step",
			path:         "taskspec.steps",
			vars:         sets.NewString("baz"),
		},
		expectedError: nil,
	}, {
		name: "undefined variable",
		args: args{
			input:        "--flag=$(inputs.params.baz)",
			prefix:       "inputs.params",
			locationName: "step",
			path:         "taskspec.steps",
			vars:         sets.NewString("foo"),
		},
		expectedError: &apis.FieldError{
			Message: `non-existent variable in "--flag=$(inputs.params.baz)" for step somefield`,
			Paths:   []string{"taskspec.steps.somefield"},
		},
	}, {
		name: "undefined variable and defined variable",
		args: args{
			input:        "--flag=$(inputs.params.baz) $(input.params.foo)",
			prefix:       "inputs.params",
			locationName: "step",
			path:         "taskspec.steps",
			vars:         sets.NewString("foo"),
		},
		expectedError: &apis.FieldError{
			Message: `non-existent variable in "--flag=$(inputs.params.baz) $(input.params.foo)" for step somefield`,
			Paths:   []string{"taskspec.steps.somefield"},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := substitution.ValidateVariable("somefield", tc.args.input, tc.args.prefix, tc.args.locationName, tc.args.path, tc.args.vars)

			if d := cmp.Diff(got, tc.expectedError, cmp.AllowUnexported(apis.FieldError{})); d != "" {
				t.Errorf("ValidateVariable() error did not match expected error %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidateVariableP(t *testing.T) {
	type args struct {
		input  string
		prefix string
		vars   sets.String
	}
	for _, tc := range []struct {
		name          string
		args          args
		expectedError *apis.FieldError
	}{
		{
			name: "valid input with double quotes",
			args: args{
				input:  "echo {\"key\": \"$(params.a)\"}",
				prefix: "params",
				vars:   sets.NewString(),
			},
			expectedError: nil,
		},
		{
			name: "valid input with single quotes",
			args: args{
				input:  "echo {'key': '$(params.a)'}",
				prefix: "params",
				vars:   sets.NewString(),
			},
			expectedError: nil,
		},
		{
			name: "expected non-existent variable",
			args: args{
				input:  "echo {\"key\": $(params.a)}",
				prefix: "params",
				vars:   sets.NewString(),
			},
			expectedError: &apis.FieldError{
				Message: `non-existent variable "a" in "echo {\"key\": $(params.a)}"`,
				Paths:   []string{""},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := substitution.ValidateVariableP(tc.args.input, tc.args.prefix, tc.args.vars)

			if d := cmp.Diff(got, tc.expectedError, cmp.AllowUnexported(apis.FieldError{})); d != "" {
				t.Errorf("ValidateVariableP() error did not match expected error %s", diff.PrintWantGot(d))
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
				replacements: map[string]string{"is": "a"},
			},
			expectedOutput: "this a a string a a string",
		},
		{
			name: "multiple replacements requested",
			args: args{
				input:        "this $(is) a $(string) $(is) a $(string)",
				replacements: map[string]string{"is": "a", "string": "sstring"},
			},
			expectedOutput: "this a a sstring a a sstring",
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
