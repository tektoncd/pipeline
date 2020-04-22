package resources

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	params "github.com/tektoncd/pipeline/pkg/apis/params/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	results "github.com/tektoncd/pipeline/pkg/apis/results/v1beta1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestExtractResultRefsForParam(t *testing.T) {
	type fields struct {
		pipelineRunState PipelineRunState
	}
	type args struct {
		param params.Param
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   ResolvedResultRefs
	}{
		{
			name: "successful resolution: param not using result reference",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun:     tb.TaskRun("aTaskRun"),
						PipelineTask: &v1alpha1.PipelineTask{
							Name:    "aTask",
							TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
						},
					},
				},
			},
			args: args{
				param: params.Param{
					Name: "targetParam",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "explicitValueNoResultReference",
					},
				},
			},
			want: nil,
		},
		{
			name: "successful resolution: using result reference",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun: tb.TaskRun("aTaskRun", tb.TaskRunStatus(
							tb.TaskRunResult("aResult", "aResultValue"),
						)),
						PipelineTask: &v1alpha1.PipelineTask{
							Name:    "aTask",
							TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
						},
					},
				},
			},
			args: args{
				param: params.Param{
					Name: "targetParam",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult)",
					},
				},
			},
			want: ResolvedResultRefs{
				{
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "aResultValue",
					},
					ResultReference: results.ResultRef{
						PipelineTask: "aTask",
						Result:       "aResult",
					},
					FromTaskRun: "aTaskRun",
				},
			},
		}, {
			name: "successful resolution: using multiple result reference",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun: tb.TaskRun("aTaskRun", tb.TaskRunStatus(
							tb.TaskRunResult("aResult", "aResultValue"),
						)),
						PipelineTask: &v1alpha1.PipelineTask{
							Name:    "aTask",
							TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
						},
					}, {
						TaskRunName: "bTaskRun",
						TaskRun: tb.TaskRun("bTaskRun", tb.TaskRunStatus(
							tb.TaskRunResult("bResult", "bResultValue"),
						)),
						PipelineTask: &v1alpha1.PipelineTask{
							Name:    "bTask",
							TaskRef: &v1alpha1.TaskRef{Name: "bTask"},
						},
					},
				},
			},
			args: args{
				param: params.Param{
					Name: "targetParam",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult) $(tasks.bTask.results.bResult)",
					},
				},
			},
			want: ResolvedResultRefs{
				{
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "aResultValue",
					},
					ResultReference: results.ResultRef{
						PipelineTask: "aTask",
						Result:       "aResult",
					},
					FromTaskRun: "aTaskRun",
				}, {
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "bResultValue",
					},
					ResultReference: results.ResultRef{
						PipelineTask: "bTask",
						Result:       "bResult",
					},
					FromTaskRun: "bTaskRun",
				},
			},
		}, {
			name: "successful resolution: duplicate result references",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun: tb.TaskRun("aTaskRun", tb.TaskRunStatus(
							tb.TaskRunResult("aResult", "aResultValue"),
						)),
						PipelineTask: &v1alpha1.PipelineTask{
							Name:    "aTask",
							TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
						},
					},
				},
			},
			args: args{
				param: params.Param{
					Name: "targetParam",
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult) $(tasks.aTask.results.aResult)",
					},
				},
			},
			want: ResolvedResultRefs{
				{
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "aResultValue",
					},
					ResultReference: results.ResultRef{
						PipelineTask: "aTask",
						Result:       "aResult",
					},
					FromTaskRun: "aTaskRun",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractResultRefsForParam(tt.fields.pipelineRunState, tt.args.param)
			// sort result ref based on task name to guarantee an certain order
			sort.SliceStable(got, func(i, j int) bool {
				return strings.Compare(got[i].FromTaskRun, got[j].FromTaskRun) < 0
			})
			if err != nil {
				t.Fatalf("Didn't expect error when extracting result refs but got %v", err)
			}
			if len(tt.want) != len(got) {
				t.Fatalf("incorrect number of refs, want %d, got %d", len(tt.want), len(got))
			}
			// TODO: use comparison function?
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

func TestExtractResultRefsForParamErrs(t *testing.T) {
	type fields struct {
		pipelineRunState PipelineRunState
	}
	type args struct {
		param params.Param
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{{
		name: "unsuccessful resolution: referenced result doesn't exist in referenced task",
		fields: fields{
			pipelineRunState: PipelineRunState{
				{
					TaskRunName: "aTaskRun",
					TaskRun:     tb.TaskRun("aTaskRun"),
					PipelineTask: &v1alpha1.PipelineTask{
						Name:    "aTask",
						TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
					},
				},
			},
		},
		args: args{
			param: params.Param{
				Name: "targetParam",
				Value: params.ArrayOrString{
					Type:      params.ParamTypeString,
					StringVal: "$(tasks.aTask.results.aResult)",
				},
			},
		},
	}, {
		name: "unsuccessful resolution: pipeline task missing",
		fields: fields{
			pipelineRunState: PipelineRunState{},
		},
		args: args{
			param: params.Param{
				Name: "targetParam",
				Value: params.ArrayOrString{
					Type:      params.ParamTypeString,
					StringVal: "$(tasks.aTask.results.aResult)",
				},
			},
		},
	}, {
		name: "unsuccessful resolution: task run missing",
		fields: fields{
			pipelineRunState: PipelineRunState{
				{
					PipelineTask: &v1alpha1.PipelineTask{
						Name:    "aTask",
						TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
					},
				},
			},
		},
		args: args{
			param: params.Param{
				Name: "targetParam",
				Value: params.ArrayOrString{
					Type:      params.ParamTypeString,
					StringVal: "$(tasks.aTask.results.aResult)",
				},
			},
		},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractResultRefsForParam(tt.fields.pipelineRunState, tt.args.param)
			if err == nil {
				t.Fatalf("Expected error when extracting invalid results but got none")
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
	type args struct {
		pipelineRunState PipelineRunState
		targets          PipelineRunState
	}
	pipelineRunState := PipelineRunState{
		{
			TaskRunName: "aTaskRun",
			TaskRun: tb.TaskRun("aTaskRun", tb.TaskRunStatus(
				tb.TaskRunResult("aResult", "aResultValue"),
			)),
			PipelineTask: &v1alpha1.PipelineTask{
				Name:    "aTask",
				TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
			},
		}, {
			PipelineTask: &v1alpha1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1alpha1.TaskRef{Name: "bTask"},
				Params: []params.Param{
					{
						Name: "bParam",
						Value: params.ArrayOrString{
							Type:      params.ParamTypeString,
							StringVal: "$(tasks.aTask.results.aResult)",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		args    args
		want    ResolvedResultRefs
		wantErr bool
	}{
		{
			name: "Test successful result references resolution",
			args: args{
				pipelineRunState: pipelineRunState,
				targets: PipelineRunState{
					pipelineRunState[1],
				},
			},
			want: ResolvedResultRefs{
				{
					Value: params.ArrayOrString{
						Type:      params.ParamTypeString,
						StringVal: "aResultValue",
					},
					ResultReference: results.ResultRef{
						PipelineTask: "aTask",
						Result:       "aResult",
					},
					FromTaskRun: "aTaskRun",
				},
			},
			wantErr: false,
		},
		{
			name: "Test successful result references resolution non result references",
			args: args{
				pipelineRunState: pipelineRunState,
				targets: PipelineRunState{
					pipelineRunState[0],
				},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveResultRefs(tt.args.pipelineRunState, tt.args.targets, nil)
			sort.SliceStable(got, func(i, j int) bool {
				return strings.Compare(got[i].FromTaskRun, got[j].FromTaskRun) < 0
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveResultRefs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Fatalf("ResolveResultRef  -want, +got: %v", d)
			}
		})
	}
}
