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
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/builder"
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
		name: "inferred type from default value",
		before: &v1beta1.ParamSpec{
			Name:    "parametername",
			Default: builder.ArrayOrString("an", "array"),
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name:    "parametername",
			Type:    v1beta1.ParamTypeArray,
			Default: builder.ArrayOrString("an", "array"),
		},
	}, {
		name: "fully defined ParamSpec",
		before: &v1beta1.ParamSpec{
			Name:        "parametername",
			Type:        v1beta1.ParamTypeArray,
			Description: "a description",
			Default:     builder.ArrayOrString("an", "array"),
		},
		defaultsApplied: &v1beta1.ParamSpec{
			Name:        "parametername",
			Type:        v1beta1.ParamTypeArray,
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
		input              *v1beta1.ArrayOrString
		stringReplacements map[string]string
		arrayReplacements  map[string][]string
	}
	tests := []struct {
		name           string
		args           args
		expectedOutput *v1beta1.ArrayOrString
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
	AOrS v1beta1.ArrayOrString `json:"val"`
}

func TestArrayOrString_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		input  string
		result v1beta1.ArrayOrString
	}{
		{"{\"val\": \"123\"}", *builder.ArrayOrString("123")},
		{"{\"val\": \"\"}", *builder.ArrayOrString("")},
		{"{\"val\":[]}", v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{}}},
		{"{\"val\":[\"oneelement\"]}", v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"oneelement"}}},
		{"{\"val\":[\"multiple\", \"elements\"]}", v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"multiple", "elements"}}},
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
		input  v1beta1.ArrayOrString
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

func TestNewResultReference(t *testing.T) {
	type args struct {
		param v1beta1.Param
	}
	tests := []struct {
		name    string
		args    args
		want    []*v1beta1.ResultRef
		wantErr bool
	}{
		{
			name: "Test valid expression",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.sumTask.results.sumResult)",
					},
				},
			},
			want: []*v1beta1.ResultRef{
				{
					PipelineTask: "sumTask",
					Result:       "sumResult",
				},
			},
			wantErr: false,
		}, {
			name: "Test valid expression: substitution within string",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "sum-will-go-here -> $(tasks.sumTask.results.sumResult)",
					},
				},
			},
			want: []*v1beta1.ResultRef{
				{
					PipelineTask: "sumTask",
					Result:       "sumResult",
				},
			},
			wantErr: false,
		}, {
			name: "Test valid expression: multiple substitution",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.sumTask1.results.sumResult) and another $(tasks.sumTask2.results.sumResult)",
					},
				},
			},
			want: []*v1beta1.ResultRef{
				{
					PipelineTask: "sumTask1",
					Result:       "sumResult",
				}, {
					PipelineTask: "sumTask2",
					Result:       "sumResult",
				},
			},
			wantErr: false,
		}, {
			name: "Test invalid expression: first separator typo",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(task.sumTasks.results.sumResult)",
					},
				},
			},
			want:    nil,
			wantErr: true,
		}, {
			name: "Test invalid expression: third separator typo",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.sumTasks.result.sumResult)",
					},
				},
			},
			want:    nil,
			wantErr: true,
		}, {
			name: "Test invalid expression: param substitution shouldn't be considered result ref",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(params.paramName)",
					},
				},
			},
			want:    nil,
			wantErr: true,
		}, {
			name: "Test invalid expression: One bad and good result substitution",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "good -> $(tasks.sumTask1.results.sumResult) bad-> $(task.sumTask2.results.sumResult)",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1beta1.NewResultRefs(tt.args.param)
			if tt.wantErr != (err != nil) {
				t.Errorf("TestNewResultReference/%s wantErr %v, error = %v", tt.name, tt.wantErr, err)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("TestNewResultReference/%s (-want, +got) = %v", tt.name, d)
			}
		})
	}
}

func TestHasResultReference(t *testing.T) {
	type args struct {
		param v1beta1.Param
	}
	tests := []struct {
		name    string
		args    args
		wantRef []*v1beta1.ResultRef
	}{
		{
			name: "Test valid expression",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.sumTask.results.sumResult)",
					},
				},
			},
			wantRef: []*v1beta1.ResultRef{
				{
					PipelineTask: "sumTask",
					Result:       "sumResult",
				},
			},
		}, {
			name: "Test valid expression with dashes",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.sum-task.results.sum-result)",
					},
				},
			},
			wantRef: []*v1beta1.ResultRef{
				{
					PipelineTask: "sum-task",
					Result:       "sum-result",
				},
			},
		}, {
			name: "Test invalid expression: param substitution shouldn't be considered result ref",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(params.paramName)",
					},
				},
			},
			wantRef: nil,
		}, {
			name: "Test valid expression in array",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"},
					},
				},
			},
			wantRef: []*v1beta1.ResultRef{
				{
					PipelineTask: "sumTask",
					Result:       "sumResult",
				},
				{
					PipelineTask: "sumTask2",
					Result:       "sumResult2",
				},
			},
		}, {
			name: "Test valid expression in array - no ref in first element",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"1", "$(tasks.sumTask2.results.sumResult2)"},
					},
				},
			},
			wantRef: []*v1beta1.ResultRef{
				{
					PipelineTask: "sumTask2",
					Result:       "sumResult2",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := v1beta1.NewResultRefs(tt.args.param)
			sort.Slice(got, func(i, j int) bool {
				if got[i].PipelineTask > got[j].PipelineTask {
					return false
				}
				if got[i].Result > got[j].Result {
					return false
				}
				return true
			})
			if d := cmp.Diff(tt.wantRef, got); d != "" {
				t.Errorf("TestHasResultReference/%s (-want, +got) = %v", tt.name, d)
			}
		})
	}
}

func TestLooksLikeResultRef(t *testing.T) {
	type args struct {
		param v1beta1.Param
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test expression that is a result ref",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.sumTasks.results.sumResult)",
					},
				},
			},
			want: true,
		}, {
			name: "test expression: looks like result ref, but typo in 'task' separator",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(task.sumTasks.results.sumResult)",
					},
				},
			},
			want: true,
		}, {
			name: "test expression: looks like result ref, but typo in 'results' separator",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.sumTasks.result.sumResult)",
					},
				},
			},
			want: true,
		}, {
			name: "test expression: missing 'task' separator",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(sumTasks.results.sumResult)",
					},
				},
			},
			want: false,
		}, {
			name: "test expression: missing variable substitution",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "tasks.sumTasks.results.sumResult",
					},
				},
			},
			want: false,
		}, {
			name: "test expression: param substitution shouldn't be considered result ref",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(params.someParam)",
					},
				},
			},
			want: false,
		}, {
			name: "test expression: one good ref, one bad one should return true",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.sumTasks.results.sumResult) $(task.sumTasks.results.sumResult)",
					},
				},
			},
			want: true,
		}, {
			name: "test expression: inside array parameter",
			args: args{
				param: v1beta1.Param{
					Name: "param",
					Value: v1beta1.ArrayOrString{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v1beta1.LooksLikeContainsResultRefs(tt.args.param); got != tt.want {
				t.Errorf("LooksLikeContainsResultRefs() = %v, want %v", got, tt.want)
			}
		})
	}
}
