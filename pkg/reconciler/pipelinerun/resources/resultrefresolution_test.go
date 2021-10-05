package resources

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	successCondition = apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	}
	failedCondition = apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionFalse,
	}
)

var pipelineRunState = PipelineRunState{{
	TaskRunName: "aTaskRun",
	TaskRun: &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "aTaskRun",
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "aResult",
					Value: "aResultValue",
				}},
			},
		},
	},
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "aTask",
		TaskRef: &v1beta1.TaskRef{Name: "aTask"},
	},
}, {
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1beta1.TaskRef{Name: "bTask"},
		Params: []v1beta1.Param{{
			Name:  "bParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult)"),
		}},
	},
}, {
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1beta1.TaskRef{Name: "bTask"},
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "$(tasks.aTask.results.aResult)",
			Operator: selection.In,
			Values:   []string{"$(tasks.aTask.results.aResult)"},
		}},
	},
}, {
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1beta1.TaskRef{Name: "bTask"},
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "$(tasks.aTask.results.missingResult)",
			Operator: selection.In,
			Values:   []string{"$(tasks.aTask.results.missingResult)"},
		}},
	},
}, {
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1beta1.TaskRef{Name: "bTask"},
		Params: []v1beta1.Param{{
			Name:  "bParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.missingResult)"),
		}},
	},
}, {
	CustomTask: true,
	RunName:    "aRun",
	Run: &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{Name: "aRun"},
		Status: v1alpha1.RunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{successCondition},
			},
			RunStatusFields: v1alpha1.RunStatusFields{
				Results: []v1alpha1.RunResult{{
					Name:  "aResult",
					Value: "aResultValue",
				}},
			},
		},
	},
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "aCustomPipelineTask",
		TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "aTask"},
	},
}, {
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1beta1.TaskRef{Name: "bTask"},
		Params: []v1beta1.Param{{
			Name:  "bParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aCustomPipelineTask.results.aResult)"),
		}},
	},
}}

func TestTaskParamResolver_ResolveResultRefs(t *testing.T) {

	for _, tt := range []struct {
		name             string
		pipelineRunState PipelineRunState
		param            v1beta1.Param
		want             ResolvedResultRefs
		wantErr          bool
	}{{
		name: "successful resolution: param not using result reference",
		pipelineRunState: PipelineRunState{{
			TaskRunName: "aTaskRun",
			TaskRun:     &v1beta1.TaskRun{ObjectMeta: metav1.ObjectMeta{Name: "aTaskRun"}},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aTask",
				TaskRef: &v1beta1.TaskRef{Name: "aTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("explicitValueNoResultReference"),
		},
		want:    nil,
		wantErr: false,
	}, {
		name: "successful resolution: using result reference",
		pipelineRunState: PipelineRunState{{
			TaskRunName: "aTaskRun",
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "aTaskRun"},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						TaskRunResults: []v1beta1.TaskRunResult{{
							Name:  "aResult",
							Value: "aResultValue",
						}},
					},
				},
			},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aTask",
				TaskRef: &v1beta1.TaskRef{Name: "aTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult)"),
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: false,
	}, {
		name: "successful resolution: using multiple result reference",
		pipelineRunState: PipelineRunState{{
			TaskRunName: "aTaskRun",
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "aTaskRun"},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						TaskRunResults: []v1beta1.TaskRunResult{{
							Name:  "aResult",
							Value: "aResultValue",
						}},
					},
				},
			},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aTask",
				TaskRef: &v1beta1.TaskRef{Name: "aTask"},
			},
		}, {
			TaskRunName: "bTaskRun",
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "bTaskRun"},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						TaskRunResults: []v1beta1.TaskRunResult{{
							Name:  "bResult",
							Value: "bResultValue",
						}},
					},
				},
			},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1beta1.TaskRef{Name: "bTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult) $(tasks.bTask.results.bResult)"),
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}, {
			Value: *v1beta1.NewArrayOrString("bResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "bTask",
				Result:       "bResult",
			},
			FromTaskRun: "bTaskRun",
		}},
		wantErr: false,
	}, {
		name: "successful resolution: duplicate result references",
		pipelineRunState: PipelineRunState{{
			TaskRunName: "aTaskRun",
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "aTaskRun"},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						TaskRunResults: []v1beta1.TaskRunResult{{
							Name:  "aResult",
							Value: "aResultValue",
						}},
					},
				},
			},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aTask",
				TaskRef: &v1beta1.TaskRef{Name: "aTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult) $(tasks.aTask.results.aResult)"),
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: false,
	}, {
		name: "unsuccessful resolution: referenced result doesn't exist in referenced task",
		pipelineRunState: PipelineRunState{{
			TaskRunName: "aTaskRun",
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "aTaskRun"},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{successCondition},
					},
				},
			},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aTask",
				TaskRef: &v1beta1.TaskRef{Name: "aTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult)"),
		},
		want:    nil,
		wantErr: true,
	}, {
		name:             "unsuccessful resolution: pipeline task missing",
		pipelineRunState: PipelineRunState{},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult)"),
		},
		want:    nil,
		wantErr: true,
	}, {
		name: "unsuccessful resolution: task run missing",
		pipelineRunState: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aTask",
				TaskRef: &v1beta1.TaskRef{Name: "aTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult)"),
		},
		want:    nil,
		wantErr: true,
	}, {
		name: "failed resolution: using result reference to a failed task",
		pipelineRunState: PipelineRunState{{
			TaskRunName: "aTaskRun",
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "aTaskRun"},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{failedCondition},
					},
				},
			},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aTask",
				TaskRef: &v1beta1.TaskRef{Name: "aTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aTask.results.aResult)"),
		},
		want:    nil,
		wantErr: true,
	}, {
		name: "successful resolution: using result reference to a Run",
		pipelineRunState: PipelineRunState{{
			CustomTask: true,
			RunName:    "aRun",
			Run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{Name: "aRun"},
				Status: v1alpha1.RunStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{successCondition},
					},
					RunStatusFields: v1alpha1.RunStatusFields{
						Results: []v1alpha1.RunResult{{
							Name:  "aResult",
							Value: "aResultValue",
						}},
					},
				},
			},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aCustomPipelineTask",
				TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "aTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aCustomPipelineTask.results.aResult)"),
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aCustomPipelineTask",
				Result:       "aResult",
			},
			FromRun: "aRun",
		}},
		wantErr: false,
	}, {
		name: "failed resolution: using result reference to a failed Run",
		pipelineRunState: PipelineRunState{{
			CustomTask: true,
			RunName:    "aRun",
			Run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{Name: "aRun"},
				Status: v1alpha1.RunStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{failedCondition},
					},
				},
			},
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "aCustomPipelineTask",
				TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "aTask"},
			},
		}},
		param: v1beta1.Param{
			Name:  "targetParam",
			Value: *v1beta1.NewArrayOrString("$(tasks.aCustomPipelineTask.results.aResult)"),
		},
		want:    nil,
		wantErr: true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("test name: %s\n", tt.name)
			got, err := extractResultRefsForParam(tt.pipelineRunState, tt.param)
			// sort result ref based on task name to guarantee an certain order
			sort.SliceStable(got, func(i, j int) bool {
				fromI := got[i].FromTaskRun
				if fromI == "" {
					fromI = got[i].FromRun
				}
				fromJ := got[j].FromTaskRun
				if fromJ == "" {
					fromJ = got[j].FromRun
				}
				return strings.Compare(fromI, fromJ) < 0
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveResultRef() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(tt.want) != len(got) {
				t.Fatalf("incorrect number of refs, want %d, got %d", len(tt.want), len(got))
			}
			for _, rGot := range got {
				foundMatch := false
				for _, rWant := range tt.want {
					if d := cmp.Diff(rGot, rWant); d == "" {
						foundMatch = true
					}
				}
				if !foundMatch {
					t.Fatalf("Expected resolved refs:\n%s\n\nbut received:\n%s\n", resolvedSliceAsString(tt.want), resolvedSliceAsString(got))
				}
			}
		})
	}
}

func resolvedSliceAsString(rs []*ResolvedResultRef) string {
	var s []string
	for _, r := range rs {
		s = append(s, fmt.Sprintf("%#v", *r))
	}
	return fmt.Sprintf("[\n%s\n]", strings.Join(s, ",\n"))
}

func TestResolveResultRefs(t *testing.T) {
	for _, tt := range []struct {
		name             string
		pipelineRunState PipelineRunState
		targets          PipelineRunState
		want             ResolvedResultRefs
		wantErr          bool
		wantPt           string
	}{{
		name:             "Test successful result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[1],
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful result references resolution - when expressions",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[2],
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful result references resolution non result references",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[0],
		},
		want:    nil,
		wantErr: false,
	}, {
		name:             "Test unsuccessful result references resolution - when expression",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[3],
		},
		want:    nil,
		wantErr: true,
		wantPt:  "aTask",
	}, {
		name:             "Test unsuccessful result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[4],
		},
		want:    nil,
		wantErr: true,
		wantPt:  "aTask",
	}, {
		name:             "Test successful result references resolution - params - Run",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[6],
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aCustomPipelineTask",
				Result:       "aResult",
			},
			FromRun: "aRun",
		}},
		wantErr: false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got, pt, err := ResolveResultRefs(tt.pipelineRunState, tt.targets)
			sort.SliceStable(got, func(i, j int) bool {
				fromI := got[i].FromTaskRun
				if fromI == "" {
					fromI = got[i].FromRun
				}
				fromJ := got[j].FromTaskRun
				if fromJ == "" {
					fromJ = got[j].FromRun
				}
				return strings.Compare(fromI, fromJ) < 0
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveResultRefs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Fatalf("ResolveResultRef %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tt.wantPt, pt); d != "" {
				t.Fatalf("ResolvedPipelineRunTask %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolveResultRef(t *testing.T) {
	for _, tt := range []struct {
		name             string
		pipelineRunState PipelineRunState
		target           *ResolvedPipelineRunTask
		want             ResolvedResultRefs
		wantErr          bool
		wantPt           string
	}{{
		name:             "Test successful result references resolution - params",
		pipelineRunState: pipelineRunState,
		target:           pipelineRunState[1],
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful result references resolution - when expressions",
		pipelineRunState: pipelineRunState,
		target:           pipelineRunState[2],
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful result references resolution non result references",
		pipelineRunState: pipelineRunState,
		target:           pipelineRunState[0],
		want:             nil,
		wantErr:          false,
	}, {
		name:             "Test unsuccessful result references resolution - when expression",
		pipelineRunState: pipelineRunState,
		target:           pipelineRunState[3],
		want:             nil,
		wantErr:          true,
		wantPt:           "aTask",
	}, {
		name:             "Test unsuccessful result references resolution - params",
		pipelineRunState: pipelineRunState,
		target:           pipelineRunState[4],
		want:             nil,
		wantErr:          true,
		wantPt:           "aTask",
	}, {
		name:             "Test successful result references resolution - params - Run",
		pipelineRunState: pipelineRunState,
		target:           pipelineRunState[6],
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewArrayOrString("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aCustomPipelineTask",
				Result:       "aResult",
			},
			FromRun: "aRun",
		}},
		wantErr: false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got, pt, err := ResolveResultRef(tt.pipelineRunState, tt.target)
			sort.SliceStable(got, func(i, j int) bool {
				fromI := got[i].FromTaskRun
				if fromI == "" {
					fromI = got[i].FromRun
				}
				fromJ := got[j].FromTaskRun
				if fromJ == "" {
					fromJ = got[j].FromRun
				}
				return strings.Compare(fromI, fromJ) < 0
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveResultRefs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Fatalf("ResolveResultRef %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tt.wantPt, pt); d != "" {
				t.Fatalf("ResolvedPipelineRunTask %s", diff.PrintWantGot(d))
			}
		})
	}
}
