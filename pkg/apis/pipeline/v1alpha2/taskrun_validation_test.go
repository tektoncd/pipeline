/*
Copyright 2019 The Tekton Authors

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

package v1alpha2_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskRun_Invalidate(t *testing.T) {
	tests := []struct {
		name string
		task *v1alpha2.TaskRun
		want *apis.FieldError
	}{{
		name: "invalid taskspec",
		task: &v1alpha2.TaskRun{},
		want: apis.ErrMissingField("spec"),
	}, {
		name: "invalid taskrun metadata",
		task: &v1alpha2.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "task.name",
			},
		},
		want: &apis.FieldError{
			Message: "Invalid resource name: special character . must not be present",
			Paths:   []string{"metadata.name"},
		},
	}}
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
	tr := &v1alpha2.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "taskrname",
		},
		Spec: v1alpha2.TaskRunSpec{
			TaskRef: &v1alpha2.TaskRef{Name: "taskrefname"},
		},
	}
	if err := tr.Validate(context.Background()); err != nil {
		t.Errorf("TaskRun.Validate() error = %v", err)
	}
}

func TestTaskRun_Workspaces_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		tr      *v1alpha2.TaskRun
		wantErr *apis.FieldError
	}{{
		name: "make sure WorkspaceBinding validation invoked",
		tr: &v1alpha2.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskname"},
			Spec: v1alpha2.TaskRunSpec{
				TaskRef: &v1alpha2.TaskRef{Name: "task"},
				Workspaces: []v1alpha2.WorkspaceBinding{{
					Name:                  "workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
				}},
			},
		},
		wantErr: apis.ErrMissingField("workspace.persistentvolumeclaim.claimname"),
	}, {
		name: "bind same workspace twice",
		tr: &v1alpha2.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskname"},
			Spec: v1alpha2.TaskRunSpec{
				TaskRef: &v1alpha2.TaskRef{Name: "task"},
				Workspaces: []v1alpha2.WorkspaceBinding{{
					Name:     "workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}, {
					Name:     "workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("spec.workspaces.name"),
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.tr.Validate(context.Background())
			if err == nil {
				t.Errorf("Expected error for invalid TaskRun but got none")
			}
			if d := cmp.Diff(ts.wantErr.Error(), err.Error()); d != "" {
				t.Errorf("TaskRunSpec.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}

func TestTaskRunSpec_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    v1alpha2.TaskRunSpec
		wantErr *apis.FieldError
	}{{
		name:    "invalid taskspec",
		spec:    v1alpha2.TaskRunSpec{},
		wantErr: apis.ErrMissingField("spec"),
	}, {
		name: "invalid taskref name",
		spec: v1alpha2.TaskRunSpec{
			TaskRef: &v1alpha2.TaskRef{},
		},
		wantErr: apis.ErrMissingField("spec.taskref.name, spec.taskspec"),
	}, {
		name: "invalid taskref and taskspec together",
		spec: v1alpha2.TaskRunSpec{
			TaskRef: &v1alpha2.TaskRef{
				Name: "taskrefname",
			},
			TaskSpec: &v1alpha2.TaskSpec{
				Steps: []v1alpha2.Step{{Container: corev1.Container{
					Name:  "mystep",
					Image: "myimage",
				}}},
			},
		},
		wantErr: apis.ErrDisallowedFields("spec.taskspec", "spec.taskref"),
	}, {
		name: "negative pipeline timeout",
		spec: v1alpha2.TaskRunSpec{
			TaskRef: &v1alpha2.TaskRef{
				Name: "taskrefname",
			},
			Timeout: &metav1.Duration{Duration: -48 * time.Hour},
		},
		wantErr: apis.ErrInvalidValue("-48h0m0s should be >= 0", "spec.timeout"),
	}, {
		name: "invalid taskspec",
		spec: v1alpha2.TaskRunSpec{
			TaskSpec: &v1alpha2.TaskSpec{
				Steps: []v1alpha2.Step{{Container: corev1.Container{
					Name:  "invalid-name-with-$weird-char/%",
					Image: "myimage",
				}}},
			},
		},
		wantErr: &apis.FieldError{
			Message: `invalid value "invalid-name-with-$weird-char/%"`,
			Paths:   []string{"taskspec.steps.name"},
			Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}, {
		name: "invalid params",
		spec: v1alpha2.TaskRunSpec{
			Params: []v1alpha1.Param{{
				Name:  "name",
				Value: *builder.ArrayOrString("value"),
			}, {
				Name:  "name",
				Value: *builder.ArrayOrString("value"),
			}},
			TaskRef: &v1alpha2.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("spec.inputs.params"),
	}}
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
		spec v1alpha2.TaskRunSpec
	}{{
		name: "taskspec without a taskRef",
		spec: v1alpha2.TaskRunSpec{
			TaskSpec: &v1alpha2.TaskSpec{
				Steps: []v1alpha2.Step{{Container: corev1.Container{
					Name:  "mystep",
					Image: "myimage",
				}}},
			},
		},
	}, {
		name: "no timeout",
		spec: v1alpha2.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: 0},
			TaskSpec: &v1alpha2.TaskSpec{
				Steps: []v1alpha2.Step{{Container: corev1.Container{
					Name:  "mystep",
					Image: "myimage",
				}}},
			},
		},
	}, {
		name: "parameters",
		spec: v1alpha2.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: 0},
			Params: []v1alpha2.Param{{
				Name:  "name",
				Value: *builder.ArrayOrString("value"),
			}},
			TaskSpec: &v1alpha2.TaskSpec{
				Steps: []v1alpha2.Step{{Container: corev1.Container{
					Name:  "mystep",
					Image: "myimage",
				}}},
			},
		},
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			if err := ts.spec.Validate(context.Background()); err != nil {
				t.Errorf("TaskRunSpec.Validate()/%s error = %v", ts.name, err)
			}
		})
	}
}

func TestResources_Validate(t *testing.T) {
	tests := []struct {
		name      string
		resources *v1alpha2.TaskRunResources
	}{{
		name: "no resources is valid",
	}, {
		name: "inputs only",
		resources: &v1alpha2.TaskRunResources{
			Inputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource",
					},
					Name: "workspace",
				},
			}},
		},
	}, {
		name: "multiple inputs only",
		resources: &v1alpha2.TaskRunResources{
			Inputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace1",
				},
			}, {
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace2",
				},
			}},
		},
	}, {
		name: "outputs only",
		resources: &v1alpha2.TaskRunResources{
			Outputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource",
					},
					Name: "workspace",
				},
			}},
		},
	}, {
		name: "multiple outputs only",
		resources: &v1alpha2.TaskRunResources{
			Outputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace1",
				},
			}, {
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace2",
				},
			}},
		},
	}, {
		name: "inputs and outputs",
		resources: &v1alpha2.TaskRunResources{
			Inputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource",
					},
					Name: "workspace",
				},
			}},
			Outputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource",
					},
					Name: "workspace",
				},
			}},
		},
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			if err := ts.resources.Validate(context.Background()); err != nil {
				t.Errorf("TaskRunInputs.Validate() error = %v", err)
			}
		})
	}

}

func TestResources_Invalidate(t *testing.T) {
	tests := []struct {
		name      string
		resources *v1alpha2.TaskRunResources
		wantErr   *apis.FieldError
	}{{
		name: "duplicate task inputs",
		resources: &v1alpha2.TaskRunResources{
			Inputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace",
				},
			}, {
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace",
				},
			}},
		},
		wantErr: apis.ErrMultipleOneOf("spec.resources.inputs.name"),
	}, {
		name: "duplicate resource ref and resource spec",
		resources: &v1alpha2.TaskRunResources{
			Inputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource",
					},
					ResourceSpec: &resource.PipelineResourceSpec{
						Type: v1alpha2.PipelineResourceTypeGit,
					},
					Name: "resource-dup",
				},
			}},
		},
		wantErr: apis.ErrDisallowedFields("spec.resources.inputs.name.resourceRef", "spec.resources.inputs.name.resourceSpec"),
	}, {
		name: "invalid resource spec",
		resources: &v1alpha2.TaskRunResources{
			Inputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceSpec: &resource.PipelineResourceSpec{
						Type: "non-existent",
					},
					Name: "resource-inv",
				},
			}},
		},
		wantErr: apis.ErrInvalidValue("spec.type", "non-existent"),
	}, {
		name: "no resource ref", // and resource spec
		resources: &v1alpha2.TaskRunResources{
			Inputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					Name: "resource",
				},
			}},
		},
		wantErr: apis.ErrMissingField("spec.resources.inputs.name.resourceRef", "spec.resources.inputs.name.resourceSpec"),
	}, {
		name: "duplicate task outputs",
		resources: &v1alpha2.TaskRunResources{
			Outputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace",
				},
			}, {
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace",
				},
			}},
		},
		wantErr: apis.ErrMultipleOneOf("spec.resources.outputs.name"),
	}, {
		name: "duplicate resource ref and resource spec",
		resources: &v1alpha2.TaskRunResources{
			Outputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceRef: &v1alpha2.PipelineResourceRef{
						Name: "testresource",
					},
					ResourceSpec: &resource.PipelineResourceSpec{
						Type: v1alpha2.PipelineResourceTypeGit,
					},
					Name: "resource-dup",
				},
			}},
		},
		wantErr: apis.ErrDisallowedFields("spec.resources.outputs.name.resourceRef", "spec.resources.outputs.name.resourceSpec"),
	}, {
		name: "invalid resource spec",
		resources: &v1alpha2.TaskRunResources{
			Inputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					ResourceSpec: &resource.PipelineResourceSpec{
						Type: "non-existent",
					},
					Name: "resource-inv",
				},
			}},
		},
		wantErr: apis.ErrInvalidValue("spec.type", "non-existent"),
	}, {
		name: "no resource ref ", // and resource spec
		resources: &v1alpha2.TaskRunResources{
			Outputs: []v1alpha2.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha2.PipelineResourceBinding{
					Name: "resource",
				},
			}},
		},
		wantErr: apis.ErrMissingField("spec.resources.outputs.name.resourceRef", "spec.resources.outputs.name.resourceSpec"),
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.resources.Validate(context.Background())
			if d := cmp.Diff(err.Error(), ts.wantErr.Error()); d != "" {
				t.Errorf("TaskRunInputs.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}
