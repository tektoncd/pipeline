/*
Copyright 2019 The Tekton Authors

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
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	params "github.com/tektoncd/pipeline/pkg/apis/params/v1beta1"
	results "github.com/tektoncd/pipeline/pkg/apis/results/v1beta1"
)

func TestNewResultReference(t *testing.T) {
	type args struct {
		param params.Param
	}
	tests := []struct {
		name string
		args args
		want []*results.ResultRef
	}{
		{
			name: "Test valid expression",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.sumTask.results.sumResult)",
					},
				},
			},
			want: []*results.ResultRef{
				{
					PipelineTask: "sumTask",
					Result:       "sumResult",
				},
			},
		}, {
			name: "Test valid expression: substitution within string no spaces and another substitution",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(params.image-registry)/someimage@$(tasks.sumTask.results.sumResult)",
					},
				},
			},
			want: []*results.ResultRef{
				{
					PipelineTask: "sumTask",
					Result:       "sumResult",
				},
			},
		}, {
			name: "Test valid expression: substitution within string",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "sum-will-go-here -> $(tasks.sumTask.results.sumResult)",
					},
				},
			},
			want: []*results.ResultRef{
				{
					PipelineTask: "sumTask",
					Result:       "sumResult",
				},
			},
		}, {
			name: "Test valid expression: multiple substitution",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.sumTask1.results.sumResult) and another $(tasks.sumTask2.results.sumResult)",
					},
				},
			},
			want: []*results.ResultRef{
				{
					PipelineTask: "sumTask1",
					Result:       "sumResult",
				}, {
					PipelineTask: "sumTask2",
					Result:       "sumResult",
				},
			},
		}, {
			name: "Test invalid expression: first separator typo",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(task.sumTasks.results.sumResult)",
					},
				},
			},
			want: nil,
		}, {
			name: "Test invalid expression: third separator typo",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.sumTasks.result.sumResult)",
					},
				},
			},
			want: nil,
		}, {
			name: "Test invalid expression: param substitution shouldn't be considered result ref",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(params.paramName)",
					},
				},
			},
			want: nil,
		}, {
			name: "Test invalid expression: One bad and good result substitution",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "good -> $(tasks.sumTask1.results.sumResult) bad-> $(task.sumTask2.results.sumResult)",
					},
				},
			},
			want: []*results.ResultRef{
				{
					PipelineTask: "sumTask1",
					Result:       "sumResult",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := results.GetVarSubstitutionExpressionsForParam(tt.args.param)
			if ok {
				got := results.NewResultRefs(expressions)
				if d := cmp.Diff(tt.want, got); d != "" {
					t.Errorf("TestNewResultReference/%s (-want, +got) = %v", tt.name, d)
				}
			}
		})
	}
}

func TestHasResultReference(t *testing.T) {
	type args struct {
		param params.Param
	}
	tests := []struct {
		name    string
		args    args
		wantRef []*results.ResultRef
	}{
		{
			name: "Test valid expression",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.sumTask.results.sumResult)",
					},
				},
			},
			wantRef: []*results.ResultRef{
				{
					PipelineTask: "sumTask",
					Result:       "sumResult",
				},
			},
		}, {
			name: "Test valid expression with dashes",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.sum-task.results.sum-result)",
					},
				},
			},
			wantRef: []*results.ResultRef{
				{
					PipelineTask: "sum-task",
					Result:       "sum-result",
				},
			},
		}, {
			name: "Test invalid expression: param substitution shouldn't be considered result ref",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(params.paramName)",
					},
				},
			},
			wantRef: nil,
		}, {
			name: "Test valid expression in array",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:     params.ParamTypeArray,
						ArrayVal: []string{"$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"},
					},
				},
			},
			wantRef: []*results.ResultRef{
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
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:     params.ParamTypeArray,
						ArrayVal: []string{"1", "$(tasks.sumTask2.results.sumResult2)"},
					},
				},
			},
			wantRef: []*results.ResultRef{
				{
					PipelineTask: "sumTask2",
					Result:       "sumResult2",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := results.GetVarSubstitutionExpressionsForParam(tt.args.param)
			if ok {
				got := results.NewResultRefs(expressions)
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
			}
		})
	}
}

func TestLooksLikeResultRef(t *testing.T) {
	type args struct {
		param params.Param
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test expression that is a result ref",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.sumTasks.results.sumResult)",
					},
				},
			},
			want: true,
		}, {
			name: "test expression: looks like result ref, but typo in 'task' separator",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(task.sumTasks.results.sumResult)",
					},
				},
			},
			want: true,
		}, {
			name: "test expression: looks like result ref, but typo in 'results' separator",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.sumTasks.result.sumResult)",
					},
				},
			},
			want: true,
		}, {
			name: "test expression: missing 'task' separator",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(sumTasks.results.sumResult)",
					},
				},
			},
			want: false,
		}, {
			name: "test expression: missing variable substitution",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "tasks.sumTasks.results.sumResult",
					},
				},
			},
			want: false,
		}, {
			name: "test expression: param substitution shouldn't be considered result ref",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(params.someParam)",
					},
				},
			},
			want: false,
		}, {
			name: "test expression: one good ref, one bad one should return true",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.sumTasks.results.sumResult) $(task.sumTasks.results.sumResult)",
					},
				},
			},
			want: true,
		}, {
			name: "test expression: inside array parameter",
			args: args{
				param: params.Param{
					Name: "param",
					Value: params.ArrayOrString{
						Type:     params.ParamTypeArray,
						ArrayVal: []string{"$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := results.GetVarSubstitutionExpressionsForParam(tt.args.param)
			if ok {
				if got := results.LooksLikeContainsResultRefs(expressions); got != tt.want {
					t.Errorf("LooksLikeContainsResultRefs() = %v, want %v", got, tt.want)
				}
			} else if tt.want {
				t.Errorf("LooksLikeContainsResultRefs() = %v, want %v", false, tt.want)
			}
		})
	}
}
