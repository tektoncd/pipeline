/*
Copyright 2023 The Tekton Authors

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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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
	TaskRunNames: []string{"aTaskRun"},
	TaskRuns: []*v1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "aTaskRun",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "aResult",
					Value: *v1.NewStructuredValues("aResultValue"),
				}},
			},
		},
	}},
	PipelineTask: &v1.PipelineTask{
		Name:    "aTask",
		TaskRef: &v1.TaskRef{Name: "aTask"},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1.TaskRef{Name: "bTask"},
		Params: []v1.Param{{
			Name:  "bParam",
			Value: *v1.NewStructuredValues("$(tasks.aTask.results.aResult)"),
		}},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1.TaskRef{Name: "bTask"},
		When: []v1.WhenExpression{{
			Input:    "$(tasks.aTask.results.aResult)",
			Operator: selection.In,
			Values:   []string{"$(tasks.aTask.results.aResult)"},
		}},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1.TaskRef{Name: "bTask"},
		When: []v1.WhenExpression{{
			Input:    "$(tasks.aTask.results.missingResult)",
			Operator: selection.In,
			Values:   []string{"$(tasks.aTask.results.missingResult)"},
		}},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1.TaskRef{Name: "bTask"},
		Params: []v1.Param{{
			Name:  "bParam",
			Value: *v1.NewStructuredValues("$(tasks.aTask.results.missingResult)"),
		}},
	},
}, {
	CustomTask:     true,
	CustomRunNames: []string{"aRun"},
	CustomRuns: []*v1beta1.CustomRun{
		{
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
		}},
	PipelineTask: &v1.PipelineTask{
		Name:    "aCustomPipelineTask",
		TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "aTask"},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "bTask",
		TaskRef: &v1.TaskRef{Name: "bTask"},
		Params: []v1.Param{{
			Name:  "bParam",
			Value: *v1.NewStructuredValues("$(tasks.aCustomPipelineTask.results.aResult)"),
		}},
	},
}, {
	TaskRunNames: []string{"cTaskRun"},
	TaskRuns: []*v1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cTaskRun",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "cResult",
					Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
				}},
			},
		},
	}},
	PipelineTask: &v1.PipelineTask{
		Name:    "cTask",
		TaskRef: &v1.TaskRef{Name: "cTask"},
		Params: []v1.Param{{
			Name:  "cParam",
			Value: *v1.NewStructuredValues("$(tasks.cTask.results.cResult[1])"),
		}},
	},
}, {
	TaskRunNames: []string{"dTaskRun"},
	TaskRuns: []*v1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dTaskRun",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "dResult",
					Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
				}},
			},
		},
	}},
	PipelineTask: &v1.PipelineTask{
		Name:    "dTask",
		TaskRef: &v1.TaskRef{Name: "dTask"},
		Params: []v1.Param{{
			Name:  "dParam",
			Value: *v1.NewStructuredValues("$(tasks.dTask.results.dResult[3])"),
		}},
	},
}, {
	TaskRunNames: []string{"eTaskRun"},
	TaskRuns: []*v1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{Name: "eTaskRun"},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{failedCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "eResult",
					Value: *v1.NewStructuredValues("eResultValue"),
				}},
			},
		},
	}},
	PipelineTask: &v1.PipelineTask{
		Name:    "eTask",
		TaskRef: &v1.TaskRef{Name: "eTask"},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "fTask",
		TaskRef: &v1.TaskRef{Name: "fTask"},
		Params: v1.Params{{
			Name:  "fParam",
			Value: *v1.NewStructuredValues("$(tasks.eTask.results.eResult)"),
		}},
	},
}, {
	TaskRunNames: []string{"xTaskRun"},
	TaskRuns: []*v1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "xTaskRun",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "xResult",
					Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "yTaskRun",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "yResult",
					Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
				}},
			},
		},
	}},
	PipelineTask: &v1.PipelineTask{
		Name:    "xTask",
		TaskRef: &v1.TaskRef{Name: "xTask"},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "xParam",
				Value: *v1.NewStructuredValues("$(tasks.xTask.results.xResult[*])"),
			}, {
				Name:  "yParam",
				Value: *v1.NewStructuredValues("$(tasks.yTask.results.yResult[*])"),
			}},
		},
	},
}, {
	CustomTask:     true,
	CustomRunNames: []string{"xRun"},
	CustomRuns: []*v1beta1.CustomRun{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "xRun"},
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{successCondition},
				},
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{
					Results: []v1beta1.CustomRunResult{{
						Name:  "xResult",
						Value: "xResultValue",
					}},
				},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "yRun"},
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{successCondition},
				},
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{
					Results: []v1beta1.CustomRunResult{{
						Name:  "yResult",
						Value: "yResultValue",
					}},
				},
			},
		}},
	PipelineTask: &v1.PipelineTask{
		Name:    "xTask",
		TaskRef: &v1.TaskRef{Name: "xTask"},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "xParam",
				Value: *v1.NewStructuredValues("$(tasks.xCustomPipelineTask.results.xResult[*])"),
			}, {
				Name:  "yParam",
				Value: *v1.NewStructuredValues("$(tasks.yCustomPipelineTask.results.yResult[*])"),
			}},
		},
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
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
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
			Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "cTask",
				Result:       "cResult",
				ResultsIndex: 1,
			},
			FromTaskRun: "cTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test unsuccessful matrix array result references resolution",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[11],
		},
		want:    nil,
		wantErr: true,
		wantPt:  "xTask",
	}, {
		name:             "Test unsuccessful result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[8],
		},
		want:    nil,
		wantErr: true,
	}, {
		name:             "Test unsuccessful matrix array result references resolution - customrun",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[12],
		},
		want:    nil,
		wantErr: true,
		wantPt:  "xCustomPipelineTask",
	}, {
		name:             "Test successful result references resolution - when expressions",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[2],
		},
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
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
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aCustomPipelineTask",
				Result:       "aResult",
			},
			FromRun: "aRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful result references resolution - params - failed taskrun",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[10],
		},
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("eResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "eTask",
				Result:       "eResult",
			},
			FromTaskRun: "eTaskRun",
		}},
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
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
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
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
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
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
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
