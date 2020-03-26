/*
Copyright 2019 The Tekton Authors.

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

package v1alpha1_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/builder"
)

func TestParamSpec_SetDefaults(t *testing.T) {
	tests := []struct {
		name            string
		before          *v1alpha1.ParamSpec
		defaultsApplied *v1alpha1.ParamSpec
	}{{
		name: "inferred string type",
		before: &v1alpha1.ParamSpec{
			Name: "parametername",
		},
		defaultsApplied: &v1alpha1.ParamSpec{
			Name: "parametername",
			Type: v1alpha1.ParamTypeString,
		},
	}, {
		name: "inferred type from default value",
		before: &v1alpha1.ParamSpec{
			Name:    "parametername",
			Default: builder.ArrayOrString("an", "array"),
		},
		defaultsApplied: &v1alpha1.ParamSpec{
			Name:    "parametername",
			Type:    v1alpha1.ParamTypeArray,
			Default: builder.ArrayOrString("an", "array"),
		},
	}, {
		name: "fully defined ParamSpec",
		before: &v1alpha1.ParamSpec{
			Name:        "parametername",
			Type:        v1alpha1.ParamTypeArray,
			Description: "a description",
			Default:     builder.ArrayOrString("an", "array"),
		},
		defaultsApplied: &v1alpha1.ParamSpec{
			Name:        "parametername",
			Type:        v1alpha1.ParamTypeArray,
			Description: "a description",
			Default:     builder.ArrayOrString("an", "array"),
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tc.before.SetDefaults(ctx)
			if d := cmp.Diff(tc.before, tc.defaultsApplied); d != "" {
				t.Errorf("ParamSpec.SetDefaults/%s (-want, +got) = %v", tc.name, d)
			}
		})
	}
}

func TestArrayOrString_ApplyReplacements(t *testing.T) {
	type args struct {
		input              *v1alpha1.ArrayOrString
		stringReplacements map[string]string
		arrayReplacements  map[string][]string
	}
	tests := []struct {
		name           string
		args           args
		expectedOutput *v1alpha1.ArrayOrString
	}{{
		name: "no replacements on array",
		args: args{
			input:              builder.ArrayOrString("an", "array"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"sdf", "sdfsd"}},
		},
		expectedOutput: builder.ArrayOrString("an", "array"),
	}, {
		name: "string replacements on string",
		args: args{
			input:              builder.ArrayOrString("astring$(some) asdf $(anotherkey)"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: builder.ArrayOrString("astringvalue asdf value"),
	}, {
		name: "single array replacement",
		args: args{
			input:              builder.ArrayOrString("firstvalue", "$(arraykey)", "lastvalue"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: builder.ArrayOrString("firstvalue", "array", "value", "lastvalue"),
	}, {
		name: "multiple array replacement",
		args: args{
			input:              builder.ArrayOrString("firstvalue", "$(arraykey)", "lastvalue", "$(sdfdf)"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: builder.ArrayOrString("firstvalue", "array", "value", "lastvalue", "asdf", "sdfsd"),
	}, {
		name: "empty array replacement",
		args: args{
			input:              builder.ArrayOrString("firstvalue", "$(arraykey)", "lastvalue"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {}},
		},
		expectedOutput: builder.ArrayOrString("firstvalue", "lastvalue"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.input.ApplyReplacements(tt.args.stringReplacements, tt.args.arrayReplacements)
			if d := cmp.Diff(tt.expectedOutput, tt.args.input); d != "" {
				t.Errorf("ApplyReplacements() output did not match expected value %s", d)
			}
		})
	}
}

type ArrayOrStringHolder struct {
	AOrS v1alpha1.ArrayOrString `json:"val"`
}

func TestArrayOrString_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		input  string
		result v1alpha1.ArrayOrString
	}{
		{"{\"val\": \"123\"}", *builder.ArrayOrString("123")},
		{"{\"val\": \"\"}", *builder.ArrayOrString("")},
		{"{\"val\":[]}", v1alpha1.ArrayOrString{Type: v1alpha1.ParamTypeArray, ArrayVal: []string{}}},
		{"{\"val\":[\"oneelement\"]}", v1alpha1.ArrayOrString{Type: v1alpha1.ParamTypeArray, ArrayVal: []string{"oneelement"}}},
		{"{\"val\":[\"multiple\", \"elements\"]}", v1alpha1.ArrayOrString{Type: v1alpha1.ParamTypeArray, ArrayVal: []string{"multiple", "elements"}}},
	}

	for _, c := range cases {
		var result ArrayOrStringHolder
		if err := json.Unmarshal([]byte(c.input), &result); err != nil {
			t.Errorf("Failed to unmarshal input '%v': %v", c.input, err)
		}
		if !reflect.DeepEqual(result.AOrS, c.result) {
			t.Errorf("Failed to unmarshal input '%v': expected %+v, got %+v", c.input, c.result, result)
		}
	}
}

func TestArrayOrString_MarshalJSON(t *testing.T) {
	cases := []struct {
		input  v1alpha1.ArrayOrString
		result string
	}{
		{*builder.ArrayOrString("123"), "{\"val\":\"123\"}"},
		{*builder.ArrayOrString("123", "1234"), "{\"val\":[\"123\",\"1234\"]}"},
		{*builder.ArrayOrString("a", "a", "a"), "{\"val\":[\"a\",\"a\",\"a\"]}"},
	}

	for _, c := range cases {
		input := ArrayOrStringHolder{c.input}
		result, err := json.Marshal(&input)
		if err != nil {
			t.Errorf("Failed to marshal input '%v': %v", input, err)
		}
		if string(result) != c.result {
			t.Errorf("Failed to marshal input '%v': expected: %+v, got %q", input, c.result, string(result))
		}
	}
}
