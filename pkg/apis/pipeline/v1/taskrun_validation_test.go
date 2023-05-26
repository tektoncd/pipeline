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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	corev1resources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskRun_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		taskRun *v1.TaskRun
		want    *apis.FieldError
		wc      func(context.Context) context.Context
	}{{
		name:    "invalid taskspec",
		taskRun: &v1.TaskRun{},
		want: apis.ErrMissingOneOf("spec.taskRef", "spec.taskSpec").Also(
			apis.ErrGeneric(`invalid resource name "": must be a valid DNS label`, "metadata.name")),
	}, {
		name: "propagating params not provided but used by step",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
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
		wc: config.EnableBetaAPIFields,
	}, {
		name: "propagating object params not provided but used by step",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)"},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.task-words.hello)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating object properties not provided",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "task-words",
						Type: v1.ParamTypeObject,
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def"},
						},
					}},
					Steps: []v1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)"},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"spec.task-words.properties"},
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
		taskRun *v1.TaskRun
		wc      func(context.Context) context.Context
	}{{
		name: "propagating params with taskrun",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"hello", "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words[*])"},
					}},
				},
			},
		},
		wc: config.EnableBetaAPIFields,
	}, {
		name: "propagating object params from taskrun to steps",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating object params with taskrun mixing value types",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type: v1.ParamTypeObject,
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "task-words",
						Type: v1.ParamTypeObject,
						Properties: map[string]v1.PropertySpec{
							"hello": {Type: v1.ParamTypeString},
						},
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def"},
						},
					}},
					Steps: []v1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating partial params with different provided and default names",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"hello", "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "task-words-2",
						Type: v1.ParamTypeArray,
					}},
					Steps: []v1.Step{{
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
		wc: config.EnableBetaAPIFields,
	}, {
		name: "propagating object params with one declared in taskspec and other provided by taskrun",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "task-words-2",
						Type: v1.ParamTypeObject,
						Properties: map[string]v1.PropertySpec{
							"hello": {Type: v1.ParamTypeString},
						},
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def"},
						},
					}},
					Steps: []v1.Step{{
						Name:    "task-echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)", "$(params.task-words-2.hello)"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating partial params in taskrun",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"hello", "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "task-words-2",
						Type: v1.ParamTypeArray,
					}, {
						Name: "task-words",
						Type: v1.ParamTypeArray,
					}},
					Steps: []v1.Step{{
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
		wc: config.EnableBetaAPIFields,
	}, {
		name: "propagating partial object params with multiple keys",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "task-words",
						Type: v1.ParamTypeObject,
						Properties: map[string]v1.PropertySpec{
							"hello": {Type: v1.ParamTypeString},
							"hi":    {Type: v1.ParamTypeString},
						},
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def", "hi": "task"},
						},
					}},
					Steps: []v1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)", "$(params.task-words.hi)"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating params with taskrun same names",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"hello", "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "task-words",
						Type: v1.ParamTypeArray,
					}},
					Steps: []v1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words[*])"},
					}},
				},
			},
		},
		wc: config.EnableBetaAPIFields,
	}, {
		name: "object params without propagation",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "task-words",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "task-words",
						Type: v1.ParamTypeObject,
						Properties: map[string]v1.PropertySpec{
							"hello": {Type: v1.ParamTypeString},
						},
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def"},
						},
					}},
					Steps: []v1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)"},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "alpha feature: valid step and sidecar specs",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "task"},
				StepSpecs: []v1.TaskRunStepSpec{{
					Name: "foo",
					ComputeResources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
					},
				}},
				SidecarSpecs: []v1.TaskRunSidecarSpec{{
					Name: "bar",
					ComputeResources: corev1.ResourceRequirements{
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
		tr      *v1.TaskRun
		wantErr *apis.FieldError
	}{{
		name: "make sure WorkspaceBinding validation invoked",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskname"},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "task"},
				Workspaces: []v1.WorkspaceBinding{{
					Name:                  "workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
				}},
			},
		},
		wantErr: apis.ErrMissingField("spec.workspaces[0].persistentvolumeclaim.claimname"),
	}, {
		name: "bind same workspace twice",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskname"},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "task"},
				Workspaces: []v1.WorkspaceBinding{{
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

func EnableForbiddenEnv(ctx context.Context) context.Context {
	ctx = config.EnableAlphaAPIFields(ctx)
	c := config.FromContext(ctx)
	c.Defaults.DefaultForbiddenEnv = []string{"TEST_ENV"}
	return config.ToContext(ctx, c)
}

func TestTaskRunSpec_Invalidate(t *testing.T) {
	invalidStatusMessage := "status message without status"
	tests := []struct {
		name    string
		spec    v1.TaskRunSpec
		wantErr *apis.FieldError
		wc      func(context.Context) context.Context
	}{{
		name:    "invalid taskspec",
		spec:    v1.TaskRunSpec{},
		wantErr: apis.ErrMissingOneOf("taskRef", "taskSpec"),
	}, {
		name: "PodTmplate with forbidden env",
		spec: v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
			PodTemplate: &pod.Template{
				Env: []corev1.EnvVar{{
					Name:  "TEST_ENV",
					Value: "false",
				}},
			},
		},
		wc:      EnableForbiddenEnv,
		wantErr: apis.ErrInvalidValue("PodTemplate cannot update a forbidden env: TEST_ENV", "PodTemplate.Env"),
	}, {
		name: "invalid taskref and taskspec together",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "taskrefname",
			},
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("taskRef", "taskSpec"),
	}, {
		name: "negative pipeline timeout",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "taskrefname",
			},
			Timeout: &metav1.Duration{Duration: -48 * time.Hour},
		},
		wantErr: apis.ErrInvalidValue("-48h0m0s should be >= 0", "timeout"),
	}, {
		name: "wrong taskrun cancel",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "taskrefname",
			},
			Status: "TaskRunCancell",
		},
		wantErr: apis.ErrInvalidValue("TaskRunCancell should be TaskRunCancelled", "status"),
	}, {
		name: "incorrectly set statusMesage",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "taskrefname",
			},
			StatusMessage: v1.TaskRunSpecStatusMessage(invalidStatusMessage),
		},
		wantErr: apis.ErrInvalidValue(fmt.Sprintf("statusMessage should not be set if status is not set, but it is currently set to %s", invalidStatusMessage), "statusMessage"),
	}, {
		name: "invalid taskspec",
		spec: v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
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
		spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "myname",
				Value: *v1.NewStructuredValues("value"),
			}, {
				Name:  "myname",
				Value: *v1.NewStructuredValues("value"),
			}},
			TaskRef: &v1.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("params[myname].name"),
	}, {
		name: "invalid params - same names but different case",
		spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "FOO",
				Value: *v1.NewStructuredValues("value"),
			}, {
				Name:  "foo",
				Value: *v1.NewStructuredValues("value"),
			}},
			TaskRef: &v1.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("params[foo].name"),
	}, {
		name: "invalid params (object type) - same names but different case",
		spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "MYOBJECTPARAM",
				Value: *v1.NewObject(map[string]string{"key1": "val1", "key2": "val2"}),
			}, {
				Name:  "myobjectparam",
				Value: *v1.NewObject(map[string]string{"key1": "val1", "key2": "val2"}),
			}},
			TaskRef: &v1.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("params[myobjectparam].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "using debug when apifields stable",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1.TaskRunDebug{
				Breakpoint: []string{"onFailure"},
			},
		},
		wc:      config.EnableStableAPIFields,
		wantErr: apis.ErrGeneric("debug requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
	}, {
		name: "invalid breakpoint",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1.TaskRunDebug{
				Breakpoint: []string{"breakito"},
			},
		},
		wantErr: apis.ErrInvalidValue("breakito is not a valid breakpoint. Available valid breakpoints include [onFailure]", "debug.breakpoint"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "stepSpecs disallowed without alpha feature gate",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "foo",
			},
			StepSpecs: []v1.TaskRunStepSpec{{
				Name: "foo",
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wc:      config.EnableStableAPIFields,
		wantErr: apis.ErrGeneric("stepSpecs requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
	}, {
		name: "sidecarSpec disallowed without alpha feature gate",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "foo",
			},
			SidecarSpecs: []v1.TaskRunSidecarSpec{{
				Name: "foo",
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wc:      config.EnableStableAPIFields,
		wantErr: apis.ErrGeneric("sidecarSpecs requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
	}, {
		name: "duplicate stepSpecs names",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task"},
			StepSpecs: []v1.TaskRunStepSpec{{
				Name: "foo",
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}, {
				Name: "foo",
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrMultipleOneOf("stepSpecs[1].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "missing stepSpecs names",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task"},
			StepSpecs: []v1.TaskRunStepSpec{{
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrMissingField("stepSpecs[0].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "duplicate sidecarSpecs names",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task"},
			SidecarSpecs: []v1.TaskRunSidecarSpec{{
				Name: "bar",
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}, {
				Name: "bar",
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrMultipleOneOf("sidecarSpecs[1].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "missing sidecarSpecs names",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task"},
			SidecarSpecs: []v1.TaskRunSidecarSpec{{
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
				},
			}},
		},
		wantErr: apis.ErrMissingField("sidecarSpecs[0].name"),
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "invalid both step-level (stepSpecs.resources) and task-level (spec.computeResources) resource requirements",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task"},
			StepSpecs: []v1.TaskRunStepSpec{{
				Name: "stepSpec",
				ComputeResources: corev1.ResourceRequirements{
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
			"stepSpecs.resources",
			"computeResources",
		),
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "computeResources disallowed without alpha feature gate",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "foo",
			},
			ComputeResources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: corev1resources.MustParse("2Gi"),
				},
			},
		},
		wc:      config.EnableStableAPIFields,
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
		spec v1.TaskRunSpec
		wc   func(context.Context) context.Context
	}{{
		name: "taskspec without a taskRef",
		spec: v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
	}, {
		name: "no timeout",
		spec: v1.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: 0},
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
	}, {
		name: "parameters",
		spec: v1.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: 0},
			Params: v1.Params{{
				Name:  "name",
				Value: *v1.NewStructuredValues("value"),
			}},
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
	}, {
		name: "task spec with credentials.path variable",
		spec: v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:   "mystep",
					Image:  "myimage",
					Script: `echo "creds-init writes to $(credentials.path)"`,
				}},
			},
		},
	}, {
		name: "valid task-level (spec.resources) resource requirements",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task"},
			ComputeResources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: corev1resources.MustParse("2Gi"),
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "valid sidecar and task-level (spec.resources) resource requirements",
		spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task"},
			ComputeResources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: corev1resources.MustParse("2Gi"),
				},
			},
			SidecarSpecs: []v1.TaskRunSidecarSpec{{
				Name: "sidecar",
				ComputeResources: corev1.ResourceRequirements{
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
