package resources

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
)

var pipelineRunState = PipelineRunState{{
	TaskRunName: "aTaskRun",
	TaskRun: &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "aTaskRun",
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "aResult",
					Value: *v1beta1.NewStructuredValues("aResultValue"),
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
		Params: v1beta1.Params{{
			Name:  "bParam",
			Value: *v1beta1.NewStructuredValues("$(tasks.aTask.results.aResult)"),
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
		Params: v1beta1.Params{{
			Name:  "bParam",
			Value: *v1beta1.NewStructuredValues("$(tasks.aTask.results.missingResult)"),
		}},
	},
}, {
	CustomTask:    true,
	RunObjectName: "aRun",
	RunObject: &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{Name: "aRun"},
		Status: v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{successCondition},
			},
			CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				Results: []v1beta1.CustomRunResult{{
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
		Params: v1beta1.Params{{
			Name:  "bParam",
			Value: *v1beta1.NewStructuredValues("$(tasks.aCustomPipelineTask.results.aResult)"),
		}},
	},
}, {
	TaskRunName: "cTaskRun",
	TaskRun: &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cTaskRun",
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "cResult",
					Value: *v1beta1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
				}},
			},
		},
	},
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "cTask",
		TaskRef: &v1beta1.TaskRef{Name: "cTask"},
		Params: v1beta1.Params{{
			Name:  "cParam",
			Value: *v1beta1.NewStructuredValues("$(tasks.cTask.results.cResult[1])"),
		}},
	},
}, {
	TaskRunName: "dTaskRun",
	TaskRun: &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dTaskRun",
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "dResult",
					Value: *v1beta1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
				}},
			},
		},
	},
	PipelineTask: &v1beta1.PipelineTask{
		Name:    "dTask",
		TaskRef: &v1beta1.TaskRef{Name: "dTask"},
		Params: v1beta1.Params{{
			Name:  "dParam",
			Value: *v1beta1.NewStructuredValues("$(tasks.dTask.results.dResult[3])"),
		}},
	},
}}

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
			Value: *v1beta1.NewStructuredValues("aResultValue"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful array result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[7],
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
			ResultReference: v1beta1.ResultRef{
				PipelineTask: "cTask",
				Result:       "cResult",
				ResultsIndex: 1,
			},
			FromTaskRun: "cTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test unsuccessful result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[8],
		},
		want:    nil,
		wantErr: true,
	}, {
		name:             "Test successful result references resolution - when expressions",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[2],
		},
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewStructuredValues("aResultValue"),
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
			Value: *v1beta1.NewStructuredValues("aResultValue"),
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
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveResultRefs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got, cmpopts.SortSlices(lessResolvedResultRefs)); d != "" {
				t.Fatalf("ResolveResultRef %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tt.wantPt, pt); d != "" {
				t.Fatalf("ResolvedPipelineTask %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolveResultRef(t *testing.T) {
	for _, tt := range []struct {
		name             string
		pipelineRunState PipelineRunState
		target           *ResolvedPipelineTask
		want             ResolvedResultRefs
		wantErr          bool
		wantPt           string
	}{{
		name:             "Test successful result references resolution - params",
		pipelineRunState: pipelineRunState,
		target:           pipelineRunState[1],
		want: ResolvedResultRefs{{
			Value: *v1beta1.NewStructuredValues("aResultValue"),
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
			Value: *v1beta1.NewStructuredValues("aResultValue"),
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
			Value: *v1beta1.NewStructuredValues("aResultValue"),
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
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveResultRefs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got, cmpopts.SortSlices(lessResolvedResultRefs)); d != "" {
				t.Fatalf("ResolveResultRef %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tt.wantPt, pt); d != "" {
				t.Fatalf("ResolvedPipelineTask %s", diff.PrintWantGot(d))
			}
		})
	}
}

func lessResolvedResultRefs(i, j *ResolvedResultRef) bool {
	fromI := i.FromTaskRun
	if fromI == "" {
		fromI = i.FromRun
	}
	fromJ := j.FromTaskRun
	if fromJ == "" {
		fromJ = j.FromRun
	}
	return strings.Compare(fromI, fromJ) < 0
}
