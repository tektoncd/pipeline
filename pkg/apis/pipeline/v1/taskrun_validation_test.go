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

package v1_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	corev1resources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskRun_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		taskRun *v1beta1.TaskRun
		want    *apis.FieldError
		wc      func(context.Context) context.Context
	}{{
		name:    "invalid taskspec",
		taskRun: &v1beta1.TaskRun{},
		want: apis.ErrMissingOneOf("spec.taskRef", "spec.taskSpec").Also(
			apis.ErrGeneric(`invalid resource name "": must be a valid DNS label`, "metadata.name")),
	}, {
		name: "propagating params not provided but used by step",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words[*])"},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.task-words[*])"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
		wc: config.EnableAlphaAPIFields,
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			err := ts.taskRun.Validate(ctx)
			if d := cmp.Diff(ts.want.Error(), err.Error()); d != "" {
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
		name: "do not validate spec on delete",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskrname"},
		},
		wc: apis.WithinDelete,
	}, {
		name: "propagating params with taskrun",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: []v1beta1.Param{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						ArrayVal: []string{"hello", "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words[*])"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating partial params with different provided and default names",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: []v1beta1.Param{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						ArrayVal: []string{"hello", "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "task-words-2",
						Type: v1beta1.ParamTypeArray,
					}},
					Steps: []v1beta1.Step{{
						Name:    "task-echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words[*])"},
					}, {
						Name:    "task-echo-2",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words-2[*])"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating partial params in taskrun",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: []v1beta1.Param{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						ArrayVal: []string{"hello", "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "task-words-2",
						Type: v1beta1.ParamTypeArray,
					}, {
						Name: "task-words",
						Type: v1beta1.ParamTypeArray,
					}},
					Steps: []v1beta1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words[*])"},
					}, {
						Name:    "echo-2",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words-2[*])"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating params with taskrun same names",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: []v1beta1.Param{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						ArrayVal: []string{"hello", "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "task-words",
						Type: v1beta1.ParamTypeArray,
					}},
					Steps: []v1beta1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words[*])"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "alpha feature: valid step and sidecar overrides",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{Name: "task"},
				StepOverrides: []v1beta1.TaskRunStepOverride{{
					Name: "foo",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
					},
				}},
				SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
					Name: "bar",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
					},
				}},
			},
		},
		wc: config.EnableAlphaAPIFields,
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
	invalidStatusMessage := "status message without status"
	tests := []struct {
		name    string
		spec    v1beta1.TaskRunSpec
		wantErr *apis.FieldError
		wc      func(context.Context) context.Context
	}{{
		name:    "invalid taskspec",
		spec:    v1beta1.TaskRunSpec{},
		wantErr: apis.ErrMissingOneOf("taskRef", "taskSpec"),
	}, {
		name: "invalid taskref and taskspec together",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "taskrefname",
			},
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("taskRef", "taskSpec"),
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
		name: "incorrectly set statusMesage",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "taskrefname",
			},
			StatusMessage: v1beta1.TaskRunSpecStatusMessage(invalidStatusMessage),
		},
		wantErr: apis.ErrInvalidValue(fmt.Sprintf("statusMessage should not be set if status is not set, but it is currently set to %s", invalidStatusMessage), "statusMessage"),
	}, {
		name: "invalid taskspec",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "invalid-name-with-$weird-char/%",
					Image: "myimage",
				}},
			},
		},
		wantErr: &apis.FieldError{
			Message: `invalid value "invalid-name-with-$weird-char/%"`,
			Paths:   []string{"taskSpec.steps[0].name"},
			Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}, {
		name: "invalid params - exactly same names",
		spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{{
				Name:  "myname",
				Value: *v1beta1.NewStructuredValues("value"),
			}, {
				Name:  "myname",
				Value: *v1beta1.NewStructuredValues("value"),
			}},
			TaskRef: &v1beta1.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("params[myname].name"),
	}, {
		name: "invalid params - same names but different case",
		spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{{
				Name:  "FOO",
				Value: *v1beta1.NewStructuredValues("value"),
			}, {
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("value"),
			}},
			TaskRef: &v1beta1.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("params[foo].name"),
	}, {
		name: "invalid params (object type) - same names but different case",
		spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{{
				Name:  "MYOBJECTPARAM",
				Value: *v1beta1.NewObject(map[string]string{"key1": "val1", "key2": "val2"}),
			}, {
				Name:  "myobjectparam",
				Value: *v1beta1.NewObject(map[string]string{"key1": "val1", "key2": "val2"}),
			}},
			TaskRef: &v1beta1.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("params[myobjectparam].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "using debug when apifields stable",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1beta1.TaskRunDebug{
				Breakpoint: []string{"onFailure"},
			},
		},
		wantErr: apis.ErrGeneric("debug requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
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
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "stepOverride disallowed without alpha feature gate",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "foo",
			},
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrGeneric("stepOverrides requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
	}, {
		name: "sidecarOverride disallowed without alpha feature gate",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "foo",
			},
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrGeneric("sidecarOverrides requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
	}, {
		name: "duplicate stepOverride names",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}, {
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrMultipleOneOf("stepOverrides[1].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "missing stepOverride names",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrMissingField("stepOverrides[0].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "duplicate sidecarOverride names",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "bar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}, {
				Name: "bar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrMultipleOneOf("sidecarOverrides[1].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "missing sidecarOverride names",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrMissingField("sidecarOverrides[0].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "invalid both step-level (stepOverrides.resources) and task-level (spec.computeResources) resource requirements",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "stepOverride",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: corev1resources.MustParse("1Gi"),
					},
				},
			}},
			ComputeResources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: corev1resources.MustParse("2Gi"),
				},
			},
		},
		wantErr: apis.ErrMultipleOneOf(
			"stepOverrides.resources",
			"computeResources",
		),
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "computeResources disallowed without alpha feature gate",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "foo",
			},
			ComputeResources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: corev1resources.MustParse("2Gi"),
				},
			},
		},
		wantErr: apis.ErrGeneric("computeResources requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
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
		wc   func(context.Context) context.Context
	}{{
		name: "taskspec without a taskRef",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
	}, {
		name: "no timeout",
		spec: v1beta1.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: 0},
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
	}, {
		name: "parameters",
		spec: v1beta1.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: 0},
			Params: []v1beta1.Param{{
				Name:  "name",
				Value: *v1beta1.NewStructuredValues("value"),
			}},
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
	}, {
		name: "task spec with credentials.path variable",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:   "mystep",
					Image:  "myimage",
					Script: `echo "creds-init writes to $(credentials.path)"`,
				}},
			},
		},
	}, {
		name: "valid task-level (spec.resources) resource requirements",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			ComputeResources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: corev1resources.MustParse("2Gi"),
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "valid sidecar and task-level (spec.resources) resource requirements",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			ComputeResources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: corev1resources.MustParse("2Gi"),
				},
			},
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "sidecar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: corev1resources.MustParse("4Gi"),
					},
				},
			}},
		},
		wc: config.EnableAlphaAPIFields,
	}}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			if err := ts.spec.Validate(ctx); err != nil {
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
