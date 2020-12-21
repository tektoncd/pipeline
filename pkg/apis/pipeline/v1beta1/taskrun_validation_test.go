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

package v1beta1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskRun_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		taskRun *v1beta1.TaskRun
		want    *apis.FieldError
	}{{
		name:    "invalid taskspec",
		taskRun: &v1beta1.TaskRun{},
		want: apis.ErrMissingField("spec.taskref.name", "spec.taskspec").Also(
			apis.ErrGeneric(`invalid resource name "": must be a valid DNS label`, "metadata.name")),
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.taskRun.Validate(context.Background())
			if d := cmp.Diff(err.Error(), ts.want.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskRun_Validate(t *testing.T) {
	tests := []struct {
		name    string
		taskRun *v1beta1.TaskRun
		wc      func(context.Context) context.Context
	}{{
		name: "simple taskref",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrname",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{Name: "taskrefname"},
			},
		},
	}, {
		name: "do not validate spec on delete",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskrname"},
		},
		wc: apis.WithinDelete,
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			if err := ts.taskRun.Validate(ctx); err != nil {
				t.Errorf("TaskRun.Validate() error = %v", err)
			}
		})
	}
}

func TestTaskRun_Workspaces_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		tr      *v1beta1.TaskRun
		wantErr *apis.FieldError
	}{{
		name: "make sure WorkspaceBinding validation invoked",
		tr: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskname"},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{Name: "task"},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:                  "workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
				}},
			},
		},
		wantErr: apis.ErrMissingField("spec.workspaces[0].persistentvolumeclaim.claimname"),
	}, {
		name: "bind same workspace twice",
		tr: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskname"},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{Name: "task"},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:     "workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}, {
					Name:     "workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("spec.workspaces[1].name"),
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.tr.Validate(context.Background())
			if err == nil {
				t.Errorf("Expected error for invalid TaskRun but got none")
			}
			if d := cmp.Diff(ts.wantErr.Error(), err.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskRunSpec_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    v1beta1.TaskRunSpec
		wantErr *apis.FieldError
		wc      func(context.Context) context.Context
	}{{
		name:    "invalid taskspec",
		spec:    v1beta1.TaskRunSpec{},
		wantErr: apis.ErrMissingField("taskref.name", "taskspec"),
	}, {
		name: "invalid taskref name",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{},
		},
		wantErr: apis.ErrMissingField("taskref.name, taskspec"),
	}, {
		name: "invalid taskref and taskspec together",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "taskrefname",
			},
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Name:  "mystep",
					Image: "myimage",
				}}},
			},
		},
		wantErr: apis.ErrDisallowedFields("taskspec", "taskref"),
	}, {
		name: "negative pipeline timeout",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "taskrefname",
			},
			Timeout: &metav1.Duration{Duration: -48 * time.Hour},
		},
		wantErr: apis.ErrInvalidValue("-48h0m0s should be >= 0", "timeout"),
	}, {
		name: "wrong taskrun cancel",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "taskrefname",
			},
			Status: "TaskRunCancell",
		},
		wantErr: apis.ErrInvalidValue("TaskRunCancell should be TaskRunCancelled", "status"),
	}, {
		name: "invalid taskspec",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Name:  "invalid-name-with-$weird-char/%",
					Image: "myimage",
				}}},
			},
		},
		wantErr: &apis.FieldError{
			Message: `invalid value "invalid-name-with-$weird-char/%"`,
			Paths:   []string{"taskspec.steps[0].name"},
			Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}, {
		name: "invalid params",
		spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{{
				Name:  "myname",
				Value: *v1beta1.NewArrayOrString("value"),
			}, {
				Name:  "myname",
				Value: *v1beta1.NewArrayOrString("value"),
			}},
			TaskRef: &v1beta1.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("params[myname].name"),
	}, {
		name: "use of bundle without the feature flag set",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name:   "my-task",
				Bundle: "docker.io/foo",
			},
		},
		wantErr: apis.ErrDisallowedFields("taskref.bundle"),
	}, {
		name: "bundle missing name",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Bundle: "docker.io/foo",
			},
			TaskSpec: &v1beta1.TaskSpec{Steps: []v1beta1.Step{{Container: corev1.Container{Image: "foo"}}}},
		},
		wantErr: apis.ErrMissingField("taskref.name"),
		wc:      enableTektonOCIBundles(t),
	}, {
		name: "invalid bundle reference",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name:   "my-task",
				Bundle: "invalid reference",
			},
		},
		wantErr: apis.ErrInvalidValue("invalid bundle reference (could not parse reference: invalid reference)", "taskref.bundle"),
		wc:      enableTektonOCIBundles(t),
	}, {
		name: "using debug when apifields stable",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1beta1.TaskRunDebug{
				Breakpoint: []string{"bReaKdAnCe"},
			},
		},
		wantErr: apis.ErrDisallowedFields("debug"),
	}, {
		name: "invalid breakpoint",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1beta1.TaskRunDebug{
				Breakpoint: []string{"breakito"},
			},
		},
		wantErr: apis.ErrInvalidValue("breakito is not a valid breakpoint. Available valid breakpoints include [onFailure]", "debug.breakpoint"),
		wc:      enableAlphaAPIFields,
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			err := ts.spec.Validate(ctx)
			if d := cmp.Diff(ts.wantErr.Error(), err.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskRunSpec_Validate(t *testing.T) {
	tests := []struct {
		name string
		spec v1beta1.TaskRunSpec
	}{{
		name: "taskspec without a taskRef",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Name:  "mystep",
					Image: "myimage",
				}}},
			},
		},
	}, {
		name: "no timeout",
		spec: v1beta1.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: 0},
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Name:  "mystep",
					Image: "myimage",
				}}},
			},
		},
	}, {
		name: "parameters",
		spec: v1beta1.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: 0},
			Params: []v1beta1.Param{{
				Name:  "name",
				Value: *v1beta1.NewArrayOrString("value"),
			}},
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Name:  "mystep",
					Image: "myimage",
				}}},
			},
		},
	}, {
		name: "task spec with credentials.path variable",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Container: corev1.Container{
						Name:  "mystep",
						Image: "myimage",
					},
					Script: `echo "creds-init writes to $(credentials.path)"`,
				}},
			},
		},
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			if err := ts.spec.Validate(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestResources_Validate(t *testing.T) {
	tests := []struct {
		name      string
		resources *v1beta1.TaskRunResources
	}{{
		name: "no resources is valid",
	}, {
		name: "inputs only",
		resources: &v1beta1.TaskRunResources{
			Inputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource",
					},
					Name: "workspace",
				},
			}},
		},
	}, {
		name: "multiple inputs only",
		resources: &v1beta1.TaskRunResources{
			Inputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace1",
				},
			}, {
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace2",
				},
			}},
		},
	}, {
		name: "outputs only",
		resources: &v1beta1.TaskRunResources{
			Outputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource",
					},
					Name: "workspace",
				},
			}},
		},
	}, {
		name: "multiple outputs only",
		resources: &v1beta1.TaskRunResources{
			Outputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace1",
				},
			}, {
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace2",
				},
			}},
		},
	}, {
		name: "inputs and outputs",
		resources: &v1beta1.TaskRunResources{
			Inputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource",
					},
					Name: "workspace",
				},
			}},
			Outputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
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
		resources *v1beta1.TaskRunResources
		wantErr   *apis.FieldError
	}{{
		name: "duplicate task inputs",
		resources: &v1beta1.TaskRunResources{
			Inputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace",
				},
			}, {
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace",
				},
			}},
		},
		wantErr: apis.ErrMultipleOneOf("spec.resources.inputs.name"),
	}, {
		name: "duplicate resource ref and resource spec",
		resources: &v1beta1.TaskRunResources{
			Inputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource",
					},
					ResourceSpec: &resource.PipelineResourceSpec{
						Type: v1beta1.PipelineResourceTypeGit,
					},
					Name: "resource-dup",
				},
			}},
		},
		wantErr: apis.ErrDisallowedFields("spec.resources.inputs.name.resourceRef", "spec.resources.inputs.name.resourceSpec"),
	}, {
		name: "invalid resource spec",
		resources: &v1beta1.TaskRunResources{
			Inputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
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
		resources: &v1beta1.TaskRunResources{
			Inputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					Name: "resource",
				},
			}},
		},
		wantErr: apis.ErrMissingField("spec.resources.inputs.name.resourceRef", "spec.resources.inputs.name.resourceSpec"),
	}, {
		name: "duplicate task outputs",
		resources: &v1beta1.TaskRunResources{
			Outputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource1",
					},
					Name: "workspace",
				},
			}, {
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource2",
					},
					Name: "workspace",
				},
			}},
		},
		wantErr: apis.ErrMultipleOneOf("spec.resources.outputs.name"),
	}, {
		name: "duplicate resource ref and resource spec",
		resources: &v1beta1.TaskRunResources{
			Outputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "testresource",
					},
					ResourceSpec: &resource.PipelineResourceSpec{
						Type: v1beta1.PipelineResourceTypeGit,
					},
					Name: "resource-dup",
				},
			}},
		},
		wantErr: apis.ErrDisallowedFields("spec.resources.outputs.name.resourceRef", "spec.resources.outputs.name.resourceSpec"),
	}, {
		name: "invalid resource spec",
		resources: &v1beta1.TaskRunResources{
			Inputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
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
		resources: &v1beta1.TaskRunResources{
			Outputs: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
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
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
