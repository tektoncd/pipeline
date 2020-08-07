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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestApplyParameters(t *testing.T) {
	tests := []struct {
		name     string
		original *v1beta1.Pipeline
		run      *v1beta1.PipelineRun
		expected *v1beta1.Pipeline
	}{{
		name: "single parameter",
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeString),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(params.first-param)"),
					tb.PipelineTaskParam("first-task-second-param", "$(params.second-param)"),
					tb.PipelineTaskParam("first-task-third-param", "static value"),
				))),
		run: tb.PipelineRun("test-pipeline-run",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunParam("second-param", "second-value"))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeString),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "default-value"),
					tb.PipelineTaskParam("first-task-second-param", "second-value"),
					tb.PipelineTaskParam("first-task-third-param", "static value"),
				))),
	}, {
		name: "pipeline parameter nested inside task parameter",
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(input.workspace.$(params.first-param))"),
					tb.PipelineTaskParam("first-task-second-param", "$(input.workspace.$(params.second-param))"),
				))),
		run: tb.PipelineRun("test-pipeline-run",
			tb.PipelineRunSpec("test-pipeline")),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(input.workspace.default-value)"),
					tb.PipelineTaskParam("first-task-second-param", "$(input.workspace.default-value)"),
				))),
	}, {
		name: "parameters in task condition",
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeString),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskCondition("task-condition",
						tb.PipelineTaskConditionParam("cond-first-param", "$(params.first-param)"),
						tb.PipelineTaskConditionParam("cond-second-param", "$(params.second-param)"),
					),
				))),
		run: tb.PipelineRun("test-pipeline-run",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunParam("second-param", "second-value"))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeString),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskCondition("task-condition",
						tb.PipelineTaskConditionParam("cond-first-param", "default-value"),
						tb.PipelineTaskConditionParam("cond-second-param", "second-value"),
					),
				))),
	}, {
		name: "array parameter",
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeArray, tb.ParamSpecDefault(
					"default", "array", "value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeArray),
				tb.PipelineParamSpec("fourth-param", v1beta1.ParamTypeArray),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "firstelement", "$(params.first-param)"),
					tb.PipelineTaskParam("first-task-second-param", "first", "$(params.second-param)"),
					tb.PipelineTaskParam("first-task-third-param", "static value"),
					tb.PipelineTaskParam("first-task-fourth-param", "first", "$(params.fourth-param)"),
				))),
		run: tb.PipelineRun("test-pipeline-run",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunParam("second-param", "second-value", "array"),
				tb.PipelineRunParam("fourth-param", "fourth-value", "array"))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeArray, tb.ParamSpecDefault(
					"default", "array", "value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeArray),
				tb.PipelineParamSpec("fourth-param", v1beta1.ParamTypeArray),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "firstelement", "default", "array", "value"),
					tb.PipelineTaskParam("first-task-second-param", "first", "second-value", "array"),
					tb.PipelineTaskParam("first-task-third-param", "static value"),
					tb.PipelineTaskParam("first-task-fourth-param", "first", "fourth-value", "array"),
				))),
	}, {
		name: "parameter evaluation with final tasks",
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeString),
				tb.FinalPipelineTask("final-task-1", "final-task",
					tb.PipelineTaskParam("final-task-first-param", "$(params.first-param)"),
					tb.PipelineTaskParam("final-task-second-param", "$(params.second-param)"),
				))),
		run: tb.PipelineRun("test-pipeline-run",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunParam("second-param", "second-value"))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1beta1.ParamTypeString),
				tb.FinalPipelineTask("final-task-1", "final-task",
					tb.PipelineTaskParam("final-task-first-param", "default-value"),
					tb.PipelineTaskParam("final-task-second-param", "second-value"),
				))),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyParameters(&tt.original.Spec, tt.run)
			if d := cmp.Diff(&tt.expected.Spec, got); d != "" {
				t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResults_MinimalExpression(t *testing.T) {
	type args struct {
		targets            PipelineRunState
		resolvedResultRefs ResolvedResultRefs
	}
	tests := []struct {
		name string
		args args
		want PipelineRunState
	}{
		{
			name: "Test result substitution on minimal variable substitution expression",
			args: args{
				resolvedResultRefs: ResolvedResultRefs{
					{
						Value: v1beta1.ArrayOrString{
							Type:      v1beta1.ParamTypeString,
							StringVal: "aResultValue",
						},
						ResultReference: v1beta1.ResultRef{
							PipelineTask: "aTask",
							Result:       "aResult",
						},
						FromTaskRun: "aTaskRun",
					},
				},
				targets: PipelineRunState{
					{
						PipelineTask: &v1beta1.PipelineTask{
							Name:    "bTask",
							TaskRef: &v1beta1.TaskRef{Name: "bTask"},
							Params: []v1beta1.Param{
								{
									Name: "bParam",
									Value: v1beta1.ArrayOrString{
										Type:      v1beta1.ParamTypeString,
										StringVal: "$(tasks.aTask.results.aResult)",
									},
								},
							},
						},
					},
				},
			},
			want: PipelineRunState{
				{
					PipelineTask: &v1beta1.PipelineTask{
						Name:    "bTask",
						TaskRef: &v1beta1.TaskRef{Name: "bTask"},
						Params: []v1beta1.Param{
							{
								Name: "bParam",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: "aResultValue",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplyTaskResults(tt.args.targets, tt.args.resolvedResultRefs)
			if d := cmp.Diff(tt.want, tt.args.targets); d != "" {
				t.Fatalf("ApplyTaskResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResults_EmbeddedExpression(t *testing.T) {
	type args struct {
		targets            PipelineRunState
		resolvedResultRefs ResolvedResultRefs
	}
	tests := []struct {
		name string
		args args
		want PipelineRunState
	}{
		{
			name: "Test result substitution on embedded variable substitution expression",
			args: args{
				resolvedResultRefs: ResolvedResultRefs{
					{
						Value: v1beta1.ArrayOrString{
							Type:      v1beta1.ParamTypeString,
							StringVal: "aResultValue",
						},
						ResultReference: v1beta1.ResultRef{
							PipelineTask: "aTask",
							Result:       "aResult",
						},
						FromTaskRun: "aTaskRun",
					},
				},
				targets: PipelineRunState{
					{
						PipelineTask: &v1beta1.PipelineTask{
							Name:    "bTask",
							TaskRef: &v1beta1.TaskRef{Name: "bTask"},
							Params: []v1beta1.Param{
								{
									Name: "bParam",
									Value: v1beta1.ArrayOrString{
										Type:      v1beta1.ParamTypeString,
										StringVal: "Result value --> $(tasks.aTask.results.aResult)",
									},
								},
							},
						},
					},
				},
			},
			want: PipelineRunState{
				{
					PipelineTask: &v1beta1.PipelineTask{
						Name:    "bTask",
						TaskRef: &v1beta1.TaskRef{Name: "bTask"},
						Params: []v1beta1.Param{
							{
								Name: "bParam",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: "Result value --> aResultValue",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplyTaskResults(tt.args.targets, tt.args.resolvedResultRefs)
			if d := cmp.Diff(tt.want, tt.args.targets); d != "" {
				t.Fatalf("ApplyTaskResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResults_Conditions(t *testing.T) {
	type args struct {
		targets            PipelineRunState
		resolvedResultRefs ResolvedResultRefs
	}
	tests := []struct {
		name string
		args args
		want PipelineRunState
	}{{
		name: "Test result substitution in condition parameter",
		args: args{
			resolvedResultRefs: ResolvedResultRefs{
				{
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "aResultValue",
					},
					ResultReference: v1beta1.ResultRef{
						PipelineTask: "aTask",
						Result:       "aResult",
					},
					FromTaskRun: "aTaskRun",
				},
			},
			targets: PipelineRunState{
				{
					ResolvedConditionChecks: TaskConditionCheckState{{
						ConditionRegisterName: "always-true-0",
						ConditionCheckName:    "test",
						Condition: &v1alpha1.Condition{
							ObjectMeta: metav1.ObjectMeta{
								Name: "always-true",
							},
							Spec: v1alpha1.ConditionSpec{
								Check: v1beta1.Step{},
							}},
						ResolvedResources: map[string]*resourcev1alpha1.PipelineResource{},
						PipelineTaskCondition: &v1beta1.PipelineTaskCondition{
							Params: []v1beta1.Param{
								{
									Name: "cParam",
									Value: v1beta1.ArrayOrString{
										Type:      v1beta1.ParamTypeString,
										StringVal: "Result value --> $(tasks.aTask.results.aResult)",
									},
								},
							},
						},
					},
					},
				},
			},
		},
		want: PipelineRunState{
			{
				ResolvedConditionChecks: TaskConditionCheckState{{
					ConditionRegisterName: "always-true-0",
					ConditionCheckName:    "test",
					Condition: &v1alpha1.Condition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "always-true",
						},
						Spec: v1alpha1.ConditionSpec{
							Check: v1beta1.Step{},
						}},
					ResolvedResources: map[string]*resourcev1alpha1.PipelineResource{},
					PipelineTaskCondition: &v1beta1.PipelineTaskCondition{
						Params: []v1beta1.Param{
							{
								Name: "cParam",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: "Result value --> aResultValue",
								},
							},
						},
					},
				},
				},
			},
		},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplyTaskResults(tt.args.targets, tt.args.resolvedResultRefs)
			if d := cmp.Diff(tt.want[0].ResolvedConditionChecks, tt.args.targets[0].ResolvedConditionChecks, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{}, ResolvedConditionCheck{})); d != "" {
				t.Fatalf("ApplyTaskResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestContext(t *testing.T) {
	for _, tc := range []struct {
		description string
		pr          *v1beta1.PipelineRun
		original    *v1beta1.Pipeline
		expected    *v1beta1.Pipeline
	}{{
		description: "context pipeline name replacement without pipelineRun in spec",
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(context.pipeline.name)-1"),
				))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "test-pipeline-1"),
				))),
		pr: &v1beta1.PipelineRun{},
	}, {
		description: "context pipeline name replacement with pipelineRun in spec",
		pr:          tb.PipelineRun("pipelineRunName"),
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(context.pipeline.name)-1"),
				))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "test-pipeline-1"),
				))),
	}, {
		description: "context pipelineRunName replacement with defined pipelineRun in spec",
		pr:          tb.PipelineRun("pipelineRunName"),
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(context.pipelineRun.name)-1"),
				))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "pipelineRunName-1"),
				))),
	}, {
		description: "context pipelineRunNameNamespace replacement with defined pipelineRunNamepsace in spec",
		pr:          tb.PipelineRun("pipelineRunName", tb.PipelineRunNamespace("prns")),
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(context.pipelineRun.namespace)-1"),
				))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "prns-1"),
				))),
	}, {
		description: "context pipelineRunName replacement with no defined pipeline in spec",
		pr:          &v1beta1.PipelineRun{},
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(context.pipelineRun.name)-1"),
				))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "-1"),
				))),
	}, {
		description: "context pipelineRunNamespace replacement with no defined pipelineRunNamespace in spec",
		pr:          tb.PipelineRun("pipelineRunName"),
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(context.pipelineRun.namespace)-1"),
				))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "-1"),
				))),
	}, {
		description: "context pipeline name replacement with pipelinerun uid",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				UID: "UID-1",
			},
		},
		original: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(context.pipelineRun.uid)"),
				))),
		expected: tb.Pipeline("test-pipeline",
			tb.PipelineSpec(
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "UID-1"),
				))),
	}} {
		t.Run(tc.description, func(t *testing.T) {
			got := ApplyContexts(&tc.original.Spec, tc.original.Name, tc.pr)
			if d := cmp.Diff(tc.expected.Spec, *got); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}
