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
	taskresources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
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
	idx1 = 1
	idx2 = 2
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
	CustomRuns: []*v1beta1.CustomRun{{
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
	PipelineTask: &v1.PipelineTask{
		Name:    "gTask",
		TaskRef: &v1.TaskRef{Name: "gTask"},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "dResults",
				Value: *v1.NewStructuredValues("$(tasks.dTask.results.dResult[*])"),
			}, {
				Name:  "cResults",
				Value: *v1.NewStructuredValues("$(tasks.cTask.results.cResult[*])"),
			}},
		},
	},
}, {
	CustomTask: true,
	PipelineTask: &v1.PipelineTask{
		Name:    "hTask",
		TaskRef: &v1.TaskRef{Name: "hTask"},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "aResult",
				Value: *v1.NewStructuredValues("$(tasks.aCustomPipelineTask.results.aResult)"),
			}},
		},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "iTask",
		TaskRef: &v1.TaskRef{Name: "iTask"},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "iDoNotExist",
				Value: *v1.NewStructuredValues("$(tasks.dTask.results.iDoNotExist[*])"),
			}},
		},
	},
}, {
	CustomTask: true,
	PipelineTask: &v1.PipelineTask{
		Name:    "jTask",
		TaskRef: &v1.TaskRef{Name: "jTask"},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "iDoNotExist",
				Value: *v1.NewStructuredValues("$(tasks.aCustomPipelineTask.results.iDoNotExist)"),
			}},
		},
	},
}, {
	TaskRunNames: []string{"kTaskRun"},
	TaskRuns: []*v1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kTaskRun-0",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "IMAGE-DIGEST",
					Value: *v1.NewStructuredValues("123"),
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "kTaskRun-1",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "IMAGE-DIGEST",
					Value: *v1.NewStructuredValues("345"),
				}},
			},
		},
	}},
	PipelineTask: &v1.PipelineTask{
		Name:    "kTask",
		TaskRef: &v1.TaskRef{Name: "kTask"},
		Matrix: &v1.Matrix{
			Include: v1.IncludeParamsList{{
				Name: "build-1",
				Params: v1.Params{{
					Name:  "NAME",
					Value: *v1.NewStructuredValues("image-1"),
				}, {
					Name:  "DOCKERFILE",
					Value: *v1.NewStructuredValues("path/to/Dockerfile1"),
				}},
			}, {
				Name: "build-2",
				Params: v1.Params{{
					Name:  "NAME",
					Value: *v1.NewStructuredValues("image-2"),
				}, {
					Name:  "DOCKERFILE",
					Value: *v1.NewStructuredValues("path/to/Dockerfile2"),
				}},
			}},
		},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "hTask",
		TaskRef: &v1.TaskRef{Name: "hTask"},
		Params: v1.Params{{
			Name:  "image-digest",
			Value: *v1.NewStructuredValues("$(tasks.kTask.results.IMAGE-DIGEST)[*]"),
		}},
	},
}, {
	PipelineTask: &v1.PipelineTask{
		Name:    "iTask",
		TaskRef: &v1.TaskRef{Name: "iTask"},
		Params: v1.Params{{
			Name:  "image-digest",
			Value: *v1.NewStructuredValues("$(tasks.kTask.results.I-DO-NOT-EXIST)[*]"),
		}},
	},
}, {
	TaskRunNames: []string{"lTaskRun"},
	TaskRuns:     []*v1.TaskRun{},
	PipelineTask: &v1.PipelineTask{
		Name:    "lTask",
		TaskRef: &v1.TaskRef{Name: "lTask"},
		Params: []v1.Param{{
			Name:  "jParam",
			Value: *v1.NewStructuredValues("$(tasks.does-not-exist.results.some-result)"),
		}},
	},
}, {
	TaskRunNames: []string{"mTaskRun"},
	TaskRuns:     []*v1.TaskRun{},
	PipelineTask: &v1.PipelineTask{
		Name:    "mTask",
		TaskRef: &v1.TaskRef{Name: "mTask"},
		Params: []v1.Param{{
			Name:  "mParam",
			Value: *v1.NewStructuredValues("$(tasks.lTask.results.aResult)"),
		}},
	},
}, {
	TaskRunNames: []string{"nTaskRun"},
	TaskRuns: []*v1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nTaskRun",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{successCondition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "nResult",
					Value: *v1.NewStructuredValues("one"),
				}},
			},
		},
	}},
	PipelineTask: &v1.PipelineTask{
		Name:    "nTask",
		TaskRef: &v1.TaskRef{Name: "nTask"},
		Matrix: &v1.Matrix{
			Include: v1.IncludeParamsList{v1.IncludeParams{}},
		},
	},
}, {
	TaskRunNames: []string{"oTaskRun"},
	TaskRuns:     []*v1.TaskRun{},
	PipelineTask: &v1.PipelineTask{
		Name:    "oTask",
		TaskRef: &v1.TaskRef{Name: "oTask"},
		Params: []v1.Param{{
			Name:  "oParam",
			Value: *v1.NewStructuredValues("$(tasks.nTask.results.nResult)"),
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
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful array result references - array indexing",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[7],
		},
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "cTask",
				Result:       "cResult",
				ResultsIndex: &idx1,
			},
			FromTaskRun: "cTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful matrix array result references resolution - whole array references",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[11],
		},
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "cTask",
				Result:       "cResult",
			},
			FromTaskRun: "cTaskRun",
		}, {
			Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "dTask",
				Result:       "dResult",
			},
			FromTaskRun: "dTaskRun",
		}},
		wantErr: false,
	}, {
		name:             "Test successful matrix result references resolution - customrun",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[12],
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
		name:             "Test unsuccessful matrix array result references resolution",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[13],
		},
		want:    nil,
		wantErr: true,
		wantPt:  "dTask",
	}, {
		name:             "Test unsuccessful matrix array result references resolution custom run",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[14],
		},
		want:    nil,
		wantErr: true,
		wantPt:  "aCustomPipelineTask",
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
	}, {
		name:             "Test successful result references matrix emitting results",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[16],
		},
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("123", "345"),
			ResultReference: v1.ResultRef{
				PipelineTask: "kTask",
				Result:       "IMAGE-DIGEST",
			},
			FromTaskRun: "kTaskRun-1",
		}},
	}, {
		name:             "Test unsuccessful result references matrix emitting results",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[17],
		},
		wantPt:  "kTask",
		wantErr: true,
	}, {
		name:             "Test result lookup single element matrix",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[21],
		},
		want: ResolvedResultRefs{{
			Value: v1.ParamValue{
				Type:     v1.ParamTypeArray,
				ArrayVal: []string{"one"},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "nTask",
				Result:       "nResult",
			},
			FromTaskRun: "nTaskRun",
		}},
	}, {
		name: "Test successful result references resolution with default value",
		pipelineRunState: PipelineRunState{{
			TaskRunNames: []string{"taskWithDefault"},
			TaskRuns: []*v1.TaskRun{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "taskWithDefault",
				},
				Status: v1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1.TaskRunStatusFields{
						// Note: no results in TaskRun status - should use default
					},
				},
			}},
			PipelineTask: &v1.PipelineTask{
				Name:    "taskWithDefault",
				TaskRef: &v1.TaskRef{Name: "taskWithDefault"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name:    "defaultResult",
						Type:    v1.ResultsTypeString,
						Default: v1.NewStructuredValues("defaultValue"),
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithDefault.results.defaultResult)"),
				}},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithDefault.results.defaultResult)"),
				}},
			},
		}},
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("defaultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "taskWithDefault",
				Result:       "defaultResult",
			},
			FromTaskRun: "taskWithDefault",
		}},
		wantErr: false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			enableDefaultResults := tt.name == "Test successful result references resolution with default value"
			got, pt, err := ResolveResultRefs(enableDefaultResults, tt.pipelineRunState, tt.targets)
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
			got, pt, err := ResolveResultRef(false, tt.pipelineRunState, tt.target)
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

func TestCheckMissingResultReferences(t *testing.T) {
	for _, tt := range []struct {
		name             string
		pipelineRunState PipelineRunState
		targets          PipelineRunState
		wantErr          string
	}{{
		name:             "Valid: successful result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[1],
		},
	}, {
		name:             "Valid: Test successful array result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[7],
		},
	}, {
		name:             "Valid: Test successful result references resolution - when expressions",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[2],
		},
	}, {
		name:             "Invalid: Test unsuccessful result references resolution - when expression",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[3],
		},
		wantErr: "Invalid task result reference: Could not find result with name missingResult for pipeline task aTask",
	}, {
		name:             "Test unsuccessful result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[4],
		},
		wantErr: "Invalid task result reference: Could not find result with name missingResult for pipeline task aTask",
	}, {
		name:             "Valid: Test successful result references resolution - params - Run",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[6],
		},
	}, {
		name:             "Valid: Test successful result references resolution non result references",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[0],
		},
	}, {
		name:             "Valid: Test successful result references resolution - params - failed taskrun",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[10],
		},
	}, {
		name:             "Valid: Test successful result references resolution - matrix - whole array replacements",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[11],
		},
	}, {
		name:             "Valid: Test successful result references resolution - matrix custom task - string replacements",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[12],
		},
	}, {
		name:             "Invalid: Test result references resolution - matrix - missing references to whole array replacements",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[13],
		},
		wantErr: "Invalid task result reference: Could not find result with name iDoNotExist for pipeline task dTask",
	}, {
		name:             "Invalid: Test result references resolution - matrix custom task - missing references to string replacements",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[14],
		},
		wantErr: "Invalid task result reference: Could not find result with name iDoNotExist for pipeline task aCustomPipelineTask",
	}, {
		name:             "Invalid: Test result references where ref does not exist in pipelineRunState map",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[18],
		},
		wantErr: "Result reference error: Could not find ref \"does-not-exist\" in internal pipelineRunState",
	}, {
		name:             "Invalid: Test result references where referencedPipelineTask has no TaskRuns",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[19],
		},
		wantErr: "Result reference error: Internal result ref \"lTask\" has zero-length TaskRuns",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			for _, target := range tt.targets {
				tmpErr := CheckMissingResultReferences(false, tt.pipelineRunState, target)
				if tmpErr != nil {
					err = tmpErr
				}
			}
			if (err != nil) && err.Error() != tt.wantErr {
				t.Errorf("CheckMissingResultReferences() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.wantErr != "" {
				t.Fatalf("Expecting error %v, but did not get an error", tt.wantErr)
			}
		})
	}
}

func TestValidateArrayResultsIndex(t *testing.T) {
	for _, tt := range []struct {
		name    string
		refs    ResolvedResultRefs
		wantErr string
	}{{
		name: "Empty Array",
		refs: ResolvedResultRefs{{
			Value: v1.ResultValue{
				Type:     "array",
				ArrayVal: []string{},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
				ResultsIndex: nil,
			},
			FromTaskRun: "aTaskRun",
		}},
	}, {
		name: "Reference an Empty Array",
		refs: ResolvedResultRefs{{
			Value: v1.ResultValue{
				Type:     "array",
				ArrayVal: []string{},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
				ResultsIndex: &idx1,
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: "array Result Index 1 for Task aTask Result aResult is out of bound of size 0",
	}, {
		name: "In Bounds Array",
		refs: ResolvedResultRefs{{
			Value: v1.ResultValue{
				Type:     "array",
				ArrayVal: []string{"a", "b", "c"},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
				ResultsIndex: &idx1,
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: "",
	}, {
		name: "Out Of Bounds Array",
		refs: ResolvedResultRefs{{
			Value: v1.ResultValue{
				Type:     "array",
				ArrayVal: []string{"a", "b"},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
				ResultsIndex: &idx2,
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: "array Result Index 2 for Task aTask Result aResult is out of bound of size 2",
	}, {
		name: "In Bounds and Out of Bounds Array",
		refs: ResolvedResultRefs{{
			Value: v1.ResultValue{
				Type:     "array",
				ArrayVal: []string{"a", "b", "c"},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
				ResultsIndex: &idx1,
			},
			FromTaskRun: "aTaskRun",
		}, {
			Value: v1.ResultValue{
				Type:     "array",
				ArrayVal: []string{"a", "b"},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
				ResultsIndex: &idx2,
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: "array Result Index 2 for Task aTask Result aResult is out of bound of size 2",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArrayResultsIndex(tt.refs)
			if (err != nil) && err.Error() != tt.wantErr {
				t.Errorf("validateArrayResultsIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.wantErr != "" {
				t.Fatalf("Expecting error %v, but did not get an error", tt.wantErr)
			}
		})
	}
}

func TestParamValueFromCustomRunResult(t *testing.T) {
	type args struct {
		result string
	}
	tests := []struct {
		name string
		args args
		want *v1.ParamValue
	}{
		{
			name: "multiple array elements result",
			args: args{
				result: `["amd64", "arm64"]`,
			},
			want: &v1.ParamValue{
				Type:     "array",
				ArrayVal: []string{"amd64", "arm64"},
			},
		},
		{
			name: "single array elements result",
			args: args{
				result: `[ "amd64" ]`,
			},
			want: &v1.ParamValue{
				Type:     "array",
				ArrayVal: []string{"amd64"},
			},
		},
		{
			name: "simple string result",
			args: args{
				result: "amd64",
			},
			want: &v1.ParamValue{
				Type:      "string",
				StringVal: "amd64",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paramValueFromCustomRunResult(tt.args.result)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Fatalf("paramValueFromCustomRunResult %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolveResultRefsWithDefaultValues(t *testing.T) {
	tests := []struct {
		name             string
		pipelineRunState PipelineRunState
		targets          PipelineRunState
		featureFlag      bool
		want             ResolvedResultRefs
		wantErr          bool
		wantErrMsg       string
	}{{
		name: "default value used when feature flag enabled and result missing",
		pipelineRunState: PipelineRunState{{
			TaskRunNames: []string{"taskWithDefault"},
			TaskRuns: []*v1.TaskRun{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "taskWithDefault",
				},
				Status: v1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1.TaskRunStatusFields{
						// Note: no results in TaskRun status - should use default
					},
				},
			}},
			PipelineTask: &v1.PipelineTask{
				Name:    "taskWithDefault",
				TaskRef: &v1.TaskRef{Name: "taskWithDefault"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name:    "defaultResult",
						Type:    v1.ResultsTypeString,
						Default: v1.NewStructuredValues("defaultValue"),
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithDefault.results.defaultResult)"),
				}},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithDefault.results.defaultResult)"),
				}},
			},
		}},
		featureFlag: true,
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("defaultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "taskWithDefault",
				Result:       "defaultResult",
			},
			FromTaskRun: "taskWithDefault",
		}},
		wantErr: false,
	}, {
		name: "default value not used when feature flag disabled",
		pipelineRunState: PipelineRunState{{
			TaskRunNames: []string{"taskWithDefault"},
			TaskRuns: []*v1.TaskRun{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "taskWithDefault",
				},
				Status: v1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1.TaskRunStatusFields{
						// Note: no results in TaskRun status
					},
				},
			}},
			PipelineTask: &v1.PipelineTask{
				Name:    "taskWithDefault",
				TaskRef: &v1.TaskRef{Name: "taskWithDefault"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name:    "defaultResult",
						Type:    v1.ResultsTypeString,
						Default: v1.NewStructuredValues("defaultValue"),
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithDefault.results.defaultResult)"),
				}},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithDefault.results.defaultResult)"),
				}},
			},
		}},
		featureFlag: false,
		wantErr:     true,
		wantErrMsg:  "Could not find result with name defaultResult",
	}, {
		name: "actual result takes precedence over default when both exist",
		pipelineRunState: PipelineRunState{{
			TaskRunNames: []string{"taskWithDefault"},
			TaskRuns: []*v1.TaskRun{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "taskWithDefault",
				},
				Status: v1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1.TaskRunStatusFields{
						Results: []v1.TaskRunResult{{
							Name:  "defaultResult",
							Value: *v1.NewStructuredValues("actualValue"),
						}},
					},
				},
			}},
			PipelineTask: &v1.PipelineTask{
				Name:    "taskWithDefault",
				TaskRef: &v1.TaskRef{Name: "taskWithDefault"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name:    "defaultResult",
						Type:    v1.ResultsTypeString,
						Default: v1.NewStructuredValues("defaultValue"),
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithDefault.results.defaultResult)"),
				}},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithDefault.results.defaultResult)"),
				}},
			},
		}},
		featureFlag: true,
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("actualValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "taskWithDefault",
				Result:       "defaultResult",
			},
			FromTaskRun: "taskWithDefault",
		}},
		wantErr: false,
	}, {
		name: "default value used for array result",
		pipelineRunState: PipelineRunState{{
			TaskRunNames: []string{"taskWithArrayDefault"},
			TaskRuns: []*v1.TaskRun{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "taskWithArrayDefault",
				},
				Status: v1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1.TaskRunStatusFields{
						// Note: no results in TaskRun status
					},
				},
			}},
			PipelineTask: &v1.PipelineTask{
				Name:    "taskWithArrayDefault",
				TaskRef: &v1.TaskRef{Name: "taskWithArrayDefault"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name:    "arrayResult",
						Type:    v1.ResultsTypeArray,
						Default: v1.NewStructuredValues("value1", "value2", "value3"),
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithArrayDefault.results.arrayResult)"),
				}},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "consumerParam",
					Value: *v1.NewStructuredValues("$(tasks.taskWithArrayDefault.results.arrayResult)"),
				}},
			},
		}},
		featureFlag: true,
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("value1", "value2", "value3"),
			ResultReference: v1.ResultRef{
				PipelineTask: "taskWithArrayDefault",
				Result:       "arrayResult",
			},
			FromTaskRun: "taskWithArrayDefault",
		}},
		wantErr: false,
	}, {
		name: "default value used for matrix parameter when result missing",
		pipelineRunState: PipelineRunState{{
			TaskRunNames: []string{"matrixTaskWithDefault"},
			TaskRuns: []*v1.TaskRun{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "matrixTaskWithDefault",
				},
				Status: v1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{successCondition},
					},
					TaskRunStatusFields: v1.TaskRunStatusFields{
						// Note: no results in TaskRun status
					},
				},
			}},
			PipelineTask: &v1.PipelineTask{
				Name:    "matrixTaskWithDefault",
				TaskRef: &v1.TaskRef{Name: "matrixTaskWithDefault"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name:    "matrixResult",
						Type:    v1.ResultsTypeArray,
						Default: v1.NewStructuredValues("default1", "default2"),
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerMatrixTask",
				TaskRef: &v1.TaskRef{Name: "consumerMatrixTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "matrixParam",
						Value: *v1.NewStructuredValues("$(tasks.matrixTaskWithDefault.results.matrixResult[*])"),
					}},
				},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerMatrixTask",
				TaskRef: &v1.TaskRef{Name: "consumerMatrixTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "matrixParam",
						Value: *v1.NewStructuredValues("$(tasks.matrixTaskWithDefault.results.matrixResult[*])"),
					}},
				},
			},
		}},
		featureFlag: true,
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("default1", "default2"),
			ResultReference: v1.ResultRef{
				PipelineTask: "matrixTaskWithDefault",
				Result:       "matrixResult",
			},
			FromTaskRun: "matrixTaskWithDefault",
		}},
		wantErr: false,
	}, {
		name: "default value used when referenced task is skipped (no TaskRun) and feature flag enabled",
		pipelineRunState: PipelineRunState{{
			// skippedTask has no TaskRuns - it was skipped (e.g. when expression evaluated to false)
			PipelineTask: &v1.PipelineTask{
				Name:    "skippedTask",
				TaskRef: &v1.TaskRef{Name: "skippedTask"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name:    "skippedResult",
						Type:    v1.ResultsTypeString,
						Default: v1.NewStructuredValues("default-from-skipped"),
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "param",
					Value: *v1.NewStructuredValues("$(tasks.skippedTask.results.skippedResult)"),
				}},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "param",
					Value: *v1.NewStructuredValues("$(tasks.skippedTask.results.skippedResult)"),
				}},
			},
		}},
		featureFlag: true,
		want: ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("default-from-skipped"),
			ResultReference: v1.ResultRef{
				PipelineTask: "skippedTask",
				Result:       "skippedResult",
			},
			FromTaskRun: "",
		}},
		wantErr: false,
	}, {
		name: "error when referenced task is skipped and feature flag disabled",
		pipelineRunState: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "skippedTask",
				TaskRef: &v1.TaskRef{Name: "skippedTask"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name:    "skippedResult",
						Type:    v1.ResultsTypeString,
						Default: v1.NewStructuredValues("default-from-skipped"),
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "param",
					Value: *v1.NewStructuredValues("$(tasks.skippedTask.results.skippedResult)"),
				}},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "param",
					Value: *v1.NewStructuredValues("$(tasks.skippedTask.results.skippedResult)"),
				}},
			},
		}},
		featureFlag: false,
		wantErr:     true,
		wantErrMsg:  "was not finished",
	}, {
		name: "error when referenced task is skipped and result has no default",
		pipelineRunState: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "skippedTask",
				TaskRef: &v1.TaskRef{Name: "skippedTask"},
			},
			ResolvedTask: &taskresources.ResolvedTask{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{{
						Name: "skippedResult",
						Type: v1.ResultsTypeString,
						// no Default
					}},
				},
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "param",
					Value: *v1.NewStructuredValues("$(tasks.skippedTask.results.skippedResult)"),
				}},
			},
		}},
		targets: PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "consumerTask",
				TaskRef: &v1.TaskRef{Name: "consumerTask"},
				Params: []v1.Param{{
					Name:  "param",
					Value: *v1.NewStructuredValues("$(tasks.skippedTask.results.skippedResult)"),
				}},
			},
		}},
		featureFlag: true,
		wantErr:     true,
		wantErrMsg:  "was not finished",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, pt, err := ResolveResultRefs(tt.featureFlag, tt.pipelineRunState, tt.targets)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveResultRefs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if err != nil && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("ResolveResultRefs() error = %v, expected to contain %q", err.Error(), tt.wantErrMsg)
				}
				return
			}
			if d := cmp.Diff(tt.want, got, cmpopts.SortSlices(lessResolvedResultRefs)); d != "" {
				t.Fatalf("ResolveResultRef %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff("", pt); d != "" {
				t.Fatalf("ResolvedPipelineTask %s", diff.PrintWantGot(d))
			}
		})
	}
}
