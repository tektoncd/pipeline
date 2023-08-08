/*
Copyright 2022 The Tekton Authors.

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

package internalversion_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/internalversion"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestParamSpec_SetDefaults(t *testing.T) {
	tests := []struct {
		name            string
		before          *internalversion.ParamSpec
		defaultsApplied *internalversion.ParamSpec
	}{{
		name: "inferred string type",
		before: &internalversion.ParamSpec{
			Name: "parametername",
		},
		defaultsApplied: &internalversion.ParamSpec{
			Name: "parametername",
			Type: internalversion.ParamTypeString,
		},
	}, {
		name: "inferred type from default value - array",
		before: &internalversion.ParamSpec{
			Name: "parametername",
			Default: &internalversion.ParamValue{
				ArrayVal: []string{"array"},
			},
		},
		defaultsApplied: &internalversion.ParamSpec{
			Name: "parametername",
			Type: internalversion.ParamTypeArray,
			Default: &internalversion.ParamValue{
				ArrayVal: []string{"array"},
			},
		},
	}, {
		name: "inferred type from default value - string",
		before: &internalversion.ParamSpec{
			Name: "parametername",
			Default: &internalversion.ParamValue{
				StringVal: "an",
			},
		},
		defaultsApplied: &internalversion.ParamSpec{
			Name: "parametername",
			Type: internalversion.ParamTypeString,
			Default: &internalversion.ParamValue{
				StringVal: "an",
			},
		},
	}, {
		name: "inferred type from default value - object",
		before: &internalversion.ParamSpec{
			Name: "parametername",
			Default: &internalversion.ParamValue{
				ObjectVal: map[string]string{"url": "test", "path": "test"},
			},
		},
		defaultsApplied: &internalversion.ParamSpec{
			Name: "parametername",
			Type: internalversion.ParamTypeObject,
			Default: &internalversion.ParamValue{
				ObjectVal: map[string]string{"url": "test", "path": "test"},
			},
		},
	}, {
		name: "inferred type from properties - PropertySpec type is not provided",
		before: &internalversion.ParamSpec{
			Name:       "parametername",
			Properties: map[string]internalversion.PropertySpec{"key1": {}},
		},
		defaultsApplied: &internalversion.ParamSpec{
			Name:       "parametername",
			Type:       internalversion.ParamTypeObject,
			Properties: map[string]internalversion.PropertySpec{"key1": {Type: "string"}},
		},
	}, {
		name: "inferred type from properties - PropertySpec type is provided",
		before: &internalversion.ParamSpec{
			Name:       "parametername",
			Properties: map[string]internalversion.PropertySpec{"key2": {Type: "string"}},
		},
		defaultsApplied: &internalversion.ParamSpec{
			Name:       "parametername",
			Type:       internalversion.ParamTypeObject,
			Properties: map[string]internalversion.PropertySpec{"key2": {Type: "string"}},
		},
	}, {
		name: "fully defined ParamSpec - array",
		before: &internalversion.ParamSpec{
			Name:        "parametername",
			Type:        internalversion.ParamTypeArray,
			Description: "a description",
			Default: &internalversion.ParamValue{
				ArrayVal: []string{"array"},
			},
		},
		defaultsApplied: &internalversion.ParamSpec{
			Name:        "parametername",
			Type:        internalversion.ParamTypeArray,
			Description: "a description",
			Default: &internalversion.ParamValue{
				ArrayVal: []string{"array"},
			},
		},
	}, {
		name: "fully defined ParamSpec - object",
		before: &internalversion.ParamSpec{
			Name:        "parametername",
			Type:        internalversion.ParamTypeObject,
			Description: "a description",
			Default: &internalversion.ParamValue{
				ObjectVal: map[string]string{"url": "test", "path": "test"},
			},
		},
		defaultsApplied: &internalversion.ParamSpec{
			Name:        "parametername",
			Type:        internalversion.ParamTypeObject,
			Description: "a description",
			Default: &internalversion.ParamValue{
				ObjectVal: map[string]string{"url": "test", "path": "test"},
			},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tc.before.SetDefaults(ctx)
			if d := cmp.Diff(tc.before, tc.defaultsApplied); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestParamValues_ApplyReplacements(t *testing.T) {
	type args struct {
		input              *internalversion.ParamValue
		stringReplacements map[string]string
		arrayReplacements  map[string][]string
		objectReplacements map[string]map[string]string
	}
	tests := []struct {
		name           string
		args           args
		expectedOutput *internalversion.ParamValue
	}{{
		name: "no replacements on array",
		args: args{
			input:              internalversion.NewStructuredValues("an", "array"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"sdf", "sdfsd"}},
		},
		expectedOutput: internalversion.NewStructuredValues("an", "array"),
	}, {
		name: "single string replacement on string",
		args: args{
			input:              internalversion.NewStructuredValues("$(params.myString1)"),
			stringReplacements: map[string]string{"params.myString1": "value1", "params.myString2": "value2"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: internalversion.NewStructuredValues("value1"),
	}, {
		name: "multiple string replacements on string",
		args: args{
			input:              internalversion.NewStructuredValues("astring$(some) asdf $(anotherkey)"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: internalversion.NewStructuredValues("astringvalue asdf value"),
	}, {
		name: "single array replacement",
		args: args{
			input:              internalversion.NewStructuredValues("firstvalue", "$(arraykey)", "lastvalue"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: internalversion.NewStructuredValues("firstvalue", "array", "value", "lastvalue"),
	}, {
		name: "multiple array replacement",
		args: args{
			input:              internalversion.NewStructuredValues("firstvalue", "$(arraykey)", "lastvalue", "$(sdfdf)"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: internalversion.NewStructuredValues("firstvalue", "array", "value", "lastvalue", "asdf", "sdfsd"),
	}, {
		name: "empty array replacement without extra elements",
		args: args{
			input:             internalversion.NewStructuredValues("$(arraykey)"),
			arrayReplacements: map[string][]string{"arraykey": {}},
		},
		expectedOutput: &internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{}},
	}, {
		name: "empty array replacement with extra elements",
		args: args{
			input:              internalversion.NewStructuredValues("firstvalue", "$(arraykey)", "lastvalue"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {}},
		},
		expectedOutput: internalversion.NewStructuredValues("firstvalue", "lastvalue"),
	}, {
		name: "array replacement on string val",
		args: args{
			input:             internalversion.NewStructuredValues("$(params.myarray)"),
			arrayReplacements: map[string][]string{"params.myarray": {"a", "b", "c"}},
		},
		expectedOutput: internalversion.NewStructuredValues("a", "b", "c"),
	}, {
		name: "array star replacement on string val",
		args: args{
			input:             internalversion.NewStructuredValues("$(params.myarray[*])"),
			arrayReplacements: map[string][]string{"params.myarray": {"a", "b", "c"}},
		},
		expectedOutput: internalversion.NewStructuredValues("a", "b", "c"),
	}, {
		name: "array indexing replacement on string val",
		args: args{
			input:              internalversion.NewStructuredValues("$(params.myarray[0])"),
			stringReplacements: map[string]string{"params.myarray[0]": "a", "params.myarray[1]": "b"},
		},
		expectedOutput: internalversion.NewStructuredValues("a"),
	}, {
		name: "object replacement on string val",
		args: args{
			input: internalversion.NewStructuredValues("$(params.object)"),
			objectReplacements: map[string]map[string]string{
				"params.object": {
					"url":    "abc.com",
					"commit": "af234",
				},
			},
		},
		expectedOutput: internalversion.NewObject(map[string]string{
			"url":    "abc.com",
			"commit": "af234",
		}),
	}, {
		name: "object star replacement on string val",
		args: args{
			input: internalversion.NewStructuredValues("$(params.object[*])"),
			objectReplacements: map[string]map[string]string{
				"params.object": {
					"url":    "abc.com",
					"commit": "af234",
				},
			},
		},
		expectedOutput: internalversion.NewObject(map[string]string{
			"url":    "abc.com",
			"commit": "af234",
		}),
	}, {
		name: "string replacement on object individual variables",
		args: args{
			input: internalversion.NewObject(map[string]string{
				"key1": "$(mystring)",
				"key2": "$(anotherObject.key)",
			}),
			stringReplacements: map[string]string{
				"mystring":          "foo",
				"anotherObject.key": "bar",
			},
		},
		expectedOutput: internalversion.NewObject(map[string]string{
			"key1": "foo",
			"key2": "bar",
		}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.input.ApplyReplacements(tt.args.stringReplacements, tt.args.arrayReplacements, tt.args.objectReplacements)
			if d := cmp.Diff(tt.expectedOutput, tt.args.input); d != "" {
				t.Errorf("ApplyReplacements() output did not match expected value %s", diff.PrintWantGot(d))
			}
		})
	}
}

type ParamValuesHolder struct {
	AOrS internalversion.ParamValue `json:"val"`
}

func TestParamValues_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		input  map[string]interface{}
		result internalversion.ParamValue
	}{
		{
			input:  map[string]interface{}{"val": 123},
			result: *internalversion.NewStructuredValues("123"),
		},
		{
			input:  map[string]interface{}{"val": "123"},
			result: *internalversion.NewStructuredValues("123"),
		},
		{
			input:  map[string]interface{}{"val": ""},
			result: *internalversion.NewStructuredValues(""),
		},
		{
			input:  map[string]interface{}{"val": nil},
			result: internalversion.ParamValue{Type: internalversion.ParamTypeString, ArrayVal: nil},
		},
		{
			input:  map[string]interface{}{"val": []string{}},
			result: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{}},
		},
		{
			input:  map[string]interface{}{"val": []string{"oneelement"}},
			result: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"oneelement"}},
		},
		{
			input:  map[string]interface{}{"val": []string{"multiple", "elements"}},
			result: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"multiple", "elements"}},
		},
		{
			input:  map[string]interface{}{"val": map[string]string{"key1": "val1", "key2": "val2"}},
			result: internalversion.ParamValue{Type: internalversion.ParamTypeObject, ObjectVal: map[string]string{"key1": "val1", "key2": "val2"}},
		},
	}

	for _, c := range cases {
		for _, opts := range []func(enc *json.Encoder){
			// Default encoding
			func(enc *json.Encoder) {},
			// Multiline encoding
			func(enc *json.Encoder) { enc.SetIndent("", "  ") },
		} {
			b := new(bytes.Buffer)
			enc := json.NewEncoder(b)
			opts(enc)
			if err := enc.Encode(c.input); err != nil {
				t.Fatalf("error encoding json: %v", err)
			}

			var result ParamValuesHolder
			if err := json.Unmarshal(b.Bytes(), &result); err != nil {
				t.Errorf("Failed to unmarshal input '%v': %v", c.input, err)
			}
			if !reflect.DeepEqual(result.AOrS, c.result) {
				t.Errorf("expected %+v, got %+v", c.result, result)
			}
		}
	}
}

func TestParamValues_UnmarshalJSON_Directly(t *testing.T) {
	cases := []struct {
		desc     string
		input    string
		expected internalversion.ParamValue
	}{
		{desc: "empty value", input: ``, expected: *internalversion.NewStructuredValues("")},
		{desc: "int value", input: `1`, expected: *internalversion.NewStructuredValues("1")},
		{desc: "int array", input: `[1,2,3]`, expected: *internalversion.NewStructuredValues("[1,2,3]")},
		{desc: "nested array", input: `[1,\"2\",3]`, expected: *internalversion.NewStructuredValues(`[1,\"2\",3]`)},
		{desc: "string value", input: `hello`, expected: *internalversion.NewStructuredValues("hello")},
		{desc: "array value", input: `["hello","world"]`, expected: *internalversion.NewStructuredValues("hello", "world")},
		{desc: "object value", input: `{"hello":"world"}`, expected: *internalversion.NewObject(map[string]string{"hello": "world"})},
	}

	for _, c := range cases {
		v := internalversion.ParamValue{}
		if err := v.UnmarshalJSON([]byte(c.input)); err != nil {
			t.Errorf("Failed to unmarshal input '%v': %v", c.input, err)
		}
		if !reflect.DeepEqual(v, c.expected) {
			t.Errorf("Failed to unmarshal input '%v': expected %+v, got %+v", c.input, c.expected, v)
		}
	}
}

func TestParamValues_UnmarshalJSON_Error(t *testing.T) {
	cases := []struct {
		desc  string
		input string
	}{
		{desc: "empty value", input: "{\"val\": }"},
		{desc: "wrong beginning value", input: "{\"val\": @}"},
	}

	for _, c := range cases {
		var result ParamValuesHolder
		if err := json.Unmarshal([]byte(c.input), &result); err == nil {
			t.Errorf("Should return err but got nil '%v'", c.input)
		}
	}
}

func TestParamValues_MarshalJSON(t *testing.T) {
	cases := []struct {
		input  internalversion.ParamValue
		result string
	}{
		{*internalversion.NewStructuredValues("123"), "{\"val\":\"123\"}"},
		{*internalversion.NewStructuredValues("123", "1234"), "{\"val\":[\"123\",\"1234\"]}"},
		{*internalversion.NewStructuredValues("a", "a", "a"), "{\"val\":[\"a\",\"a\",\"a\"]}"},
		{*internalversion.NewObject(map[string]string{"key1": "var1", "key2": "var2"}), "{\"val\":{\"key1\":\"var1\",\"key2\":\"var2\"}}"},
	}

	for _, c := range cases {
		input := ParamValuesHolder{c.input}
		result, err := json.Marshal(&input)
		if err != nil {
			t.Errorf("Failed to marshal input '%v': %v", input, err)
		}
		if string(result) != c.result {
			t.Errorf("Failed to marshal input '%v': expected: %+v, got %q", input, c.result, string(result))
		}
	}
}

func TestArrayReference(t *testing.T) {
	tests := []struct {
		name, p, expectedResult string
	}{{
		name:           "valid array parameter expression with star notation returns param name",
		p:              "$(params.arrayParam[*])",
		expectedResult: "arrayParam",
	}, {
		name:           "invalid array parameter without dollar notation returns the input as is",
		p:              "params.arrayParam[*]",
		expectedResult: "params.arrayParam[*]",
	}}
	for _, tt := range tests {
		if d := cmp.Diff(tt.expectedResult, internalversion.ArrayReference(tt.p)); d != "" {
			t.Errorf(diff.PrintWantGot(d))
		}
	}
}

func TestExtractNames(t *testing.T) {
	tests := []struct {
		name   string
		params internalversion.Params
		want   sets.String
	}{{
		name:   "no params",
		params: internalversion.Params{{}},
		want:   sets.NewString(""),
	}, {
		name: "extract param names from ParamTypeString",
		params: internalversion.Params{{
			Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
		}, {
			Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
		want: sets.NewString("IMAGE", "DOCKERFILE"),
	}, {
		name: "extract param names from ParamTypeArray",
		params: internalversion.Params{{
			Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
		}},
		want: sets.NewString("GOARCH"),
	}, {
		name: "extract param names from ParamTypeString and ParamTypeArray",
		params: internalversion.Params{{
			Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
		}, {
			Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
		}},
		want: sets.NewString("GOARCH", "IMAGE"),
	}, {
		name: "extract param name from duplicate params",
		params: internalversion.Params{{
			Name: "duplicate", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
		}, {
			Name: "duplicate", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
		}},
		want: sets.NewString("duplicate"),
	}}
	for _, tt := range tests {
		if d := cmp.Diff(tt.want, internalversion.Params.ExtractNames(tt.params)); d != "" {
			t.Errorf(diff.PrintWantGot(d))
		}
	}
}

func TestParams_ReplaceVariables(t *testing.T) {
	tests := []struct {
		name               string
		ps                 internalversion.Params
		stringReplacements map[string]string
		arrayReplacements  map[string][]string
		objectReplacements map[string]map[string]string
		want               internalversion.Params
	}{
		{
			name: "string replacement",
			ps: internalversion.Params{{
				Name:  "foo",
				Value: internalversion.ParamValue{StringVal: "$(params.foo)"},
			}},
			stringReplacements: map[string]string{
				"params.foo": "bar",
			},
			want: internalversion.Params{{
				Name:  "foo",
				Value: internalversion.ParamValue{StringVal: "bar"},
			}},
		},
		{
			name: "array replacement",
			ps: internalversion.Params{{
				Name:  "foo",
				Value: internalversion.ParamValue{StringVal: "$(params.foo)"},
			}},
			arrayReplacements: map[string][]string{
				"params.foo": {"bar", "zoo"},
			},
			want: internalversion.Params{{
				Name:  "foo",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"bar", "zoo"}},
			}},
		},
		{
			name: "object replacement",
			ps: internalversion.Params{{
				Name:  "foo",
				Value: internalversion.ParamValue{StringVal: "$(params.foo)"},
			}},
			objectReplacements: map[string]map[string]string{
				"params.foo": {
					"abc": "123",
				},
			},
			want: internalversion.Params{{
				Name:  "foo",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeObject, ObjectVal: map[string]string{"abc": "123"}},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ps.ReplaceVariables(tt.stringReplacements, tt.arrayReplacements, tt.objectReplacements)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestExtractParamArrayLengths(t *testing.T) {
	tcs := []struct {
		name   string
		params internalversion.Params
		want   map[string]int
	}{{
		name:   "string params",
		params: internalversion.Params{{Name: "foo", Value: internalversion.ParamValue{StringVal: "bar", Type: internalversion.ParamTypeString}}},
		want:   nil,
	}, {
		name:   "one array param",
		params: internalversion.Params{{Name: "foo", Value: internalversion.ParamValue{ArrayVal: []string{"bar", "baz"}, Type: internalversion.ParamTypeArray}}},
		want:   map[string]int{"foo": 2},
	}, {
		name:   "object params",
		params: internalversion.Params{{Name: "foo", Value: internalversion.ParamValue{ObjectVal: map[string]string{"bar": "baz"}, Type: internalversion.ParamTypeObject}}},
		want:   nil,
	}, {
		name: "multiple array params",
		params: internalversion.Params{
			{Name: "foo", Value: internalversion.ParamValue{ArrayVal: []string{"bar", "baz"}, Type: internalversion.ParamTypeArray}},
			{Name: "abc", Value: internalversion.ParamValue{ArrayVal: []string{"123", "456", "789"}, Type: internalversion.ParamTypeArray}},
			{Name: "empty", Value: internalversion.ParamValue{ArrayVal: []string{}, Type: internalversion.ParamTypeArray}},
		},
		want: map[string]int{"foo": 2, "abc": 3, "empty": 0},
	}, {
		name: "mixed param types",
		params: internalversion.Params{
			{Name: "foo", Value: internalversion.ParamValue{StringVal: "abc", Type: internalversion.ParamTypeString}},
			{Name: "bar", Value: internalversion.ParamValue{ArrayVal: []string{"def", "ghi"}, Type: internalversion.ParamTypeArray}},
			{Name: "baz", Value: internalversion.ParamValue{ObjectVal: map[string]string{"jkl": "mno"}, Type: internalversion.ParamTypeObject}},
		},
		want: map[string]int{"bar": 2},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.params.ExtractParamArrayLengths()
			if d := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("wrong param array lengths: %s", d)
			}
		})
	}
}

func TestExtractDefaultParamArrayLengths(t *testing.T) {
	tcs := []struct {
		name   string
		params internalversion.ParamSpecs
		want   map[string]int
	}{{
		name:   "string params",
		params: internalversion.ParamSpecs{{Name: "foo", Default: &internalversion.ParamValue{StringVal: "bar", Type: internalversion.ParamTypeString}}},
		want:   nil,
	}, {
		name:   "one array param",
		params: internalversion.ParamSpecs{{Name: "foo", Default: &internalversion.ParamValue{ArrayVal: []string{"bar", "baz"}, Type: internalversion.ParamTypeArray}}},
		want:   map[string]int{"foo": 2},
	}, {
		name:   "object params",
		params: internalversion.ParamSpecs{{Name: "foo", Default: &internalversion.ParamValue{ObjectVal: map[string]string{"bar": "baz"}, Type: internalversion.ParamTypeObject}}},
		want:   nil,
	}, {
		name: "multiple array params",
		params: internalversion.ParamSpecs{
			{Name: "foo", Default: &internalversion.ParamValue{ArrayVal: []string{"bar", "baz"}, Type: internalversion.ParamTypeArray}},
			{Name: "abc", Default: &internalversion.ParamValue{ArrayVal: []string{"123", "456", "789"}, Type: internalversion.ParamTypeArray}},
			{Name: "empty", Default: &internalversion.ParamValue{ArrayVal: []string{}, Type: internalversion.ParamTypeArray}},
		},
		want: map[string]int{"foo": 2, "abc": 3, "empty": 0},
	}, {
		name: "mixed param types",
		params: internalversion.ParamSpecs{
			{Name: "foo", Default: &internalversion.ParamValue{StringVal: "abc", Type: internalversion.ParamTypeString}},
			{Name: "bar", Default: &internalversion.ParamValue{ArrayVal: []string{"def", "ghi"}, Type: internalversion.ParamTypeArray}},
			{Name: "baz", Default: &internalversion.ParamValue{ObjectVal: map[string]string{"jkl": "mno"}, Type: internalversion.ParamTypeObject}},
		},
		want: map[string]int{"bar": 2},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.params.ExtractDefaultParamArrayLengths()
			if d := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("wrong default param array lengths: %s", d)
			}
		})
	}
}
