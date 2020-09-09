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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/selection"
)

func TestNewResultReference(t *testing.T) {
	for _, tt := range []struct {
		name  string
		param v1beta1.Param
		want  []*v1beta1.ResultRef
	}{{
		name: "Test valid expression",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTask.results.sumResult)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "substitution within string",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("sum-will-go-here -> $(tasks.sumTask.results.sumResult)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "multiple substitution",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTask1.results.sumResult) and another $(tasks.sumTask2.results.sumResult)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult",
		}},
	}, {
		name: "multiple substitution with param",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(params.param) $(tasks.sumTask1.results.sumResult) and another $(tasks.sumTask2.results.sumResult)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult",
		}},
	}, {
		name: "first separator typo",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(task.sumTasks.results.sumResult)"),
		},
		want: nil,
	}, {
		name: "third separator typo",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTasks.result.sumResult)"),
		},
		want: nil,
	}, {
		name: "param substitution shouldn't be considered result ref",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(params.paramName)"),
		},
		want: nil,
	}, {
		name: "One bad and good result substitution",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("good -> $(tasks.sumTask1.results.sumResult) bad-> $(task.sumTask2.results.sumResult)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult",
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := v1beta1.GetVarSubstitutionExpressionsForParam(tt.param)
			if !ok && tt.want != nil {
				t.Fatalf("expected to find expressions but didn't find any")
			} else {
				got := v1beta1.NewResultRefs(expressions)
				if d := cmp.Diff(tt.want, got); d != "" {
					t.Errorf("TestNewResultReference/%s %s", tt.name, diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestHasResultReference(t *testing.T) {
	for _, tt := range []struct {
		name    string
		param   v1beta1.Param
		wantRef []*v1beta1.ResultRef
	}{{
		name: "Test valid expression",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTask.results.sumResult)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "Test valid expression with dashes",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sum-task.results.sum-result)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sum-task",
			Result:       "sum-result",
		}},
	}, {
		name: "Test valid expression with underscores",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sum-task.results.sum_result)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sum-task",
			Result:       "sum_result",
		}},
	}, {
		name: "Test invalid expression: param substitution shouldn't be considered result ref",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(params.paramName)"),
		},
		wantRef: nil,
	}, {
		name: "Test valid expression in array",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult2",
		}},
	}, {
		name: "Test valid expression in array - no ref in first element",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("1", "$(tasks.sumTask2.results.sumResult2)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask2",
			Result:       "sumResult2",
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := v1beta1.GetVarSubstitutionExpressionsForParam(tt.param)
			if !ok {
				t.Fatalf("expected to find expressions but didn't find any")
			}
			got := v1beta1.NewResultRefs(expressions)
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
				t.Errorf("TestHasResultReference/%s %s", tt.name, diff.PrintWantGot(d))
			}
		})
	}
}

func TestLooksLikeResultRef(t *testing.T) {
	for _, tt := range []struct {
		name  string
		param v1beta1.Param
		want  bool
	}{{
		name: "test expression that is a result ref",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTasks.results.sumResult)"),
		},
		want: true,
	}, {
		name: "test expression: looks like result ref, but typo in 'task' separator",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(task.sumTasks.results.sumResult)"),
		},
		want: true,
	}, {
		name: "test expression: looks like result ref, but typo in 'results' separator",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTasks.result.sumResult)"),
		},
		want: true,
	}, {
		name: "test expression: missing 'task' separator",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(sumTasks.results.sumResult)"),
		},
		want: false,
	}, {
		name: "test expression: missing variable substitution",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("tasks.sumTasks.results.sumResult"),
		},
		want: false,
	}, {
		name: "test expression: param substitution shouldn't be considered result ref",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(params.someParam)"),
		},
		want: false,
	}, {
		name: "test expression: one good ref, one bad one should return true",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTasks.results.sumResult) $(task.sumTasks.results.sumResult)"),
		},
		want: true,
	}, {
		name: "test expression: inside array parameter",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewArrayOrString("$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"),
		},
		want: true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := v1beta1.GetVarSubstitutionExpressionsForParam(tt.param)
			if ok {
				if got := v1beta1.LooksLikeContainsResultRefs(expressions); got != tt.want {
					t.Errorf("LooksLikeContainsResultRefs() = %v, want %v", got, tt.want)
				}
			} else if tt.want {
				t.Errorf("LooksLikeContainsResultRefs() = %v, want %v", false, tt.want)
			}
		})
	}
}

func TestNewResultReferenceWhenExpressions(t *testing.T) {
	for _, tt := range []struct {
		name string
		we   v1beta1.WhenExpression
		want []*v1beta1.ResultRef
	}{{
		name: "Test valid expression",
		we: v1beta1.WhenExpression{
			Input:    "$(tasks.sumTask.results.sumResult)",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "substitution within string",
		we: v1beta1.WhenExpression{
			Input:    "sum-will-go-here -> $(tasks.sumTask.results.sumResult)",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "multiple substitution",
		we: v1beta1.WhenExpression{
			Input:    "$(tasks.sumTask1.results.sumResult) and another $(tasks.sumTask2.results.sumResult)",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult",
		}},
	}, {
		name: "multiple substitution with param",
		we: v1beta1.WhenExpression{
			Input:    "$(params.param) $(tasks.sumTask1.results.sumResult) and another $(tasks.sumTask2.results.sumResult)",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult",
		}},
	}, {
		name: "first separator typo",
		we: v1beta1.WhenExpression{
			Input:    "$(task.sumTasks.results.sumResult)",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
		want: nil,
	}, {
		name: "third separator typo",
		we: v1beta1.WhenExpression{
			Input:    "$(tasks.sumTasks.result.sumResult)",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
		want: nil,
	}, {
		name: "param substitution shouldn't be considered result ref",
		we: v1beta1.WhenExpression{
			Input:    "$(params.paramName)",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
		want: nil,
	}, {
		name: "One bad and good result substitution",
		we: v1beta1.WhenExpression{
			Input:    "good -> $(tasks.sumTask1.results.sumResult) bad-> $(task.sumTask2.results.sumResult)",
			Operator: selection.In,
			Values:   []string{"foo"},
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult",
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := tt.we.GetVarSubstitutionExpressions()
			if !ok {
				t.Fatalf("expected to find expressions but didn't find any")
			} else {
				got := v1beta1.NewResultRefs(expressions)
				if d := cmp.Diff(tt.want, got); d != "" {
					t.Errorf("TestNewResultReference/%s %s", tt.name, diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestHasResultReferenceWhenExpression(t *testing.T) {
	for _, tt := range []struct {
		name    string
		we      v1beta1.WhenExpression
		wantRef []*v1beta1.ResultRef
	}{{
		name: "Test valid expression",
		we: v1beta1.WhenExpression{
			Input:    "sumResult",
			Operator: selection.In,
			Values:   []string{"$(tasks.sumTask.results.sumResult)"},
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "Test valid expression with dashes",
		we: v1beta1.WhenExpression{
			Input:    "$(tasks.sum-task.results.sum-result)",
			Operator: selection.In,
			Values:   []string{"sum-result"},
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sum-task",
			Result:       "sum-result",
		}},
	}, {
		name: "Test valid expression with underscores",
		we: v1beta1.WhenExpression{
			Input:    "$(tasks.sum-task.results.sum_result)",
			Operator: selection.In,
			Values:   []string{"sum-result"},
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sum-task",
			Result:       "sum_result",
		}},
	}, {
		name: "Test invalid expression: param substitution shouldn't be considered result ref",
		we: v1beta1.WhenExpression{
			Input:    "$(params.paramName)",
			Operator: selection.In,
			Values:   []string{"sum-result"},
		},
		wantRef: nil,
	}, {
		name: "Test valid expression in array",
		we: v1beta1.WhenExpression{
			Input:    "$sumResult",
			Operator: selection.In,
			Values:   []string{"$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"},
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult2",
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := tt.we.GetVarSubstitutionExpressions()
			if !ok {
				t.Fatalf("expected to find expressions but didn't find any")
			}
			got := v1beta1.NewResultRefs(expressions)
			if d := cmp.Diff(tt.wantRef, got); d != "" {
				t.Errorf("TestHasResultReference/%s %s", tt.name, diff.PrintWantGot(d))
			}
		})
	}
}

func TestLooksLikeResultRefWhenExpressionTrue(t *testing.T) {
	tests := []struct {
		name string
		we   v1beta1.WhenExpression
	}{
		{
			name: "test expression that is a result ref",
			we: v1beta1.WhenExpression{
				Input:    "$(tasks.sumTasks.results.sumResult)",
				Operator: selection.In,
				Values:   []string{"foo"},
			},
		}, {
			name: "test expression: looks like result ref, but typo in 'task' separator",
			we: v1beta1.WhenExpression{
				Input:    "$(task.sumTasks.results.sumResult)",
				Operator: selection.In,
				Values:   []string{"foo"},
			},
		}, {
			name: "test expression: looks like result ref, but typo in 'results' separator",
			we: v1beta1.WhenExpression{
				Input:    "$(tasks.sumTasks.result.sumResult)",
				Operator: selection.In,
				Values:   []string{"foo"},
			},
		}, {
			name: "test expression: one good ref, one bad one should return true",
			we: v1beta1.WhenExpression{
				Input:    "$(tasks.sumTasks.results.sumResult) $(task.sumTasks.results.sumResult)",
				Operator: selection.In,
				Values:   []string{"foo"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := tt.we.GetVarSubstitutionExpressions()
			if !ok {
				t.Fatalf("expected to find expressions but didn't find any")
			}
			if !v1beta1.LooksLikeContainsResultRefs(expressions) {
				t.Errorf("expected expressions to look like they contain result refs")
			}
		})
	}
}

func TestLooksLikeResultRefWhenExpressionFalse(t *testing.T) {
	tests := []struct {
		name string
		we   v1beta1.WhenExpression
	}{
		{
			name: "test expression: missing 'task' separator",
			we: v1beta1.WhenExpression{
				Input:    "$(sumTasks.results.sumResult)",
				Operator: selection.In,
				Values:   []string{"foo"},
			},
		}, {
			name: "test expression: missing 'results' separator",
			we: v1beta1.WhenExpression{
				Input:    "$(tasks.sumTasks.sumResult)",
				Operator: selection.In,
				Values:   []string{"foo"},
			},
		}, {
			name: "test expression: param substitution shouldn't be considered result ref",
			we: v1beta1.WhenExpression{
				Input:    "$(params.someParam)",
				Operator: selection.In,
				Values:   []string{"foo"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := tt.we.GetVarSubstitutionExpressions()
			if !ok {
				t.Fatalf("expected to find expressions but didn't find any")
			}
			if v1beta1.LooksLikeContainsResultRefs(expressions) {
				t.Errorf("expected expressions to not look like they contain results refs")
			}
		})
	}
}
