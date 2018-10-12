package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTaskRun_Invalidate(t *testing.T) {
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
				t.Errorf("TaskRun.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}

func TestTaskRun_Validate(t *testing.T) {
	tr := TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "taskname",
		},
		Spec: TaskRunSpec{
			TaskRef: TaskRef{
				Name: "taskrefname",
			},
			Trigger: TaskTrigger{
				TriggerRef: TaskTriggerRef{
					Type: TaskTriggerTypePipelineRun,
					Name: "testtriggername",
				},
			},
		},
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("TaskRun.Validate() error = %v", err)
	}
}

func TestTaskRunSpec_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    TaskRunSpec
		wantErr *apis.FieldError
	}{
		{
			name:    "invalid taskspec",
			spec:    TaskRunSpec{},
			wantErr: apis.ErrMissingField("spec"),
		},
		{
			name: "invalid taskref name",
			spec: TaskRunSpec{
				TaskRef: TaskRef{},
				Trigger: TaskTrigger{
					TriggerRef: TaskTriggerRef{
						Type: TaskTriggerTypeManual,
					},
				},
			},
			wantErr: apis.ErrMissingField("spec.taskref.name"),
		},
		{
			name: "invalid taskref type",
			spec: TaskRunSpec{
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
			name: "invalid taskref",
			spec: TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Trigger: TaskTrigger{
					TriggerRef: TaskTriggerRef{
						Type: TaskTriggerTypePipelineRun,
						Name: "",
					},
				},
			},
			wantErr: apis.ErrMissingField("spec.trigger.triggerref.name"),
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.spec.Validate()
			if d := cmp.Diff(ts.wantErr.Error(), err.Error()); d != "" {
				t.Errorf("TaskRunSpec.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}

func TestTaskRunSpec_Validate(t *testing.T) {
	tests := []struct {
		name string
		spec TaskRunSpec
	}{
		{
			name: "task trigger run type",
			spec: TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Trigger: TaskTrigger{
					TriggerRef: TaskTriggerRef{
						Type: TaskTriggerTypePipelineRun,
						Name: "testtrigger",
					},
				},
			},
		},
		{
			name: "task trigger run type with different capitalization",
			spec: TaskRunSpec{
				TaskRef: TaskRef{
					Name: "taskrefname",
				},
				Trigger: TaskTrigger{
					TriggerRef: TaskTriggerRef{
						Type: "PiPeLiNeRuN",
						Name: "testtrigger",
					},
				},
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			if err := ts.spec.Validate(); err != nil {
				t.Errorf("TaskRunSpec.Validate()/%s error = %v", ts.name, err)
			}
		})
	}
}

func TestInput_Validate(t *testing.T) {
	i := TaskRunInputs{
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
	}
	if err := i.Validate("spec.inputs"); err != nil {
		t.Errorf("TaskRunInputs.Validate() error = %v", err)
	}
}
func TestInput_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		inputs  TaskRunInputs
		wantErr *apis.FieldError
	}{
		{
			name: "invalid task inputs",
			inputs: TaskRunInputs{
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
			wantErr: apis.ErrMultipleOneOf("spec.Inputs.Resources.Name"),
		},
		{
			name: "invalid task input params",
			inputs: TaskRunInputs{
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
			wantErr: apis.ErrMultipleOneOf("spec.inputs.params"),
		},
	}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.inputs.Validate("spec.Inputs")
			if d := cmp.Diff(err.Error(), ts.wantErr.Error()); d != "" {
				t.Errorf("TaskRunInputs.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}
func TestResult_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		result  *Results
		wantErr *apis.FieldError
	}{
		{
			name: "invalid task type result",
			result: &Results{
				Runs: ResultTarget{
					Name: "resultlogs",
					Type: "wrongtype",
				},
			},
			wantErr: apis.ErrInvalidValue("wrongtype", "spec.results.runs.Type"),
		},
		{
			name: "invalid task type result name",
			result: &Results{
				Runs: ResultTarget{
					Name: "",
					Type: ResultTargetTypeGCS,
				},
			},
			wantErr: apis.ErrMissingField("spec.results.runs.name"),
		},
		{
			name: "invalid task type result url",
			result: &Results{
				Runs: ResultTarget{
					Name: "resultrunname",
					Type: ResultTargetTypeGCS,
					URL:  "",
				},
			},
			wantErr: apis.ErrMissingField("spec.results.runs.URL"),
		},
		{
			name: "invalid task type result url",
			result: &Results{
				Tests: &ResultTarget{
					Name: "resultrunname",
					Type: "badtype",
				},
			},
			wantErr: apis.ErrInvalidValue("badtype", "spec.results.tests.Type"),
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.result.Validate("spec.results")
			if d := cmp.Diff(err.Error(), ts.wantErr.Error()); d != "" {
				t.Errorf("Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}
func TestResultTarget_Validate(t *testing.T) {
	rs := &Results{
		Runs: ResultTarget{
			Name: "testname",
			Type: ResultTargetTypeGCS,
			URL:  "http://github.com",
		},
		Logs: ResultTarget{
			Name: "testname",
			Type: ResultTargetTypeGCS,
			URL:  "http://github.com",
		},
		Tests: &ResultTarget{
			Name: "testname",
			Type: ResultTargetTypeGCS,
			URL:  "http://github.com",
		},
	}
	if err := rs.Validate("spec.results"); err != nil {
		t.Errorf("ResultTarget.Validate() error = %v", err)
	}
}
func TestOutput_Validate(t *testing.T) {
	o := Outputs{
		Resources: []TaskResource{{
			Type: "git",
			Name: "resource1",
		}},
	}
	if err := o.Validate("spec.outputs"); err != nil {
		t.Errorf("Outputs.Validate() error = %v", err)
	}
}
func TestOutput_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		outputs Outputs
		wantErr *apis.FieldError
	}{
		{
			name: "invalid task output type",
			outputs: Outputs{
				Resources: []TaskResource{{
					Name: "resourceName",
					Type: "invalidtype",
				}},
			},
			wantErr: apis.ErrInvalidValue("invalidtype", "spec.Outputs.Resources.resourceName.Type"),
		},
		{
			name: "duplicate task output name",
			outputs: Outputs{
				Resources: []TaskResource{{
					Type: "git",
					Name: "resource1",
				}, {
					Type: "git",
					Name: "resource1",
				}},
			},
			wantErr: apis.ErrMultipleOneOf("spec.Outputs.Resources.resource1.Name"),
		},
	}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.outputs.Validate("spec.Outputs")
			if d := cmp.Diff(err.Error(), ts.wantErr.Error()); d != "" {
				t.Errorf("Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}
