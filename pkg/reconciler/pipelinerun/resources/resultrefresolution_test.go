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
				ResultsIndex: 1,
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
				ResultsIndex: 0,
			},
			FromTaskRun: "cTaskRun",
		}, {
			Value: *v1.NewStructuredValues("arrayResultOne", "arrayResultTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "dTask",
				Result:       "dResult",
				ResultsIndex: 0,
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
		wantErr: "Invalid task result reference: Could not find result with name missingResult for task aTask",
	}, {
		name:             "Test unsuccessful result references resolution - params",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[4],
		},
		wantErr: "Invalid task result reference: Could not find result with name missingResult for task aTask",
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
		wantErr: "Invalid task result reference: Could not find result with name iDoNotExist for task dTask",
	}, {
		name:             "Invalid: Test result references resolution - matrix custom task - missing references to string replacements",
		pipelineRunState: pipelineRunState,
		targets: PipelineRunState{
			pipelineRunState[14],
		},
		wantErr: "Invalid task result reference: Could not find result with name iDoNotExist for task aCustomPipelineTask",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckMissingResultReferences(tt.pipelineRunState, tt.targets)
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
				ResultsIndex: 1,
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: "Array Result Index 1 for Task aTask Result aResult is out of bound of size 0",
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
				ResultsIndex: 1,
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: "",
	}, {
		name: "Out Of Bounds Array",
		refs: ResolvedResultRefs{{
			Value: v1.ResultValue{
				Type:     "array",
				ArrayVal: []string{"a", "b", "c"},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
				ResultsIndex: 3,
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: "Array Result Index 3 for Task aTask Result aResult is out of bound of size 3",
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
				ResultsIndex: 1,
			},
			FromTaskRun: "aTaskRun",
		}, {
			Value: v1.ResultValue{
				Type:     "array",
				ArrayVal: []string{"a", "b", "c"},
			},
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
				ResultsIndex: 3,
			},
			FromTaskRun: "aTaskRun",
		}},
		wantErr: "Array Result Index 3 for Task aTask Result aResult is out of bound of size 3",
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
