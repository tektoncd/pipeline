/*
Copyright 2020 The Tekton Authors
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
package jsonpath

import (
	"fmt"
	"reflect"
	"testing"
)

func hasVariables(text string) bool {
	matches := expandRE.FindAllString(text, -1)
	for _, m := range matches {
		if m != "$$" {
			return true
		}
	}
	return false
}
func TestHasVariables(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"", false},
		{"abc", false},
		{"$(foo.bar)", true},
		{"abc $(foo.bar)", true},
		{"$$(foo.bar)", false},
		{"$$(foo.bar) $(foo.bar)", true},
		{"$$$(foo.bar)", true},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			if hasVariables(test.text) != test.expected {
				t.Errorf("hasVariables() for  %v unexpectedly return %v.", test.text, !test.expected)
			}
		})
	}
}

func TestExpandVariable(t *testing.T) {
	context := map[string]interface{}{
		"bool":   true,
		"num":    float64(99),
		"string": "abc",
		"array":  []interface{}{float64(1), float64(2), float64(3), "a", "b", "c"},
		"obj": map[string]interface{}{
			"a": float64(1),
			"b": "abcd",
		},
		"null": nil,
	}

	tests := []struct {
		variable   string
		expected   interface{}
		expectList bool
	}{
		{"$('')", "", false},
		{"$(bool)", context["bool"], false},
		{"$(['bool'])", context["bool"], false},
		{"$(num)", context["num"], false},
		{"$(string)", context["string"], false},
		{"$(array)", context["array"], false},
		{"$(obj)", context["obj"], false},
		{"$(obj.a)", context["obj"].(map[string]interface{})["a"], false},
		{"$(['obj'].a)", context["obj"].(map[string]interface{})["a"], false},
		{"$(['obj']['a'])", context["obj"].(map[string]interface{})["a"], false},
		{"$(obj['a'])", context["obj"].(map[string]interface{})["a"], false},
		{"$(null)", context["null"], false},
		{"$(array[*])", context["array"], true},
		{"$(array[1:3])", []interface{}{context["array"].([]interface{})[1], context["array"].([]interface{})[2]}, true},
	}

	for _, test := range tests {
		t.Run(test.variable, func(t *testing.T) {
			result, err := expandVariable(test.variable, context)
			if err != nil {
				t.Fatalf("Could not expandVariable : %q", err)
			}

			if !test.expectList {
				if !reflect.DeepEqual(result[0], test.expected) {
					t.Errorf("expandVariable() for %v - expected %v (%T) but was %v (%T).", test.variable, test.expected, test.expected, result[0], result[0])
				}
			} else {
				if !reflect.DeepEqual(result, test.expected) {
					t.Errorf("expandVariable() for %v - expected %v (%T) but was %v (%T).", test.variable, test.expected, test.expected, result, result)
				}
			}
		})
	}
}

func arrayLookup(arr interface{}, i int) interface{} {
	return arr.([]interface{})[i]
}

func TestExpand(t *testing.T) {
	context := map[string]interface{}{
		"bool":   true,
		"num":    float64(99),
		"string": "abc",
		"array":  []interface{}{float64(1), float64(2), float64(3), "a", "b", "c"},
		"obj": map[string]interface{}{
			"a": float64(1),
			"b": "abcd",
		},
		"null":        nil,
		"empty_array": []interface{}{},
	}

	tests := []struct {
		input    interface{}
		expected interface{}
	}{
		{true, true},
		{false, false},
		{nil, nil},
		{float64(99), float64(99)},
		{context["array"], context["array"]},
		{context["obj"], context["obj"]},
		{"$('')", ""},
		{`$("")`, ""},
		{`$$`, "$"},
		{"$(bool)", context["bool"]},
		{"$(['bool'])", context["bool"]},
		{"$(num)", context["num"]},
		{"$(string)", context["string"]},
		{"$(array)", context["array"]},
		{"$(obj)", context["obj"]},
		{"$(obj.a)", context["obj"].(map[string]interface{})["a"]},
		{"$(['obj'].a)", context["obj"].(map[string]interface{})["a"]},
		{"$(['obj']['a'])", context["obj"].(map[string]interface{})["a"]},
		{"$(obj['a'])", context["obj"].(map[string]interface{})["a"]},
		{"$(null)", context["null"]},
		{"$(array[*])", context["array"].([]interface{})[0]},
		{"$(array[1:3])", context["array"].([]interface{})[1]},
		{[]interface{}{"$(array[*])"}, context["array"]},
		{[]interface{}{"$(array[1:3])"}, []interface{}{arrayLookup(context["array"], 1), arrayLookup(context["array"], 2)}},
		{[]interface{}{float64(1), "$(array[1:3])", "a", "b", "c"}, context["array"]},
		{"$('')$(null)", "null"},
		{"$('')$(obj)", `{"a":1,"b":"abcd"}`},
		{"$('')$(array)", `[1,2,3,"a","b","c"]`},
		{"$(empty_array[*])", ""},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s", test.input), func(t *testing.T) {
			result, err := Expand(test.input, context)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("expandVariable() for %v - expected %v (%T) but was %v (%T).", test.input, test.expected, test.expected, result, result)
			}
		})
	}
}

// these tests come from https://github.com/kubernetes/kubernetes/blob/master/third_party/forked/golang/expansion/expand_test.go
func TestContainerEnvMapping(t *testing.T) {
	context := map[string]string{
		"VAR_A":     "A",
		"VAR_B":     "B",
		"VAR_C":     "C",
		"VAR_REF":   "$(VAR_A)",
		"VAR_EMPTY": "",
	}

	var expectedError error

	cases := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "whole string",
			input:    "$(VAR_A)",
			expected: "A",
		},
		{
			name:     "repeat",
			input:    "$(VAR_A)-$(VAR_A)",
			expected: "A-A",
		},
		{
			name:     "beginning",
			input:    "$(VAR_A)-1",
			expected: "A-1",
		},
		{
			name:     "middle",
			input:    "___$(VAR_B)___",
			expected: "___B___",
		},
		{
			name:     "end",
			input:    "___$(VAR_C)",
			expected: "___C",
		},
		{
			name:     "compound",
			input:    "$(VAR_A)_$(VAR_B)_$(VAR_C)",
			expected: "A_B_C",
		},
		{
			name:     "escape & expand",
			input:    "$$(VAR_B)_$(VAR_A)",
			expected: "$(VAR_B)_A",
		},
		{
			name:     "compound escape",
			input:    "$$(VAR_A)_$$(VAR_B)",
			expected: "$(VAR_A)_$(VAR_B)",
		},
		{
			name:     "mixed in escapes",
			input:    "f000-$$VAR_A",
			expected: "f000-$VAR_A",
		},
		{
			name:     "backslash escape ignored",
			input:    "foo\\$(VAR_C)bar",
			expected: "foo\\Cbar",
		},
		{
			name:     "backslash escape ignored",
			input:    "foo\\\\$(VAR_C)bar",
			expected: "foo\\\\Cbar",
		},
		{
			name:     "lots of backslashes",
			input:    "foo\\\\\\\\$(VAR_A)bar",
			expected: "foo\\\\\\\\Abar",
		},
		{
			name:     "nested var references",
			input:    "$(VAR_A$(VAR_B))",
			expected: expectedError, //"$(VAR_A$(VAR_B))" -- in Tekton this is a bad JSONPath expression
		},
		{
			name:     "nested var references second type",
			input:    "$(VAR_A$(VAR_B)",
			expected: "$(VAR_AB", //"$(VAR_A$(VAR_B)"
		},
		{
			name:     "value is a reference",
			input:    "$(VAR_REF)",
			expected: "$(VAR_A)",
		},
		{
			name:     "value is a reference x 2",
			input:    "%%$(VAR_REF)--$(VAR_REF)%%",
			expected: "%%$(VAR_A)--$(VAR_A)%%",
		},
		{
			name:     "empty var",
			input:    "foo$(VAR_EMPTY)bar",
			expected: "foobar",
		},
		{
			name:     "unterminated expression",
			input:    "foo$(VAR_Awhoops!",
			expected: "foo$(VAR_Awhoops!",
		},
		{
			name:     "expression without operator",
			input:    "f00__(VAR_A)__",
			expected: "f00__(VAR_A)__",
		},
		{
			name:     "shell special vars pass through",
			input:    "$?_boo_$!",
			expected: "$?_boo_$!",
		},
		{
			name:     "bare operators are ignored",
			input:    "$VAR_A",
			expected: "$VAR_A",
		},
		{
			name:     "undefined vars are passed through",
			input:    "$(VAR_DNE)",
			expected: expectedError, //"$(VAR_DNE)" -- in Tekton a missing key is an error
		},
		{
			name:     "multiple (even) operators, var undefined",
			input:    "$$$$$$(BIG_MONEY)",
			expected: "$$$(BIG_MONEY)",
		},
		{
			name:     "multiple (even) operators, var defined",
			input:    "$$$$$$(VAR_A)",
			expected: "$$$(VAR_A)",
		},
		{
			name:     "multiple (odd) operators, var undefined",
			input:    "$$$$$$$(GOOD_ODDS)",
			expected: expectedError, //"$$$$(GOOD_ODDS)" -- in Tekton a missing key is an error
		},
		{
			name:     "multiple (odd) operators, var defined",
			input:    "$$$$$$$(VAR_A)",
			expected: "$$$A",
		},
		{
			name:     "missing open expression",
			input:    "$VAR_A)",
			expected: "$VAR_A)",
		},
		{
			name:     "shell syntax ignored",
			input:    "${VAR_A}",
			expected: "${VAR_A}",
		},
		{
			name:     "trailing incomplete expression not consumed",
			input:    "$(VAR_B)_______$(A",
			expected: "B_______$(A",
		},
		{
			name:     "trailing incomplete expression, no content, is not consumed",
			input:    "$(VAR_C)_______$(",
			expected: "C_______$(",
		},
		{
			name:     "operator at end of input string is preserved",
			input:    "$(VAR_A)foobarzab$",
			expected: "Afoobarzab$",
		},
		{
			name:     "shell escaped incomplete expr",
			input:    "foo-\\$(VAR_A",
			expected: "foo-\\$(VAR_A",
		},
		{
			name:     "lots of $( in middle",
			input:    "--$($($($($--",
			expected: "--$($($($($--",
		},
		{
			name:     "lots of $( in beginning",
			input:    "$($($($($--foo$(",
			expected: "$($($($($--foo$(",
		},
		{
			name:     "lots of $( at end",
			input:    "foo0--$($($($(",
			expected: "foo0--$($($($(",
		},
		{
			name:     "escaped operators in variable names are not escaped",
			input:    "$('foo$$var')",
			expected: "foo$$var", //"$(foo$$var)" -- (reworked valid testcase) in Tekton a missing key is an error
		},
		{
			name:     "newline not expanded",
			input:    "\n",
			expected: "\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Expand(tc.input, context)
			if err != nil {
				if tc.expected == expectedError {
					return
				}
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("expandVariable() for %v - expected %v (%T) but was %v (%T).", tc.input, tc.expected, tc.expected, result, result)
			}
		})
	}
}
