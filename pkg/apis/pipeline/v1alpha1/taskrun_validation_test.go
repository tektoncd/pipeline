/*
Copyright 2019 The Tekton Authors.

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

package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestTaskRun_Invalidate(t *testing.T) {
	tests := []struct {
		name string
		task *v1alpha1.TaskRun
		want *apis.FieldError
	}{
		{
			name: "invalid taskspec",
			task: tb.TaskRun("taskmetaname", "default"),
			want: apis.ErrMissingField("spec"),
		},
		{
			name: "invalid taskrun metadata",
			task: tb.TaskRun("task.name", "default"),
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
	tr := tb.TaskRun("taskname", "default", tb.TaskRunSpec(
		tb.TaskRunTaskRef("taskrefname"),
	))
	if err := tr.Validate(context.Background()); err != nil {
		t.Errorf("TaskRun.Validate() error = %v", err)
	}
}

func TestTaskRunSpec_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    v1alpha1.TaskRunSpec
		wantErr *apis.FieldError
	}{
		{
			name:    "invalid taskspec",
			spec:    v1alpha1.TaskRunSpec{},
			wantErr: apis.ErrMissingField("spec"),
		},
		{
			name: "invalid taskref name",
			spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{},
			},
			wantErr: apis.ErrMissingField("spec.taskref.name, spec.taskspec"),
		},
		{
			name: "invalid taskref and taskspec together",
			spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "taskrefname",
				},
				TaskSpec: &v1alpha1.TaskSpec{
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
		spec v1alpha1.TaskRunSpec
	}{
		{
			name: "taskspec without a taskRef",
			spec: v1alpha1.TaskRunSpec{
				TaskSpec: &v1alpha1.TaskSpec{
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
	i := v1alpha1.TaskRunInputs{
		Params: []v1alpha1.Param{{
			Name:  "name",
			Value: "value",
		}},
		Resources: []v1alpha1.TaskResourceBinding{{
			ResourceRef: v1alpha1.PipelineResourceRef{
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
		inputs  v1alpha1.TaskRunInputs
		wantErr *apis.FieldError
	}{
		{
			name: "duplicate task inputs",
			inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace",
				}, {
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace",
				}},
			},
			wantErr: apis.ErrMultipleOneOf("spec.Inputs.Resources.Name"),
		},
		{
			name: "invalid task input params",
			inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "testresource",
					},
					Name: "resource",
				}},
				Params: []v1alpha1.Param{{
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
			inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "testresource",
					},
					ResourceSpec: &v1alpha1.PipelineResourceSpec{
						Type: v1alpha1.PipelineResourceTypeGit,
					},
					Name: "resource-dup",
				}},
			},
			wantErr: apis.ErrDisallowedFields("spec.Inputs.Resources.Name.ResourceRef", "spec.Inputs.Resources.Name.ResourceSpec"),
		}, {
			name: "invalid resource spec",
			inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceSpec: &v1alpha1.PipelineResourceSpec{
						Type: "non-existent",
					},
					Name: "resource-inv",
				}},
			},
			wantErr: apis.ErrInvalidValue("spec.type", "non-existent"),
		}, {
			name: "no resource ref and resource spec",
			inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
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
	i := v1alpha1.TaskRunOutputs{
		Resources: []v1alpha1.TaskResourceBinding{{
			ResourceRef: v1alpha1.PipelineResourceRef{
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
		outputs v1alpha1.TaskRunOutputs
		wantErr *apis.FieldError
	}{
		{
			name: "duplicated task outputs",
			outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace",
				}, {
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace",
				}},
			},
			wantErr: apis.ErrMultipleOneOf("spec.Outputs.Resources.Name"),
		}, {
			name: "no output resource with resource spec nor resource ref",
			outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
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
