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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask.results.sumResult)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "refer whole array result",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask.results.sumResult[*])"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "Test valid expression with single object result property",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask.results.sumResult.key1)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
			Property:     "key1",
		}},
	}, {
		name: "refer array indexing result",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask.results.sumResult[1])"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
			ResultsIndex: 1,
		}},
	}, {
		name: "Test valid expression with multiple object result properties",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask.results.imageresult.digest), and another one $(tasks.sumTask.results.imageresult.tag)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "imageresult",
			Property:     "digest",
		}, {
			PipelineTask: "sumTask",
			Result:       "imageresult",
			Property:     "tag",
		}},
	}, {
		name: "substitution within string",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("sum-will-go-here -> $(tasks.sumTask.results.sumResult)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "multiple substitution",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask1.results.sumResult) and another $(tasks.sumTask2.results.sumResult), last one $(tasks.sumTask3.results.sumResult.key1)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask3",
			Result:       "sumResult",
			Property:     "key1",
		}},
	}, {
		name: "multiple substitution with param",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(params.param) $(tasks.sumTask1.results.sumResult) and another $(tasks.sumTask2.results.sumResult), last one $(tasks.sumTask3.results.sumResult.key1)"),
		},
		want: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult",
		}, {
			PipelineTask: "sumTask3",
			Result:       "sumResult",
			Property:     "key1",
		}},
	}, {
		name: "first separator typo",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(task.sumTasks.results.sumResult)"),
		},
		want: nil,
	}, {
		name: "third separator typo",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTasks.result.sumResult)"),
		},
		want: nil,
	}, {
		name: "more than 5 dot-separated components",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTasks.result.sumResult.key.extra)"),
		},
		want: nil,
	}, {
		name: "param substitution shouldn't be considered result ref",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(params.paramName)"),
		},
		want: nil,
	}, {
		name: "One bad and good result substitution",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("good -> $(tasks.sumTask1.results.sumResult) bad-> $(task.sumTask2.results.sumResult)"),
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
					t.Error(diff.PrintWantGot(d))
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
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask.results.sumResult)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask",
			Result:       "sumResult",
		}},
	}, {
		name: "Test valid expression with dashes",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sum-task.results.sum-result)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sum-task",
			Result:       "sum-result",
		}},
	}, {
		name: "Test valid expression with underscores",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sum-task.results.sum_result)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sum-task",
			Result:       "sum_result",
		}},
	}, {
		name: "Test invalid expression: param substitution shouldn't be considered result ref",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(params.paramName)"),
		},
		wantRef: nil,
	}, {
		name: "Test valid expression in array",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"),
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
			Value: *v1beta1.NewStructuredValues("1", "$(tasks.sumTask2.results.sumResult2)"),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask2",
			Result:       "sumResult2",
		}},
	}, {
		name: "Test valid expression in object",
		param: v1beta1.Param{
			Name: "param",
			Value: *v1beta1.NewObject(map[string]string{
				"key1": "$(tasks.sumTask1.results.sumResult1)",
				"key2": "$(tasks.sumTask2.results.sumResult2) and another one $(tasks.sumTask3.results.sumResult3)",
				"key3": "no ref here",
			}),
		},
		wantRef: []*v1beta1.ResultRef{{
			PipelineTask: "sumTask1",
			Result:       "sumResult1",
		}, {
			PipelineTask: "sumTask2",
			Result:       "sumResult2",
		}, {
			PipelineTask: "sumTask3",
			Result:       "sumResult3",
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			expressions, ok := v1beta1.GetVarSubstitutionExpressionsForParam(tt.param)
			if !ok {
				t.Fatalf("expected to find expressions but didn't find any")
			}
			got := v1beta1.NewResultRefs(expressions)
			if d := cmp.Diff(tt.wantRef, got, cmpopts.SortSlices(func(i, j *v1beta1.ResultRef) bool {
				if i.PipelineTask > j.PipelineTask {
					return false
				}
				if i.Result > j.Result {
					return false
				}
				return true
			})); d != "" {
				t.Error(diff.PrintWantGot(d))
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
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTasks.results.sumResult)"),
		},
		want: true,
	}, {
		name: "test expression: invalid result ref, typo in 'task' separator",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(task.sumTasks.results.sumResult)"),
		},
		want: false,
	}, {
		name: "test expression: invalid result ref, typo in 'results' separator",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTasks.result.sumResult)"),
		},
		want: false,
	}, {
		name: "test expression: missing 'task' separator",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(sumTasks.results.sumResult)"),
		},
		want: false,
	}, {
		name: "test expression: missing variable substitution",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("tasks.sumTasks.results.sumResult"),
		},
		want: false,
	}, {
		name: "test expression: param substitution shouldn't be considered result ref",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(params.someParam)"),
		},
		want: false,
	}, {
		name: "test expression: one good ref, one bad one should return true",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTasks.results.sumResult) $(task.sumTasks.results.sumResult)"),
		},
		want: true,
	}, {
		name: "test expression: inside array parameter",
		param: v1beta1.Param{
			Name:  "param",
			Value: *v1beta1.NewStructuredValues("$(tasks.sumTask.results.sumResult)", "$(tasks.sumTask2.results.sumResult2)"),
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
					t.Error(diff.PrintWantGot(d))
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
				t.Errorf(diff.PrintWantGot(d))
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

// TestPipelineTaskResultReferences tests that PipelineTaskResultRefs()
// parses all the result variables used in a PipelineTask correctly and
// returns them all in the expected order.
func TestPipelineTaskResultRefs(t *testing.T) {
	pt := v1beta1.PipelineTask{
		Params: v1beta1.Params{{
			Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.r1)"),
		}, {
			Value: *v1beta1.NewStructuredValues("$(tasks.pt2.results.r2)"),
		}},
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "$(tasks.pt3.results.r3)",
			Operator: selection.In,
			Values: []string{
				"$(tasks.pt4.results.r4)",
			},
		}},
		Matrix: &v1beta1.Matrix{
			Include: []v1beta1.IncludeParams{{
				Name: "build-1",
				Params: v1beta1.Params{{
					Name: "a-param", Value: *v1beta1.NewStructuredValues("$(tasks.pt9.results.r9)"),
				}},
			}},
			Params: v1beta1.Params{{
				Value: *v1beta1.NewStructuredValues("$(tasks.pt5.results.r5)", "$(tasks.pt6.results.r6)"),
			}, {
				Value: *v1beta1.NewStructuredValues("$(tasks.pt7.results.r7)", "$(tasks.pt8.results.r8)"),
			}}},
	}
	refs := v1beta1.PipelineTaskResultRefs(&pt)
	expectedRefs := []*v1beta1.ResultRef{{
		PipelineTask: "pt1",
		Result:       "r1",
	}, {
		PipelineTask: "pt2",
		Result:       "r2",
	}, {
		PipelineTask: "pt3",
		Result:       "r3",
	}, {
		PipelineTask: "pt4",
		Result:       "r4",
	}, {
		PipelineTask: "pt5",
		Result:       "r5",
	}, {
		PipelineTask: "pt6",
		Result:       "r6",
	}, {
		PipelineTask: "pt7",
		Result:       "r7",
	}, {
		PipelineTask: "pt8",
		Result:       "r8",
	}, {
		PipelineTask: "pt9",
		Result:       "r9",
	}}
	if d := cmp.Diff(refs, expectedRefs, cmpopts.SortSlices(lessResultRef)); d != "" {
		t.Errorf("%v", d)
	}
}

func lessResultRef(i, j *v1beta1.ResultRef) bool {
	return i.PipelineTask+i.Result < j.PipelineTask+i.Result
}

func TestParseResultName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{{
		name:  "array indexing",
		input: "anArrayResult[1]",
		want:  []string{"anArrayResult", "1"},
	},
		{
			name:  "array star reference",
			input: "anArrayResult[*]",
			want:  []string{"anArrayResult", "*"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultName, idx := v1beta1.ParseResultName(tt.input)
			if d := cmp.Diff(tt.want, []string{resultName, idx}); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetVarSubstitutionExpressionsForPipelineResult(t *testing.T) {
	tests := []struct {
		name   string
		result v1beta1.PipelineResult
		want   []string
	}{{
		name: "get string result expressions",
		result: v1beta1.PipelineResult{
			Name:  "string result",
			Type:  v1beta1.ResultsTypeString,
			Value: *v1beta1.NewStructuredValues("$(tasks.task1.results.result1) and $(tasks.task2.results.result2)"),
		},
		want: []string{"tasks.task1.results.result1", "tasks.task2.results.result2"},
	}, {
		name: "get array result expressions",
		result: v1beta1.PipelineResult{
			Name:  "array result",
			Type:  v1beta1.ResultsTypeString,
			Value: *v1beta1.NewStructuredValues("$(tasks.task1.results.result1)", "$(tasks.task2.results.result2)"),
		},
		want: []string{"tasks.task1.results.result1", "tasks.task2.results.result2"},
	}, {
		name: "get object result expressions",
		result: v1beta1.PipelineResult{
			Name: "object result",
			Type: v1beta1.ResultsTypeString,
			Value: *v1beta1.NewObject(map[string]string{
				"key1": "$(tasks.task1.results.result1)",
				"key2": "$(tasks.task2.results.result2) and another one $(tasks.task3.results.result3)",
				"key3": "no ref here",
			}),
		},
		want: []string{"tasks.task1.results.result1", "tasks.task2.results.result2", "tasks.task3.results.result3"},
	},
	}
	var sortStrings = func(x, y string) bool {
		return x < y
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			get, _ := v1beta1.GetVarSubstitutionExpressionsForPipelineResult(tt.result)
			if d := cmp.Diff(tt.want, get, cmpopts.SortSlices(sortStrings)); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
