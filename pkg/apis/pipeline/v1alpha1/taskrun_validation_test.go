/*
Copyright 2018 The Knative Authors.

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
package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
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
			err := ts.task.Validate(context.Background())
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
			TaskRef: &TaskRef{
				Name: "taskrefname",
			},
		},
	}
	if err := tr.Validate(context.Background()); err != nil {
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
				TaskRef: &TaskRef{},
			},
			wantErr: apis.ErrMissingField("spec.taskref.name, spec.taskspec"),
		},
		{
			name: "invalid taskref and taskspec together",
			spec: TaskRunSpec{
				TaskRef: &TaskRef{
					Name: "taskrefname",
				},
				TaskSpec: &TaskSpec{
					Steps: []corev1.Container{{
						Name:  "mystep",
						Image: "myimage",
					}},
				},
			},
			wantErr: apis.ErrDisallowedFields("spec.taskspec", "spec.taskref"),
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.spec.Validate(context.Background())
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
			name: "taskspec without a taskRef",
			spec: TaskRunSpec{
				TaskSpec: &TaskSpec{
					Steps: []corev1.Container{{
						Name:  "mystep",
						Image: "myimage",
					}},
				},
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			if err := ts.spec.Validate(context.Background()); err != nil {
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
		Resources: []TaskResourceBinding{{
			ResourceRef: PipelineResourceRef{
				Name: "testresource",
			},
			Name: "workspace",
		}},
	}
	if err := i.Validate(context.Background(), "spec.inputs"); err != nil {
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
			name: "duplicate task inputs",
			inputs: TaskRunInputs{
				Resources: []TaskResourceBinding{{
					ResourceRef: PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace",
				}, {
					ResourceRef: PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace",
				}},
			},
			wantErr: apis.ErrMultipleOneOf("spec.Inputs.Resources.Name"),
		},
		{
			name: "invalid task input params",
			inputs: TaskRunInputs{
				Resources: []TaskResourceBinding{{
					ResourceRef: PipelineResourceRef{
						Name: "testresource",
					},
					Name: "resource",
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
		}, {
			name: "duplicate resource ref and resource spec",
			inputs: TaskRunInputs{
				Resources: []TaskResourceBinding{{
					ResourceRef: PipelineResourceRef{
						Name: "testresource",
					},
					ResourceSpec: &PipelineResourceSpec{
						Type: PipelineResourceTypeGit,
					},
					Name: "resource-dup",
				}},
			},
			wantErr: apis.ErrDisallowedFields("spec.Inputs.Resources.Name.ResourceRef", "spec.Inputs.Resources.Name.ResourceSpec"),
		}, {
			name: "invalid resource spec",
			inputs: TaskRunInputs{
				Resources: []TaskResourceBinding{{
					ResourceSpec: &PipelineResourceSpec{
						Type: "non-existent",
					},
					Name: "resource-inv",
				}},
			},
			wantErr: apis.ErrInvalidValue("spec.type", "non-existent"),
		}, {
			name: "no resource ref and resource spec",
			inputs: TaskRunInputs{
				Resources: []TaskResourceBinding{{
					Name: "resource",
				}},
			},
			wantErr: apis.ErrMissingField("spec.Inputs.Resources.Name.ResourceRef", "spec.Inputs.Resources.Name.ResourceSpec"),
		},
	}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.inputs.Validate(context.Background(), "spec.Inputs")
			if d := cmp.Diff(err.Error(), ts.wantErr.Error()); d != "" {
				t.Errorf("TaskRunInputs.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}

func TestOutput_Validate(t *testing.T) {
	i := TaskRunOutputs{
		Resources: []TaskResourceBinding{{
			ResourceRef: PipelineResourceRef{
				Name: "testresource",
			},
			Name: "someimage",
		}},
	}
	if err := i.Validate(context.Background(), "spec.outputs"); err != nil {
		t.Errorf("TaskRunOutputs.Validate() error = %v", err)
	}
}
func TestOutput_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		outputs TaskRunOutputs
		wantErr *apis.FieldError
	}{
		{
			name: "duplicated task outputs",
			outputs: TaskRunOutputs{
				Resources: []TaskResourceBinding{{
					ResourceRef: PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace",
				}, {
					ResourceRef: PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace",
				}},
			},
			wantErr: apis.ErrMultipleOneOf("spec.Outputs.Resources.Name"),
		}, {
			name: "no output resource with resource spec nor resource ref",
			outputs: TaskRunOutputs{
				Resources: []TaskResourceBinding{{
					Name: "workspace",
				}},
			},
			wantErr: apis.ErrMissingField("spec.Outputs.Resources.Name.ResourceSpec", "spec.Outputs.Resources.Name.ResourceRef"),
		},
	}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.outputs.Validate(context.Background(), "spec.Outputs")
			if d := cmp.Diff(err.Error(), ts.wantErr.Error()); d != "" {
				t.Errorf("TaskRunOutputs.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}
