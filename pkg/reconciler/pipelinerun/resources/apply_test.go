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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
)

func TestApplyParameters(t *testing.T) {
	for _, tt := range []struct {
		name     string
		original v1beta1.PipelineSpec
		params   []v1beta1.Param
		expected v1beta1.PipelineSpec
	}{{
		name: "single parameter",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewArrayOrString("$(params.first-param)")},
					{Name: "first-task-second-param", Value: *v1beta1.NewArrayOrString("$(params.second-param)")},
					{Name: "first-task-third-param", Value: *v1beta1.NewArrayOrString("static value")},
				},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewArrayOrString("second-value")}},
		expected: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewArrayOrString("default-value")},
					{Name: "first-task-second-param", Value: *v1beta1.NewArrayOrString("second-value")},
					{Name: "first-task-third-param", Value: *v1beta1.NewArrayOrString("static value")},
				},
			}},
		},
	}, {
		name: "single parameter with when expression",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "$(params.first-param)",
					Operator: selection.In,
					Values:   []string{"$(params.second-param)"},
				}},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewArrayOrString("second-value")}},
		expected: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "default-value",
					Operator: selection.In,
					Values:   []string{"second-value"},
				}},
			}},
		},
	}, {
		name: "pipeline parameter nested inside task parameter",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewArrayOrString("$(input.workspace.$(params.first-param))")},
					{Name: "first-task-second-param", Value: *v1beta1.NewArrayOrString("$(input.workspace.$(params.second-param))")},
				},
			}},
		},
		params: nil, // no parameter values.
		expected: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewArrayOrString("$(input.workspace.default-value)")},
					{Name: "first-task-second-param", Value: *v1beta1.NewArrayOrString("$(input.workspace.default-value)")},
				},
			}},
		},
	}, {
		name: "parameters in task condition",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				Conditions: []v1beta1.PipelineTaskCondition{{
					Params: []v1beta1.Param{
						{Name: "cond-first-param", Value: *v1beta1.NewArrayOrString("$(params.first-param)")},
						{Name: "cond-second-param", Value: *v1beta1.NewArrayOrString("$(params.second-param)")},
					},
				}},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewArrayOrString("second-value")}},
		expected: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				Conditions: []v1beta1.PipelineTaskCondition{{
					Params: []v1beta1.Param{
						{Name: "cond-first-param", Value: *v1beta1.NewArrayOrString("default-value")},
						{Name: "cond-second-param", Value: *v1beta1.NewArrayOrString("second-value")},
					},
				}},
			}},
		},
	}, {
		name: "array parameter",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewArrayOrString("default", "array", "value")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewArrayOrString("firstelement", "$(params.first-param)")},
					{Name: "first-task-second-param", Value: *v1beta1.NewArrayOrString("firstelement", "$(params.second-param)")},
				},
			}},
		},
		params: []v1beta1.Param{
			{Name: "second-param", Value: *v1beta1.NewArrayOrString("second-value", "array")},
		},
		expected: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeArray, Default: v1beta1.NewArrayOrString("default", "array", "value")},
				{Name: "second-param", Type: v1beta1.ParamTypeArray},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "first-task-first-param", Value: *v1beta1.NewArrayOrString("firstelement", "default", "array", "value")},
					{Name: "first-task-second-param", Value: *v1beta1.NewArrayOrString("firstelement", "second-value", "array")},
				},
			}},
		},
	}, {
		name: "parameter evaluation with final tasks",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Finally: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewArrayOrString("$(params.first-param)")},
					{Name: "final-task-second-param", Value: *v1beta1.NewArrayOrString("$(params.second-param)")},
				},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewArrayOrString("second-value")}},
		expected: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Finally: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewArrayOrString("default-value")},
					{Name: "final-task-second-param", Value: *v1beta1.NewArrayOrString("second-value")},
				},
			}},
		},
	}, {
		name: "parameter evaluation with both tasks and final tasks",
		original: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewArrayOrString("$(params.first-param)")},
					{Name: "final-task-second-param", Value: *v1beta1.NewArrayOrString("$(params.second-param)")},
				},
			}},
			Finally: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewArrayOrString("$(params.first-param)")},
					{Name: "final-task-second-param", Value: *v1beta1.NewArrayOrString("$(params.second-param)")},
				},
			}},
		},
		params: []v1beta1.Param{{Name: "second-param", Value: *v1beta1.NewArrayOrString("second-value")}},
		expected: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{
				{Name: "first-param", Type: v1beta1.ParamTypeString, Default: v1beta1.NewArrayOrString("default-value")},
				{Name: "second-param", Type: v1beta1.ParamTypeString},
			},
			Tasks: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewArrayOrString("default-value")},
					{Name: "final-task-second-param", Value: *v1beta1.NewArrayOrString("second-value")},
				},
			}},
			Finally: []v1beta1.PipelineTask{{
				Params: []v1beta1.Param{
					{Name: "final-task-first-param", Value: *v1beta1.NewArrayOrString("default-value")},
					{Name: "final-task-second-param", Value: *v1beta1.NewArrayOrString("second-value")},
				},
			}},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := &v1beta1.PipelineRun{
				Spec: v1beta1.PipelineRunSpec{
					Params: tt.params,
				},
			}
			got := ApplyParameters(&tt.original, run)
			if d := cmp.Diff(&tt.expected, got); d != "" {
				t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResults_MinimalExpression(t *testing.T) {
	for _, tt := range []struct {
		name               string
		targets            PipelineRunState
		resolvedResultRefs ResolvedResultRefs
		want               PipelineRunState
	}{{
		name: "Test result substitution on minimal variable substitution expression - params",
		resolvedResultRefs: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
				Params: []v1beta1.Param{{
					Name:  "bParam",
					Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult)"),
				}},
			},
		}},
		want: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
				Params: []v1beta1.Param{{
					Name:  "bParam",
					Value: *v1beta1.NewArrayOrString("aResultValue"),
				}},
			},
		}},
	}, {
		name: "Test result substitution on minimal variable substitution expression - when expressions",
		resolvedResultRefs: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "$(tasks.aTask.results.aResult)",
					Operator: selection.In,
					Values:   []string{"$(tasks.aTask.results.aResult)"},
				}},
			},
		}},
		want: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "aResultValue",
					Operator: selection.In,
					Values:   []string{"aResultValue"},
				}},
			},
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			ApplyTaskResults(tt.targets, tt.resolvedResultRefs)
			if d := cmp.Diff(tt.want, tt.targets); d != "" {
				t.Fatalf("ApplyTaskResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResults_EmbeddedExpression(t *testing.T) {
	for _, tt := range []struct {
		name               string
		targets            PipelineRunState
		resolvedResultRefs ResolvedResultRefs
		want               PipelineRunState
	}{{
		name: "Test result substitution on embedded variable substitution expression - params",
		resolvedResultRefs: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
				Params: []v1beta1.Param{{
					Name:  "bParam",
					Value: *v1beta1.NewArrayOrString("Result value --> $(tasks.aTask.results.aResult)"),
				}},
			},
		}},
		want: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
				Params: []v1beta1.Param{{
					Name:  "bParam",
					Value: *v1beta1.NewArrayOrString("Result value --> aResultValue"),
				}},
			},
		}},
	}, {
		name: "Test result substitution on embedded variable substitution expression - when expressions",
		resolvedResultRefs: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
				WhenExpressions: []v1beta1.WhenExpression{
					{
						Input:    "Result value --> $(tasks.aTask.results.aResult)",
						Operator: selection.In,
						Values:   []string{"Result value --> $(tasks.aTask.results.aResult)"},
					},
				},
			},
		}},
		want: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "Result value --> aResultValue",
					Operator: selection.In,
					Values:   []string{"Result value --> aResultValue"},
				}},
			},
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			ApplyTaskResults(tt.targets, tt.resolvedResultRefs)
			if d := cmp.Diff(tt.want, tt.targets); d != "" {
				t.Fatalf("ApplyTaskResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResults_Conditions(t *testing.T) {
	for _, tt := range []struct {
		name               string
		targets            PipelineRunState
		resolvedResultRefs ResolvedResultRefs
		want               PipelineRunState
	}{{
		name: "Test result substitution in condition parameter",
		resolvedResultRefs: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: PipelineRunState{{
			ResolvedConditionChecks: TaskConditionCheckState{{
				ConditionRegisterName: "always-true-0",
				ConditionCheckName:    "test",
				Condition: &v1alpha1.Condition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "always-true",
					},
					Spec: v1alpha1.ConditionSpec{
						Check: v1beta1.Step{},
					},
				},
				ResolvedResources: map[string]*resourcev1alpha1.PipelineResource{},
				PipelineTaskCondition: &v1beta1.PipelineTaskCondition{
					Params: []v1beta1.Param{{
						Name:  "cParam",
						Value: *v1beta1.NewArrayOrString("Result value --> $(tasks.aTask.results.aResult)"),
					}},
				},
			}},
		}},
		want: PipelineRunState{{
			ResolvedConditionChecks: TaskConditionCheckState{{
				ConditionRegisterName: "always-true-0",
				ConditionCheckName:    "test",
				Condition: &v1alpha1.Condition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "always-true",
					},
					Spec: v1alpha1.ConditionSpec{
						Check: v1beta1.Step{},
					},
				},
				ResolvedResources: map[string]*resourcev1alpha1.PipelineResource{},
				PipelineTaskCondition: &v1beta1.PipelineTaskCondition{Params: []v1beta1.Param{{
					Name:  "cParam",
					Value: *v1beta1.NewArrayOrString("Result value --> aResultValue"),
				}}},
			}},
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			ApplyTaskResults(tt.targets, tt.resolvedResultRefs)
			if d := cmp.Diff(tt.want[0].ResolvedConditionChecks, tt.targets[0].ResolvedConditionChecks, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{}, ResolvedConditionCheck{})); d != "" {
				t.Fatalf("ApplyTaskResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestContext(t *testing.T) {
	for _, tc := range []struct {
		description string
		pr          *v1beta1.PipelineRun
		original    v1beta1.Param
		expected    v1beta1.Param
	}{{
		description: "context.pipeline.name defined",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "name"},
		},
		original: v1beta1.Param{Value: *v1beta1.NewArrayOrString("$(context.pipeline.name)-1")},
		expected: v1beta1.Param{Value: *v1beta1.NewArrayOrString("test-pipeline-1")},
	}, {
		description: "context.pipelineRun.name defined",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "name"},
		},
		original: v1beta1.Param{Value: *v1beta1.NewArrayOrString("$(context.pipelineRun.name)-1")},
		expected: v1beta1.Param{Value: *v1beta1.NewArrayOrString("name-1")},
	}, {
		description: "context.pipelineRun.name undefined",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: ""},
		},
		original: v1beta1.Param{Value: *v1beta1.NewArrayOrString("$(context.pipelineRun.name)-1")},
		expected: v1beta1.Param{Value: *v1beta1.NewArrayOrString("-1")},
	}, {
		description: "context.pipelineRun.namespace defined",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Namespace: "namespace"},
		},
		original: v1beta1.Param{Value: *v1beta1.NewArrayOrString("$(context.pipelineRun.namespace)-1")},
		expected: v1beta1.Param{Value: *v1beta1.NewArrayOrString("namespace-1")},
	}, {
		description: "context.pipelineRun.namespace undefined",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Namespace: ""},
		},
		original: v1beta1.Param{Value: *v1beta1.NewArrayOrString("$(context.pipelineRun.namespace)-1")},
		expected: v1beta1.Param{Value: *v1beta1.NewArrayOrString("-1")},
	}, {
		description: "context.pipelineRun.uid defined",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{UID: "UID"},
		},
		original: v1beta1.Param{Value: *v1beta1.NewArrayOrString("$(context.pipelineRun.uid)-1")},
		expected: v1beta1.Param{Value: *v1beta1.NewArrayOrString("UID-1")},
	}, {
		description: "context.pipelineRun.uid undefined",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{UID: ""},
		},
		original: v1beta1.Param{Value: *v1beta1.NewArrayOrString("$(context.pipelineRun.uid)-1")},
		expected: v1beta1.Param{Value: *v1beta1.NewArrayOrString("-1")},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			orig := &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline"},
				Spec: v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Params: []v1beta1.Param{tc.original},
					}},
				},
			}
			got := ApplyContexts(&orig.Spec, orig.Name, tc.pr)
			if d := cmp.Diff(tc.expected, got.Tasks[0].Params[0]); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		description         string
		declarations        []v1beta1.PipelineWorkspaceDeclaration
		bindings            []v1beta1.WorkspaceBinding
		variableUsage       string
		expectedReplacement string
	}{{
		description: "workspace declared and bound",
		declarations: []v1beta1.PipelineWorkspaceDeclaration{{
			Name: "foo",
		}},
		bindings: []v1beta1.WorkspaceBinding{{
			Name: "foo",
		}},
		variableUsage:       "$(workspaces.foo.bound)",
		expectedReplacement: "true",
	}, {
		description: "workspace declared not bound",
		declarations: []v1beta1.PipelineWorkspaceDeclaration{{
			Name:     "foo",
			Optional: true,
		}},
		bindings:            []v1beta1.WorkspaceBinding{},
		variableUsage:       "$(workspaces.foo.bound)",
		expectedReplacement: "false",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			p1 := v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Params: []v1beta1.Param{{Value: *v1beta1.NewArrayOrString(tc.variableUsage)}},
				}},
				Workspaces: tc.declarations,
			}
			pr := &v1beta1.PipelineRun{
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{
						Name: "test-pipeline",
					},
					Workspaces: tc.bindings,
				},
			}
			p2 := ApplyWorkspaces(&p1, pr)
			str := p2.Tasks[0].Params[0].Value.StringVal
			if str != tc.expectedReplacement {
				t.Errorf("expected %q, received %q", tc.expectedReplacement, str)
			}
		})
	}
}
