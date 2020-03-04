package resources

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestTaskParamResolver_ResolveResultRefs(t *testing.T) {
	type fields struct {
		pipelineRunState PipelineRunState
	}
	type args struct {
		param v1beta1.Param
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    ResolvedResultRefs
		wantErr bool
	}{
		{
			name: "successful resolution: param not using result reference",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun:     tb.TaskRun("aTaskRun", "namespace"),
						PipelineTask: &v1alpha1.PipelineTask{
							Name:    "aTask",
							TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
						},
					},
				},
			},
			args: args{
				param: v1beta1.Param{
					Name: "targetParam",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "explicitValueNoResultReference",
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "successful resolution: using result reference",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun: tb.TaskRun("aTaskRun", "namespace", tb.TaskRunStatus(
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
				param: v1beta1.Param{
					Name: "targetParam",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult)",
					},
				},
			},
			want: ResolvedResultRefs{
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
			wantErr: false,
		}, {
			name: "successful resolution: using multiple result reference",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun: tb.TaskRun("aTaskRun", "namespace", tb.TaskRunStatus(
							tb.TaskRunResult("aResult", "aResultValue"),
						)),
						PipelineTask: &v1alpha1.PipelineTask{
							Name:    "aTask",
							TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
						},
					}, {
						TaskRunName: "bTaskRun",
						TaskRun: tb.TaskRun("bTaskRun", "namespace", tb.TaskRunStatus(
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
				param: v1beta1.Param{
					Name: "targetParam",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult) $(tasks.bTask.results.bResult)",
					},
				},
			},
			want: ResolvedResultRefs{
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
				}, {
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "bResultValue",
					},
					ResultReference: v1beta1.ResultRef{
						PipelineTask: "bTask",
						Result:       "bResult",
					},
					FromTaskRun: "bTaskRun",
				},
			},
			wantErr: false,
		}, {
			name: "successful resolution: duplicate result references",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun: tb.TaskRun("aTaskRun", "namespace", tb.TaskRunStatus(
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
				param: v1beta1.Param{
					Name: "targetParam",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult) $(tasks.aTask.results.aResult)",
					},
				},
			},
			want: ResolvedResultRefs{
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
			wantErr: false,
		}, {
			name: "unsuccessful resolution: referenced result doesn't exist in referenced task",
			fields: fields{
				pipelineRunState: PipelineRunState{
					{
						TaskRunName: "aTaskRun",
						TaskRun:     tb.TaskRun("aTaskRun", "namespace"),
						PipelineTask: &v1alpha1.PipelineTask{
							Name:    "aTask",
							TaskRef: &v1alpha1.TaskRef{Name: "aTask"},
						},
					},
				},
			},
			args: args{
				param: v1beta1.Param{
					Name: "targetParam",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult)",
					},
				},
			},
			want:    nil,
			wantErr: true,
		}, {
			name: "unsuccessful resolution: pipeline task missing",
			fields: fields{
				pipelineRunState: PipelineRunState{},
			},
			args: args{
				param: v1beta1.Param{
					Name: "targetParam",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult)",
					},
				},
			},
			want:    nil,
			wantErr: true,
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
				param: v1beta1.Param{
					Name: "targetParam",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "$(tasks.aTask.results.aResult)",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("test name: %s\n", tt.name)
			got, err := extractResultRefsFromParam(tt.fields.pipelineRunState, tt.args.param)
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
	type args struct {
		pipelineRunState PipelineRunState
		targets          PipelineRunState
	}
	pipelineRunState := PipelineRunState{
		{
			TaskRunName: "aTaskRun",
			TaskRun: tb.TaskRun("aTaskRun", "namespace", tb.TaskRunStatus(
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
			got, err := ResolveResultRefs(tt.args.pipelineRunState, tt.args.targets)
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
