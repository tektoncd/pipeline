package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTaskRunValidate(t *testing.T) {
	tests := []struct {
		name string
		task TaskRun
		want *apis.FieldError
	}{
		{
			name: "invalid taskspec",
			task: TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "taskmetaname",
				},
			},
			want: apis.ErrMissingField("spec"),
		},
		{
			name: "invalid taskrun metadata",
			task: TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "task.name",
				},
			},
			want: &apis.FieldError{
				Message: "Invalid resource name: special character . must not be present",
				Paths:   []string{"metadata.name"},
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.task.Validate()
			if d := cmp.Diff(err.Error(), ts.want.Error()); d != "" {
				t.Errorf("Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}

func TestTaskRunSpecValidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    *TaskRunSpec
		wantErr *apis.FieldError
	}{
		{
			name:    "invalid taskspec",
			spec:    &TaskRunSpec{},
			wantErr: apis.ErrMissingField("spec"),
		},
		{
			name: "invalid taskref",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{},
				Trigger: TaskTrigger{
					TriggerRef: TaskTriggerRef{
						Type: TaskTriggerTypePipelineRun,
					},
				},
			},
			wantErr: apis.ErrMissingField("spec.taskref.name"),
		},
		{
			name: "invalid taskref",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Trigger: TaskTrigger{
					TriggerRef: TaskTriggerRef{
						Type: "wrongtype",
					},
				},
			},
			wantErr: apis.ErrInvalidValue("wrongtype", "spec.trigger.triggerref.type"),
		},
		{
			name: "valid task trigger run type",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Trigger: TaskTrigger{
					TriggerRef: TaskTriggerRef{
						Type: TaskTriggerTypePipelineRun,
					},
				},
			},
		},
		{
			name: "valid task inputs",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Inputs: TaskRunInputs{
					Params: []Param{{
						Name:  "name",
						Value: "value",
					}},
					Resources: []PipelineResourceVersion{{
						Version: "testv1",
						ResourceRef: PipelineResourceRef{
							Name: "testresource",
						},
					}},
				},
			},
		},
		{
			name: "invalid task inputs",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Inputs: TaskRunInputs{
					Resources: []PipelineResourceVersion{{
						Version: "testv1",
						ResourceRef: PipelineResourceRef{
							Name: "testresource",
						},
					}, {
						Version: "testv1",
						ResourceRef: PipelineResourceRef{
							Name: "testresource",
						},
					}},
				},
			},
			wantErr: apis.ErrMultipleOneOf("spec.Inputs.Resources.Name"),
		},
		{
			name: "invalid task input params",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Inputs: TaskRunInputs{
					Resources: []PipelineResourceVersion{{
						Version: "testv1",
						ResourceRef: PipelineResourceRef{
							Name: "testresource",
						},
					}},
					Params: []Param{{
						Name:  "name",
						Value: "value",
					}, {
						Name:  "name",
						Value: "value",
					}},
				},
			},
			wantErr: apis.ErrMultipleOneOf("spec.inputs.params"),
		},
		{
			name: "invalid task output type",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Outputs: Outputs{
					Resources: []TaskResource{{
						Name: "resourceName",
						Type: "invalidtype",
					}},
				},
			},
			// TODO(shashwathi): wrong error msg
			wantErr: apis.ErrInvalidValue("invalidtype", "spec.Outputs.Resources.resourceName.Type"),
		},
		{
			name: "duplicate task output name",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Outputs: Outputs{
					Resources: []TaskResource{{
						Type: "git",
						Name: "resource1",
					}, {
						Type: "git",
						Name: "resource1",
					}},
				},
			},
			wantErr: apis.ErrMultipleOneOf("spec.Outputs.Resources.Name"),
		},
		{
			name: "valid task output",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Outputs: Outputs{
					Resources: []TaskResource{{
						Type: "git",
					}},
				},
			},
		},
		{
			name: "invalid task type result",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Results: Results{
					Logs: ResultTarget{
						Name: "resultlogs",
						Type: "wrongtype",
					},
				},
			},
			wantErr: apis.ErrInvalidValue("wrongtype", "spec.results.logs.Type"),
		},
		{
			name: "invalid task type result",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Results: Results{
					Runs: ResultTarget{
						Name: "resultlogs",
						Type: "wrongtype",
					},
				},
			},
			wantErr: apis.ErrInvalidValue("wrongtype", "spec.results.runs.Type"),
		},
		{
			name: "invalid task type result",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Results: Results{
					Tests: &ResultTarget{
						Name: "resultlogs",
						Type: "wrongtype",
					},
				},
			},
			wantErr: apis.ErrInvalidValue("wrongtype", "spec.results.tests.Type"),
		},
		{
			name: "invalid task type result name",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Results: Results{
					Runs: ResultTarget{
						Name: "",
						Type: ResultTargetTypeGCS,
					},
				},
			},
			wantErr: apis.ErrMissingField("spec.results.runs.name"),
		},
		{
			name: "invalid task type result url",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Results: Results{
					Runs: ResultTarget{
						Name: "resultrunname",
						Type: ResultTargetTypeGCS,
						URL:  "",
					},
				},
			},
			wantErr: apis.ErrMissingField("spec.results.runs.URL"),
		},
		{
			name: "valid task type result",
			spec: &TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Results: Results{
					Runs: ResultTarget{
						Name: "testname",
						Type: ResultTargetTypeGCS,
						URL:  "http://github.com",
					},
				},
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.spec.Validate()
			if d := cmp.Diff(err.Error(), ts.wantErr.Error()); d != "" {
				t.Errorf("Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}
