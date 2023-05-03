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

package v1beta1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestParamSpec_SetDefaults(t *testing.T) {
	tests := []struct {
		name            string
		before          *v1beta1.ParamSpec
		defaultsApplied *v1beta1.ParamSpec
	}{{
		name: "inferred string type",
		before: &v1beta1.ParamSpec{
			Name: "parametername",
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name: "parametername",
			Type: v1beta1.ParamTypeString,
		},
	}, {
		name: "inferred type from default value - array",
		before: &v1beta1.ParamSpec{
			Name: "parametername",
			Default: &v1beta1.ParamValue{
				ArrayVal: []string{"array"},
			},
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name: "parametername",
			Type: v1beta1.ParamTypeArray,
			Default: &v1beta1.ParamValue{
				ArrayVal: []string{"array"},
			},
		},
	}, {
		name: "inferred type from default value - string",
		before: &v1beta1.ParamSpec{
			Name: "parametername",
			Default: &v1beta1.ParamValue{
				StringVal: "an",
			},
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name: "parametername",
			Type: v1beta1.ParamTypeString,
			Default: &v1beta1.ParamValue{
				StringVal: "an",
			},
		},
	}, {
		name: "inferred type from default value - object",
		before: &v1beta1.ParamSpec{
			Name: "parametername",
			Default: &v1beta1.ParamValue{
				ObjectVal: map[string]string{"url": "test", "path": "test"},
			},
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name: "parametername",
			Type: v1beta1.ParamTypeObject,
			Default: &v1beta1.ParamValue{
				ObjectVal: map[string]string{"url": "test", "path": "test"},
			},
		},
	}, {
		name: "inferred type from properties - PropertySpec type is not provided",
		before: &v1beta1.ParamSpec{
			Name:       "parametername",
			Properties: map[string]v1beta1.PropertySpec{"key1": {}},
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name:       "parametername",
			Type:       v1beta1.ParamTypeObject,
			Properties: map[string]v1beta1.PropertySpec{"key1": {Type: "string"}},
		},
	}, {
		name: "inferred type from properties - PropertySpec type is provided",
		before: &v1beta1.ParamSpec{
			Name:       "parametername",
			Properties: map[string]v1beta1.PropertySpec{"key2": {Type: "string"}},
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name:       "parametername",
			Type:       v1beta1.ParamTypeObject,
			Properties: map[string]v1beta1.PropertySpec{"key2": {Type: "string"}},
		},
	}, {
		name: "fully defined ParamSpec - array",
		before: &v1beta1.ParamSpec{
			Name:        "parametername",
			Type:        v1beta1.ParamTypeArray,
			Description: "a description",
			Default: &v1beta1.ParamValue{
				ArrayVal: []string{"array"},
			},
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name:        "parametername",
			Type:        v1beta1.ParamTypeArray,
			Description: "a description",
			Default: &v1beta1.ParamValue{
				ArrayVal: []string{"array"},
			},
		},
	}, {
		name: "fully defined ParamSpec - object",
		before: &v1beta1.ParamSpec{
			Name:        "parametername",
			Type:        v1beta1.ParamTypeObject,
			Description: "a description",
			Default: &v1beta1.ParamValue{
				ObjectVal: map[string]string{"url": "test", "path": "test"},
			},
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name:        "parametername",
			Type:        v1beta1.ParamTypeObject,
			Description: "a description",
			Default: &v1beta1.ParamValue{
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
		input              *v1beta1.ParamValue
		stringReplacements map[string]string
		arrayReplacements  map[string][]string
		objectReplacements map[string]map[string]string
	}
	tests := []struct {
		name           string
		args           args
		expectedOutput *v1beta1.ParamValue
	}{{
		name: "no replacements on array",
		args: args{
			input:              v1beta1.NewStructuredValues("an", "array"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"sdf", "sdfsd"}},
		},
		expectedOutput: v1beta1.NewStructuredValues("an", "array"),
	}, {
		name: "single string replacement on string",
		args: args{
			input:              v1beta1.NewStructuredValues("$(params.myString1)"),
			stringReplacements: map[string]string{"params.myString1": "value1", "params.myString2": "value2"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: v1beta1.NewStructuredValues("value1"),
	}, {
		name: "multiple string replacements on string",
		args: args{
			input:              v1beta1.NewStructuredValues("astring$(some) asdf $(anotherkey)"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: v1beta1.NewStructuredValues("astringvalue asdf value"),
	}, {
		name: "single array replacement",
		args: args{
			input:              v1beta1.NewStructuredValues("firstvalue", "$(arraykey)", "lastvalue"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: v1beta1.NewStructuredValues("firstvalue", "array", "value", "lastvalue"),
	}, {
		name: "multiple array replacement",
		args: args{
			input:              v1beta1.NewStructuredValues("firstvalue", "$(arraykey)", "lastvalue", "$(sdfdf)"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {"array", "value"}, "sdfdf": {"asdf", "sdfsd"}},
		},
		expectedOutput: v1beta1.NewStructuredValues("firstvalue", "array", "value", "lastvalue", "asdf", "sdfsd"),
	}, {
		name: "empty array replacement without extra elements",
		args: args{
			input:             v1beta1.NewStructuredValues("$(arraykey)"),
			arrayReplacements: map[string][]string{"arraykey": {}},
		},
		expectedOutput: &v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{}},
	}, {
		name: "empty array replacement with extra elements",
		args: args{
			input:              v1beta1.NewStructuredValues("firstvalue", "$(arraykey)", "lastvalue"),
			stringReplacements: map[string]string{"some": "value", "anotherkey": "value"},
			arrayReplacements:  map[string][]string{"arraykey": {}},
		},
		expectedOutput: v1beta1.NewStructuredValues("firstvalue", "lastvalue"),
	}, {
		name: "array replacement on string val",
		args: args{
			input:             v1beta1.NewStructuredValues("$(params.myarray)"),
			arrayReplacements: map[string][]string{"params.myarray": {"a", "b", "c"}},
		},
		expectedOutput: v1beta1.NewStructuredValues("a", "b", "c"),
	}, {
		name: "array star replacement on string val",
		args: args{
			input:             v1beta1.NewStructuredValues("$(params.myarray[*])"),
			arrayReplacements: map[string][]string{"params.myarray": {"a", "b", "c"}},
		},
		expectedOutput: v1beta1.NewStructuredValues("a", "b", "c"),
	}, {
		name: "array indexing replacement on string val",
		args: args{
			input:              v1beta1.NewStructuredValues("$(params.myarray[0])"),
			stringReplacements: map[string]string{"params.myarray[0]": "a", "params.myarray[1]": "b"},
		},
		expectedOutput: v1beta1.NewStructuredValues("a"),
	}, {
		name: "object replacement on string val",
		args: args{
			input: v1beta1.NewStructuredValues("$(params.object)"),
			objectReplacements: map[string]map[string]string{
				"params.object": {
					"url":    "abc.com",
					"commit": "af234",
				},
			},
		},
		expectedOutput: v1beta1.NewObject(map[string]string{
			"url":    "abc.com",
			"commit": "af234",
		}),
	}, {
		name: "object star replacement on string val",
		args: args{
			input: v1beta1.NewStructuredValues("$(params.object[*])"),
			objectReplacements: map[string]map[string]string{
				"params.object": {
					"url":    "abc.com",
					"commit": "af234",
				},
			},
		},
		expectedOutput: v1beta1.NewObject(map[string]string{
			"url":    "abc.com",
			"commit": "af234",
		}),
	}, {
		name: "string replacement on object individual variables",
		args: args{
			input: v1beta1.NewObject(map[string]string{
				"key1": "$(mystring)",
				"key2": "$(anotherObject.key)",
			}),
			stringReplacements: map[string]string{
				"mystring":          "foo",
				"anotherObject.key": "bar",
			},
		},
		expectedOutput: v1beta1.NewObject(map[string]string{
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
	AOrS v1beta1.ParamValue `json:"val"`
}

func TestParamValues_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		input  map[string]interface{}
		result v1beta1.ParamValue
	}{
		{
			input:  map[string]interface{}{"val": 123},
			result: *v1beta1.NewStructuredValues("123"),
		},
		{
			input:  map[string]interface{}{"val": "123"},
			result: *v1beta1.NewStructuredValues("123"),
		},
		{
			input:  map[string]interface{}{"val": ""},
			result: *v1beta1.NewStructuredValues(""),
		},
		{
			input:  map[string]interface{}{"val": nil},
			result: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, ArrayVal: nil},
		},
		{
			input:  map[string]interface{}{"val": []string{}},
			result: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{}},
		},
		{
			input:  map[string]interface{}{"val": []string{"oneelement"}},
			result: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"oneelement"}},
		},
		{
			input:  map[string]interface{}{"val": []string{"multiple", "elements"}},
			result: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"multiple", "elements"}},
		},
		{
			input:  map[string]interface{}{"val": map[string]string{"key1": "val1", "key2": "val2"}},
			result: v1beta1.ParamValue{Type: v1beta1.ParamTypeObject, ObjectVal: map[string]string{"key1": "val1", "key2": "val2"}},
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
		expected v1beta1.ParamValue
	}{
		{desc: "empty value", input: ``, expected: *v1beta1.NewStructuredValues("")},
		{desc: "int value", input: `1`, expected: *v1beta1.NewStructuredValues("1")},
		{desc: "int array", input: `[1,2,3]`, expected: *v1beta1.NewStructuredValues("[1,2,3]")},
		{desc: "nested array", input: `[1,\"2\",3]`, expected: *v1beta1.NewStructuredValues(`[1,\"2\",3]`)},
		{desc: "string value", input: `hello`, expected: *v1beta1.NewStructuredValues("hello")},
		{desc: "array value", input: `["hello","world"]`, expected: *v1beta1.NewStructuredValues("hello", "world")},
		{desc: "object value", input: `{"hello":"world"}`, expected: *v1beta1.NewObject(map[string]string{"hello": "world"})},
	}

	for _, c := range cases {
		v := v1beta1.ParamValue{}
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
		input  v1beta1.ParamValue
		result string
	}{
		{*v1beta1.NewStructuredValues("123"), "{\"val\":\"123\"}"},
		{*v1beta1.NewStructuredValues("123", "1234"), "{\"val\":[\"123\",\"1234\"]}"},
		{*v1beta1.NewStructuredValues("a", "a", "a"), "{\"val\":[\"a\",\"a\",\"a\"]}"},
		{*v1beta1.NewObject(map[string]string{"key1": "var1", "key2": "var2"}), "{\"val\":{\"key1\":\"var1\",\"key2\":\"var2\"}}"},
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
		if d := cmp.Diff(tt.expectedResult, v1beta1.ArrayReference(tt.p)); d != "" {
			t.Errorf(diff.PrintWantGot(d))
		}
	}
}

func TestArrayOrString(t *testing.T) {
	tests := []struct {
		name     string
		inputA   string
		inputB   string
		expected *v1beta1.ParamValue
	}{{
		name:     "string",
		inputA:   "astring",
		inputB:   "",
		expected: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "astring"},
	}, {
		name:     "array",
		inputA:   "astring",
		inputB:   "bstring",
		expected: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"astring", "bstring"}},
	}}
	for _, tt := range tests {
		expected := &v1beta1.ArrayOrString{}
		if tt.inputB == "" {
			expected = v1beta1.NewArrayOrString(tt.inputA)
		} else {
			expected = v1beta1.NewArrayOrString(tt.inputA, tt.inputB)
		}

		if d := cmp.Diff(expected, tt.expected); d != "" {
			t.Errorf(diff.PrintWantGot(d))
		}
	}
}

func TestExtractNames(t *testing.T) {
	tests := []struct {
		name   string
		params v1beta1.Params
		want   sets.String
	}{{
		name:   "no params",
		params: v1beta1.Params{{}},
		want:   sets.NewString(""),
	}, {
		name: "extract param names from ParamTypeString",
		params: v1beta1.Params{{
			Name: "IMAGE", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "image-1"},
		}, {
			Name: "DOCKERFILE", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
		want: sets.NewString("IMAGE", "DOCKERFILE"),
	}, {
		name: "extract param names from ParamTypeArray",
		params: v1beta1.Params{{
			Name: "GOARCH", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
		}},
		want: sets.NewString("GOARCH"),
	}, {
		name: "extract param names from ParamTypeString and ParamTypeArray",
		params: v1beta1.Params{{
			Name: "GOARCH", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
		}, {
			Name: "IMAGE", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "image-1"},
		}},
		want: sets.NewString("GOARCH", "IMAGE"),
	}, {
		name: "extract param name from duplicate params",
		params: v1beta1.Params{{
			Name: "duplicate", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
		}, {
			Name: "duplicate", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "image-1"},
		}},
		want: sets.NewString("duplicate"),
	}}
	for _, tt := range tests {
		if d := cmp.Diff(tt.want, v1beta1.Params.ExtractNames(tt.params)); d != "" {
			t.Errorf(diff.PrintWantGot(d))
		}
	}
}

func TestParams_ReplaceVariables(t *testing.T) {
	tests := []struct {
		name               string
		ps                 v1beta1.Params
		stringReplacements map[string]string
		arrayReplacements  map[string][]string
		objectReplacements map[string]map[string]string
		want               v1beta1.Params
	}{
		{
			name: "string replacement",
			ps: v1beta1.Params{{
				Name:  "foo",
				Value: v1beta1.ParamValue{StringVal: "$(params.foo)"},
			}},
			stringReplacements: map[string]string{
				"params.foo": "bar",
			},
			want: v1beta1.Params{{
				Name:  "foo",
				Value: v1beta1.ParamValue{StringVal: "bar"},
			}},
		},
		{
			name: "array replacement",
			ps: v1beta1.Params{{
				Name:  "foo",
				Value: v1beta1.ParamValue{StringVal: "$(params.foo)"},
			}},
			arrayReplacements: map[string][]string{
				"params.foo": {"bar", "zoo"},
			},
			want: v1beta1.Params{{
				Name:  "foo",
				Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"bar", "zoo"}},
			}},
		},
		{
			name: "object replacement",
			ps: v1beta1.Params{{
				Name:  "foo",
				Value: v1beta1.ParamValue{StringVal: "$(params.foo)"},
			}},
			objectReplacements: map[string]map[string]string{
				"params.foo": {
					"abc": "123",
				},
			},
			want: v1beta1.Params{{
				Name:  "foo",
				Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeObject, ObjectVal: map[string]string{"abc": "123"}},
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
		params v1beta1.Params
		want   map[string]int
	}{{
		name:   "string params",
		params: v1beta1.Params{{Name: "foo", Value: v1beta1.ParamValue{StringVal: "bar", Type: v1beta1.ParamTypeString}}},
		want:   nil,
	}, {
		name:   "one array param",
		params: v1beta1.Params{{Name: "foo", Value: v1beta1.ParamValue{ArrayVal: []string{"bar", "baz"}, Type: v1beta1.ParamTypeArray}}},
		want:   map[string]int{"foo": 2},
	}, {
		name:   "object params",
		params: v1beta1.Params{{Name: "foo", Value: v1beta1.ParamValue{ObjectVal: map[string]string{"bar": "baz"}, Type: v1beta1.ParamTypeObject}}},
		want:   nil,
	}, {
		name: "multiple array params",
		params: v1beta1.Params{
			{Name: "foo", Value: v1beta1.ParamValue{ArrayVal: []string{"bar", "baz"}, Type: v1beta1.ParamTypeArray}},
			{Name: "abc", Value: v1beta1.ParamValue{ArrayVal: []string{"123", "456", "789"}, Type: v1beta1.ParamTypeArray}},
			{Name: "empty", Value: v1beta1.ParamValue{ArrayVal: []string{}, Type: v1beta1.ParamTypeArray}},
		},
		want: map[string]int{"foo": 2, "abc": 3, "empty": 0},
	}, {
		name: "mixed param types",
		params: v1beta1.Params{
			{Name: "foo", Value: v1beta1.ParamValue{StringVal: "abc", Type: v1beta1.ParamTypeString}},
			{Name: "bar", Value: v1beta1.ParamValue{ArrayVal: []string{"def", "ghi"}, Type: v1beta1.ParamTypeArray}},
			{Name: "baz", Value: v1beta1.ParamValue{ObjectVal: map[string]string{"jkl": "mno"}, Type: v1beta1.ParamTypeObject}},
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
		params v1beta1.ParamSpecs
		want   map[string]int
	}{{
		name:   "string params",
		params: v1beta1.ParamSpecs{{Name: "foo", Default: &v1beta1.ParamValue{StringVal: "bar", Type: v1beta1.ParamTypeString}}},
		want:   nil,
	}, {
		name:   "one array param",
		params: v1beta1.ParamSpecs{{Name: "foo", Default: &v1beta1.ParamValue{ArrayVal: []string{"bar", "baz"}, Type: v1beta1.ParamTypeArray}}},
		want:   map[string]int{"foo": 2},
	}, {
		name:   "object params",
		params: v1beta1.ParamSpecs{{Name: "foo", Default: &v1beta1.ParamValue{ObjectVal: map[string]string{"bar": "baz"}, Type: v1beta1.ParamTypeObject}}},
		want:   nil,
	}, {
		name: "multiple array params",
		params: v1beta1.ParamSpecs{
			{Name: "foo", Default: &v1beta1.ParamValue{ArrayVal: []string{"bar", "baz"}, Type: v1beta1.ParamTypeArray}},
			{Name: "abc", Default: &v1beta1.ParamValue{ArrayVal: []string{"123", "456", "789"}, Type: v1beta1.ParamTypeArray}},
			{Name: "empty", Default: &v1beta1.ParamValue{ArrayVal: []string{}, Type: v1beta1.ParamTypeArray}},
		},
		want: map[string]int{"foo": 2, "abc": 3, "empty": 0},
	}, {
		name: "mixed param types",
		params: v1beta1.ParamSpecs{
			{Name: "foo", Default: &v1beta1.ParamValue{StringVal: "abc", Type: v1beta1.ParamTypeString}},
			{Name: "bar", Default: &v1beta1.ParamValue{ArrayVal: []string{"def", "ghi"}, Type: v1beta1.ParamTypeArray}},
			{Name: "baz", Default: &v1beta1.ParamValue{ObjectVal: map[string]string{"jkl": "mno"}, Type: v1beta1.ParamTypeObject}},
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
