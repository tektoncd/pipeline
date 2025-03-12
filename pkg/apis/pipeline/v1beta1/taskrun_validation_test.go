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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	corev1resources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func EnableForbiddenEnv(ctx context.Context) context.Context {
	c := config.FromContextOrDefaults(ctx)
	c.Defaults.DefaultForbiddenEnv = []string{"TEST_ENV"}
	return config.ToContext(ctx, c)
}

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
		name: "PodTmplate with forbidden env",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{
						Name:  "echo",
						Image: "ubuntu",
					}},
				},
				PodTemplate: &pod.Template{
					Env: []corev1.EnvVar{{
						Name:  "TEST_ENV",
						Value: "false",
					}},
				},
			},
		},
		wc:   EnableForbiddenEnv,
		want: apis.ErrInvalidValue("PodTemplate cannot update a forbidden env: TEST_ENV", "spec.PodTemplate.Env"),
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
	}, {
		name: "propagating object params not provided but used by step",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{
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
	}, {
		name: "propagating object properties not provided",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "task-words",
						Type: v1beta1.ParamTypeObject,
						Default: &v1beta1.ParamValue{
							Type:      v1beta1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def"},
						},
					}},
					Steps: []v1beta1.Step{{
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
	}, {
		name: "uses bundle (deprecated) on creation is disallowed",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunrunname",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name:   "foo",
					Bundle: "example.com/foo/bar",
				},
			},
		},
		want: &apis.FieldError{Message: "must not set the field(s)", Paths: []string{"spec.taskRef.bundle"}},
		wc:   apis.WithinCreate,
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
		name: "propagating params with taskrun",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:     v1beta1.ParamTypeArray,
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
	}, {
		name: "propagating object params with taskrun",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)"},
					}},
				},
			},
		},
	}, {
		name: "propagating object params with taskrun no value provided",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type: v1beta1.ParamTypeObject,
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "task-words",
						Type: v1beta1.ParamTypeObject,
						Properties: map[string]v1beta1.PropertySpec{
							"hello": {Type: v1beta1.ParamTypeString},
						},
						Default: &v1beta1.ParamValue{
							Type:      v1beta1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def"},
						},
					}},
					Steps: []v1beta1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)"},
					}},
				},
			},
		},
	}, {
		name: "propagating partial params with different provided and default names",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:     v1beta1.ParamTypeArray,
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
	}, {
		name: "propagating object params with one declared in taskspec and other provided by taskrun",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "task-words-2",
						Type: v1beta1.ParamTypeObject,
						Properties: map[string]v1beta1.PropertySpec{
							"hello": {Type: v1beta1.ParamTypeString},
						},
						Default: &v1beta1.ParamValue{
							Type:      v1beta1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def"},
						},
					}},
					Steps: []v1beta1.Step{{
						Name:    "task-echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)", "$(params.task-words-2.hello)"},
					}},
				},
			},
		},
	}, {
		name: "propagating partial params in taskrun",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:     v1beta1.ParamTypeArray,
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
	}, {
		name: "propagating partial object params with multiple keys",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "task-words",
						Type: v1beta1.ParamTypeObject,
						Properties: map[string]v1beta1.PropertySpec{
							"hello": {Type: v1beta1.ParamTypeString},
							"hi":    {Type: v1beta1.ParamTypeString},
						},
						Default: &v1beta1.ParamValue{
							Type:      v1beta1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def", "hi": "task"},
						},
					}},
					Steps: []v1beta1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)", "$(params.task-words.hi)"},
					}},
				},
			},
		},
	}, {
		name: "propagating params with taskrun same names",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:     v1beta1.ParamTypeArray,
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
	}, {
		name: "object params without propagation",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr"},
			Spec: v1beta1.TaskRunSpec{
				Params: v1beta1.Params{{
					Name: "task-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "task run"},
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "task-words",
						Type: v1beta1.ParamTypeObject,
						Properties: map[string]v1beta1.PropertySpec{
							"hello": {Type: v1beta1.ParamTypeString},
						},
						Default: &v1beta1.ParamValue{
							Type:      v1beta1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "task run def"},
						},
					}},
					Steps: []v1beta1.Step{{
						Name:    "echo",
						Image:   "ubuntu",
						Command: []string{"echo"},
						Args:    []string{"$(params.task-words.hello)"},
					}},
				},
			},
		},
	}, {
		name: "beta feature: valid step and sidecar overrides",
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
		wc: cfgtesting.EnableBetaAPIFields,
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
		wantErr: apis.ErrInvalidValue("statusMessage should not be set if status is not set, but it is currently set to "+invalidStatusMessage, "statusMessage"),
	}, {
		name: "taskspec when inline disabled",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
		wantErr: apis.ErrDisallowedFields("taskSpec"),
		wc: func(ctx context.Context) context.Context {
			return config.ToContext(ctx, &config.Config{
				FeatureFlags: &config.FeatureFlags{
					DisableInlineSpec: "taskrun",
				},
			})
		},
	}, {
		name: "taskspec when inline disabled all",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
		},
		wantErr: apis.ErrDisallowedFields("taskSpec"),
		wc: func(ctx context.Context) context.Context {
			return config.ToContext(ctx, &config.Config{
				FeatureFlags: &config.FeatureFlags{
					DisableInlineSpec: "taskrun,pipelinerun,pipeline",
				},
			})
		},
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
			Params: v1beta1.Params{{
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
			Params: v1beta1.Params{{
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
			Params: v1beta1.Params{{
				Name:  "MYOBJECTPARAM",
				Value: *v1beta1.NewObject(map[string]string{"key1": "val1", "key2": "val2"}),
			}, {
				Name:  "myobjectparam",
				Value: *v1beta1.NewObject(map[string]string{"key1": "val1", "key2": "val2"}),
			}},
			TaskRef: &v1beta1.TaskRef{Name: "mytask"},
		},
		wantErr: apis.ErrMultipleOneOf("params[myobjectparam].name"),
	}, {
		name: "using debug when apifields stable",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1beta1.TaskRunDebug{
				Breakpoints: &v1beta1.TaskBreakpoints{
					OnFailure: "enabled",
				},
			},
		},
		wc:      cfgtesting.EnableStableAPIFields,
		wantErr: apis.ErrGeneric("debug requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
	}, {
		name: "invalid onFailure breakpoint",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1beta1.TaskRunDebug{
				Breakpoints: &v1beta1.TaskBreakpoints{
					OnFailure: "turnOn",
				},
			},
		},
		wantErr: apis.ErrInvalidValue("turnOn is not a valid onFailure breakpoint value, onFailure breakpoint is only allowed to be set as enabled", "debug.breakpoints.onFailure"),
		wc:      cfgtesting.EnableAlphaAPIFields,
	}, {
		name: "invalid breakpoint duplicate before steps",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1beta1.TaskRunDebug{
				Breakpoints: &v1beta1.TaskBreakpoints{
					BeforeSteps: []string{"step-1", "step-1"},
					OnFailure:   "enabled",
				},
			},
		},
		wantErr: apis.ErrGeneric("before step must be unique, the same step: step-1 is defined multiple times at", "debug.breakpoints.beforeSteps[1]"),
		wc:      cfgtesting.EnableAlphaAPIFields,
	}, {
		name: "empty onFailure breakpoint",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "my-task",
			},
			Debug: &v1beta1.TaskRunDebug{
				Breakpoints: &v1beta1.TaskBreakpoints{
					OnFailure: "",
				},
			},
		},
		wantErr: apis.ErrInvalidValue("onFailure breakpoint is empty, it is only allowed to be set as enabled", "debug.breakpoints.onFailure"),
		wc:      cfgtesting.EnableAlphaAPIFields,
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
		wc:      cfgtesting.EnableAlphaAPIFields,
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
		wc:      cfgtesting.EnableAlphaAPIFields,
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
		wc:      cfgtesting.EnableAlphaAPIFields,
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
		wc:      cfgtesting.EnableAlphaAPIFields,
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
		wc: cfgtesting.EnableAlphaAPIFields,
	}, {
		name: "computeResources disallowed without beta feature gate",
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
		wc:      cfgtesting.EnableStableAPIFields,
		wantErr: apis.ErrGeneric("computeResources requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\""),
	}, {
		name: "uses resources",
		spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "foo",
			},
			Resources: &v1beta1.TaskRunResources{},
		},
		wantErr: apis.ErrDisallowedFields("resources"),
	}, {
		name: "uses resources in task spec",
		spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps:     []v1beta1.Step{{Image: "my-image"}},
				Resources: &v1beta1.TaskResources{},
			},
		},
		wantErr: apis.ErrDisallowedFields("resources").ViaField("taskSpec"),
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
			Params: v1beta1.Params{{
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
		wc: cfgtesting.EnableAlphaAPIFields,
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
		wc: cfgtesting.EnableBetaAPIFields,
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

func TestTaskRunSpec_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name            string
		isCreate        bool
		isUpdate        bool
		baselineTaskRun *v1beta1.TaskRun
		taskRun         *v1beta1.TaskRun
		expectedError   apis.FieldError
	}{
		{
			name: "is create ctx",
			taskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{},
			},
			isCreate:      true,
			isUpdate:      false,
			expectedError: apis.FieldError{},
		}, {
			name: "is update ctx, no changes",
			baselineTaskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Status: "TaskRunCancelled",
				},
			},
			taskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Status: "TaskRunCancelled",
				},
			},
			isCreate:      false,
			isUpdate:      true,
			expectedError: apis.FieldError{},
		}, {
			name:            "is update ctx, baseline is nil, skip validation",
			baselineTaskRun: nil,
			taskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Timeout: &metav1.Duration{Duration: 1},
				},
			},
			isCreate:      false,
			isUpdate:      true,
			expectedError: apis.FieldError{},
		}, {
			name: "is update ctx, baseline is unknown, only status changes",
			baselineTaskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Status:        "",
					StatusMessage: "",
				},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{Type: apis.ConditionSucceeded, Status: corev1.ConditionUnknown},
						},
					},
				},
			},
			taskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Status:        "TaskRunCancelled",
					StatusMessage: "TaskRun is cancelled",
				},
			},
			isCreate:      false,
			isUpdate:      true,
			expectedError: apis.FieldError{},
		}, {
			name: "is update ctx, baseline is unknown, status and timeout changes",
			baselineTaskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Status:        "",
					StatusMessage: "",
					Timeout:       &metav1.Duration{Duration: 0},
				},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{Type: apis.ConditionSucceeded, Status: corev1.ConditionUnknown},
						},
					},
				},
			},
			taskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Status:        "TaskRunCancelled",
					StatusMessage: "TaskRun is cancelled",
					Timeout:       &metav1.Duration{Duration: 1},
				},
			},
			isCreate: false,
			isUpdate: true,
			expectedError: apis.FieldError{
				Message: `invalid value: Once the TaskRun has started, only status and statusMessage updates are allowed`,
				Paths:   []string{""},
			},
		}, {
			name: "is update ctx, baseline is done, status changes",
			baselineTaskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Status: "",
				},
				Status: v1beta1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue},
						},
					},
				},
			},
			taskRun: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					Status: "TaskRunCancelled",
				},
			},
			isCreate: false,
			isUpdate: true,
			expectedError: apis.FieldError{
				Message: `invalid value: Once the TaskRun is complete, no updates are allowed`,
				Paths:   []string{""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := config.ToContext(context.Background(), &config.Config{
				FeatureFlags: &config.FeatureFlags{},
				Defaults:     &config.Defaults{},
			})
			if tt.isCreate {
				ctx = apis.WithinCreate(ctx)
			}
			if tt.isUpdate {
				ctx = apis.WithinUpdate(ctx, tt.baselineTaskRun)
			}
			tr := tt.taskRun
			err := tr.Spec.ValidateUpdate(ctx)
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskRunSpec.ValidateUpdate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
