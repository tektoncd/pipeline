/*
Copyright 2020 The Tekton Authors

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
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	corev1resources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineRun_Invalid(t *testing.T) {
	tests := []struct {
		name string
		pr   v1beta1.PipelineRun
		want *apis.FieldError
		wc   func(context.Context) context.Context
	}{{
		name: "no pipeline reference",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				ServiceAccountName: "foo",
			},
		},
		want: apis.ErrMissingOneOf("spec.pipelineRef", "spec.pipelineSpec"),
	}, {
		name: "PodTemplate contains forbidden environment variable.",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				ServiceAccountName: "foo",
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				PodTemplate: &pod.Template{
					Env: []corev1.EnvVar{{
						Name:  "TEST_ENV",
						Value: "false",
					}},
				},
			},
		},
		want: apis.ErrInvalidValue("PodTemplate cannot update a forbidden env: TEST_ENV", "spec.PodTemplate.Env"),
		wc:   EnableForbiddenEnv,
	}, {
		name: "invalid pipelinerun metadata",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun,name",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
		want: &apis.FieldError{
			Message: `invalid resource name "pipelinerun,name": must be a valid DNS label`,
			Paths:   []string{"metadata.name"},
		},
	}, {
		name: "negative pipeline timeout",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeout: &metav1.Duration{Duration: -48 * time.Hour},
			},
		},
		want: apis.ErrInvalidValue("-48h0m0s should be >= 0", "spec.timeout"),
	}, {
		name: "wrong pipelinerun cancel",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Status: "PipelineRunCancell",
			},
		},
		want: apis.ErrInvalidValue("PipelineRunCancell should be Cancelled, CancelledRunFinally, StoppedRunFinally or PipelineRunPending", "spec.status"),
	}, {
		name: "propagating params with pipelinespec and taskspec params not provided",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.random-words[*])"},
							}},
						}},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.random-words[*])"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "propagating object params with pipelinespec and taskspec params not provided",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "pipeline"},
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words.not-hello)"},
							}},
						}},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.pipeline-words.not-hello)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "propagating object params with pipelinespec and taskspec params not provided",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Params: []v1beta1.ParamSpec{{
								Name: "pipeline-words",
								Type: v1beta1.ParamTypeObject,
								Properties: map[string]v1beta1.PropertySpec{
									"hello": {Type: v1beta1.ParamTypeString},
								},
								Default: &v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeObject,
									ObjectVal: map[string]string{"hello": "taskspec"},
								},
							}},
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words.not-hello)"},
							}},
						}},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.pipeline-words.not-hello)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "propagating object params with pipelinespec and taskspec params provided in taskrun",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Params: v1beta1.Params{{
							Name: "pipeline-words",
							Value: v1beta1.ParamValue{
								Type:      v1beta1.ParamTypeObject,
								ObjectVal: map[string]string{"hello": "pipeline"},
							},
						}},
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Params: []v1beta1.ParamSpec{{
								Name: "pipeline-words",
								Type: v1beta1.ParamTypeObject,
								Properties: map[string]v1beta1.PropertySpec{
									"hello": {Type: v1beta1.ParamTypeString},
								},
								Default: &v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeObject,
									ObjectVal: map[string]string{"hello": "taskspec"},
								},
							}},
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words.not-hello)"},
							}},
						}},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.pipeline-words.not-hello)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "propagating params with pipelinespec and taskspec",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ArrayOrString{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"hello", "pipeline"},
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.random-words[*])"},
							}},
						}},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.random-words[*])"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "propagating object params with pipelinespec and taskspec",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "objectpipelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "pipeline"},
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "pipeline-words",
						Type: v1beta1.ParamTypeObject,
						Properties: map[string]v1beta1.PropertySpec{
							"hello": {Type: v1beta1.ParamTypeString},
						},
						Default: &v1beta1.ParamValue{
							Type:      v1beta1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "pipelinespec"},
						},
					}},
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						Params: v1beta1.Params{{
							Name: "pipeline-words",
							Value: v1beta1.ParamValue{
								Type:      v1beta1.ParamTypeObject,
								ObjectVal: map[string]string{"hello": "task"},
							},
						}},
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Params: []v1beta1.ParamSpec{{
								Name: "pipeline-words",
								Type: v1beta1.ParamTypeObject,
								Properties: map[string]v1beta1.PropertySpec{
									"hello": {Type: v1beta1.ParamTypeString},
								},
								Default: &v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeObject,
									ObjectVal: map[string]string{"hello": "taskspec"},
								},
							}},
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words.not-hello)"},
							}},
						}},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.pipeline-words.not-hello)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "duplicate param names",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "some-param",
					Value: v1beta1.ArrayOrString{
						ArrayVal: []string{"hello", "pipeline"},
					},
				}, {
					Name: "some-param",
					Value: v1beta1.ArrayOrString{
						ArrayVal: []string{"goodbye", "pipeline"},
					},
				}},
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
		want: &apis.FieldError{
			Message: "expected exactly one, got both",
			Paths:   []string{"spec.params[some-param].name"},
		},
	}, {
		name: "task result in string param value",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "some-param",
					Value: v1beta1.ArrayOrString{
						StringVal: "$(tasks.some-task.results.foo)",
						Type:      v1beta1.ParamTypeString,
					},
				}},
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
		want: &apis.FieldError{
			Message: "invalid value: cannot use result expressions in [tasks.some-task.results.foo] as PipelineRun parameter values",
			Paths:   []string{"spec.params[some-param].value"},
		},
	}, {
		name: "task result in array param value",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "some-param",
					Value: v1beta1.ArrayOrString{
						ArrayVal: []string{"$(tasks.some-task.results.foo)"},
						Type:     v1beta1.ParamTypeArray,
					},
				}},
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
		want: &apis.FieldError{
			Message: "invalid value: cannot use result expressions in [tasks.some-task.results.foo] as PipelineRun parameter values",
			Paths:   []string{"spec.params[some-param].value"},
		},
	}, {
		name: "params with pipelinespec and taskspec",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ArrayOrString{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"hello", "pipeline"},
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "pipeline-words",
						Type: v1beta1.ParamTypeArray,
					}},
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						Params: v1beta1.Params{{
							Name: "pipeline-words",
							Value: v1beta1.ArrayOrString{
								Type:     v1beta1.ParamTypeArray,
								ArrayVal: []string{"$(params.pipeline-words)"},
							},
						}},
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Params: []v1beta1.ParamSpec{{
								Name: "pipeline-words",
								Type: v1beta1.ParamTypeArray,
							}},
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.random-words[*])"},
							}},
						}},
					}},
				},
			},
		},
		want: &apis.FieldError{
			Message: `non-existent variable in "$(params.random-words[*])"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "pipelinerun pending while running",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerunname",
			},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusPending,
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: time.Now()},
				},
			},
		},
		want: &apis.FieldError{
			Message: "invalid value: PipelineRun cannot be Pending after it is started",
			Paths:   []string{"spec.status"},
		},
	}, {
		name: "uses resources",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerunname",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
				Resources:   []v1beta1.PipelineResourceBinding{{Name: "bar"}},
			},
		},
		want: &apis.FieldError{Message: "must not set the field(s)", Paths: []string{"spec.resources"}},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.wc != nil {
				ctx = tc.wc(ctx)
			}
			err := tc.pr.Validate(ctx)
			if d := cmp.Diff(tc.want.Error(), err.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRun_Validate(t *testing.T) {
	tests := []struct {
		name string
		pr   v1beta1.PipelineRun
		wc   func(context.Context) context.Context
	}{{
		name: "normal case",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
	}, {
		name: "no timeout",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeout: &metav1.Duration{Duration: 0},
			},
		},
	}, {
		name: "array param with pipelinespec and taskspec",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ArrayOrString{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"hello", "pipeline"},
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "pipeline-words-2",
						Type: v1beta1.ParamTypeArray,
					}},
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						Params: v1beta1.Params{{
							Name: "task-words",
							Value: v1beta1.ParamValue{
								Type:     v1beta1.ParamTypeArray,
								ArrayVal: []string{"$(params.pipeline-words)"},
							},
						}},
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Params: []v1beta1.ParamSpec{{
								Name: "task-words-2",
								Type: v1beta1.ParamTypeArray,
							}},
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words[*])"},
							}, {
								Name:    "echo-2",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words-2[*])"},
							}, {
								Name:    "echo-3",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.task-words[*])"},
							}, {
								Name:    "echo-4",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.task-words-2[*])"},
							}},
						}},
					}},
				},
			},
		},
	}, {
		name: "propagating params with pipelinespec and taskspec",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ArrayOrString{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"hello", "pipeline"},
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words[*])"},
							}},
						}},
					}},
				},
			},
		},
	}, {
		name: "propagating object params with pipelinespec and taskspec",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "pipeline"},
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words.hello)"},
							}},
						}},
					}},
				},
			},
		},
	}, {
		name: "propagating object params no value in params but value in default",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ParamValue{
						Type: v1beta1.ParamTypeObject,
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "pipeline-words",
						Type: v1beta1.ParamTypeObject,
						Properties: map[string]v1beta1.PropertySpec{
							"hello": {Type: v1beta1.ParamTypeString},
						},
						Default: &v1beta1.ParamValue{
							Type:      v1beta1.ParamTypeObject,
							ObjectVal: map[string]string{"hello": "pipelinespec"},
						},
					}},
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words.hello)"},
							}},
						}},
					}},
				},
			},
		},
	}, {
		name: "propagating object params with some params defined in taskspec only",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Params: v1beta1.Params{{
					Name: "pipeline-words",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"hello": "pipeline"},
					},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Params: []v1beta1.ParamSpec{{
								Name: "task-words",
								Type: v1beta1.ParamTypeObject,
								Properties: map[string]v1beta1.PropertySpec{
									"hello": {Type: v1beta1.ParamTypeString},
								},
								Default: &v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeObject,
									ObjectVal: map[string]string{"hello": "taskspec"},
								},
							}},
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(params.pipeline-words.hello)", "$(params.task-words.hello)"},
							}},
						}},
					}},
				},
			},
		},
	}, {
		name: "propagating workspaces",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:     "ws",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(workspaces.ws.path)"},
							}},
						}},
					}},
					Finally: []v1beta1.PipelineTask{{
						Name: "echoitifinally",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(workspaces.ws.path)"},
							}},
						}},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating workspaces partially defined",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:     "ws",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
						Name: "ws",
					}},
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(workspaces.ws.path)"},
							}},
						}},
					}},
					Finally: []v1beta1.PipelineTask{{
						Name: "echoitfinally",
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(workspaces.ws.path)"},
							}},
						}},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "propagating workspaces fully defined",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:     "ws",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
				PipelineSpec: &v1beta1.PipelineSpec{
					Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
						Name: "ws",
					}},
					Tasks: []v1beta1.PipelineTask{{
						Name: "echoit",
						Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
							Name:    "ws",
							SubPath: "/foo",
						}},
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Workspaces: []v1beta1.WorkspaceDeclaration{{
								Name: "ws",
							}},
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(workspaces.ws.path)"},
							}},
						}},
					}},
					Finally: []v1beta1.PipelineTask{{
						Name: "echoitfinally",
						Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
							Name:    "ws",
							SubPath: "/foo",
						}},
						TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
							Workspaces: []v1beta1.WorkspaceDeclaration{{
								Name: "ws",
							}},
							Steps: []v1beta1.Step{{
								Name:    "echo",
								Image:   "ubuntu",
								Command: []string{"echo"},
								Args:    []string{"$(workspaces.ws.path)"},
							}},
						}},
					}},
				},
			},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "pipelinerun pending",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerunname",
			},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusPending,
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
	}, {
		name: "pipelinerun cancelled",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerunname",
			},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
	}, {
		name: "pipelinerun gracefully cancelled",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerunname",
			},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelledRunFinally,
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
	}, {
		name: "pipelinerun gracefully stopped",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerunname",
			},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusStoppedRunFinally,
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
			},
		},
	}, {
		name: "alpha feature: sidecar and step overrides",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "pr"},
				TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
					{
						PipelineTaskName: "bar",
						StepOverrides: []v1beta1.TaskRunStepOverride{{
							Name: "task-1",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
							}},
						},
						SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
							Name: "task-1",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
							}},
						},
					},
				},
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
			if err := ts.pr.Validate(ctx); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestPipelineRunSpec_Invalidate(t *testing.T) {
	tests := []struct {
		name        string
		spec        v1beta1.PipelineRunSpec
		wantErr     *apis.FieldError
		withContext func(context.Context) context.Context
	}{{
		name: "pipelineRef and pipelineSpec together",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "pipelinerefname",
			},
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "mytask",
					TaskRef: &v1beta1.TaskRef{
						Name: "mytask",
					},
				}}},
		},
		wantErr: apis.ErrMultipleOneOf("pipelineRef", "pipelineSpec"),
	}, {
		name: "workspaces may only appear once",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "pipelinerefname",
			},
			Workspaces: []v1beta1.WorkspaceBinding{{
				Name:     "ws",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}, {
				Name:     "ws",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}},
		},
		wantErr: &apis.FieldError{
			Message: `workspace "ws" provided by pipelinerun more than once, at index 0 and 1`,
			Paths:   []string{"workspaces[1].name"},
		},
	}, {
		name: "workspaces must contain a valid volume config",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "pipelinerefname",
			},
			Workspaces: []v1beta1.WorkspaceBinding{{
				Name: "ws",
			}},
		},
		wantErr: &apis.FieldError{
			Message: "expected exactly one, got neither",
			Paths: []string{
				"workspaces[0].configmap",
				"workspaces[0].emptydir",
				"workspaces[0].persistentvolumeclaim",
				"workspaces[0].secret",
				"workspaces[0].volumeclaimtemplate",
			},
		},
	}, {
		name: "duplicate stepOverride names",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "bar",
					StepOverrides: []v1beta1.TaskRunStepOverride{
						{Name: "baz"}, {Name: "baz"},
					},
				},
			},
		},
		wantErr:     apis.ErrMultipleOneOf("taskRunSpecs[0].stepOverrides[1].name"),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "stepOverride disallowed without alpha feature gate",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "bar",
					StepOverrides: []v1beta1.TaskRunStepOverride{{
						Name: "task-1",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
						}},
					},
				},
			},
		},
		withContext: config.EnableStableAPIFields,
		wantErr:     apis.ErrGeneric("stepOverrides requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\"").ViaIndex(0).ViaField("taskRunSpecs"),
	}, {
		name: "sidecarOverride disallowed without alpha feature gate",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "bar",
					SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
						Name: "task-1",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
						}},
					},
				},
			},
		},
		withContext: config.EnableStableAPIFields,
		wantErr:     apis.ErrGeneric("sidecarOverrides requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\"").ViaIndex(0).ViaField("taskRunSpecs"),
	}, {
		name: "missing stepOverride name",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "bar",
					StepOverrides: []v1beta1.TaskRunStepOverride{{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
						}},
					},
				},
			},
		},
		wantErr:     apis.ErrMissingField("taskRunSpecs[0].stepOverrides[0].name"),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "duplicate sidecarOverride names",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "bar",
					SidecarOverrides: []v1beta1.TaskRunSidecarOverride{
						{Name: "baz"}, {Name: "baz"},
					},
				},
			},
		},
		wantErr:     apis.ErrMultipleOneOf("taskRunSpecs[0].sidecarOverrides[1].name"),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "missing sidecarOverride name",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "bar",
					SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
						}},
					},
				},
			},
		},
		wantErr:     apis.ErrMissingField("taskRunSpecs[0].sidecarOverrides[0].name"),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "invalid both step-level (stepOverrides.resources) and task-level (taskRunSpecs.resources) resource requirements configured",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "pipeline"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "pipelineTask",
					StepOverrides: []v1beta1.TaskRunStepOverride{{
						Name: "stepOverride",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
						}},
					},
					ComputeResources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("2Gi")},
					},
				},
			},
		},
		wantErr: apis.ErrMultipleOneOf(
			"taskRunSpecs[0].stepOverrides.resources",
			"taskRunSpecs[0].computeResources",
		),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "computeResources disallowed without alpha feature gate",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "bar",
					ComputeResources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("2Gi")},
					},
				},
			},
		},
		withContext: config.EnableStableAPIFields,
		wantErr:     apis.ErrGeneric("computeResources requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\"").ViaIndex(0).ViaField("taskRunSpecs"),
	}}

	for _, ps := range tests {
		t.Run(ps.name, func(t *testing.T) {
			ctx := context.Background()
			if ps.withContext != nil {
				ctx = ps.withContext(ctx)
			}
			err := ps.spec.Validate(ctx)
			if d := cmp.Diff(ps.wantErr.Error(), err.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunSpec_Validate(t *testing.T) {
	tests := []struct {
		name        string
		spec        v1beta1.PipelineRunSpec
		withContext func(context.Context) context.Context
	}{{
		name: "PipelineRun without pipelineRef",
		spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "mytask",
					TaskRef: &v1beta1.TaskRef{
						Name: "mytask",
					},
				}},
			},
		},
	}, {
		name: "valid task-level (taskRunSpecs.resources) resource requirements configured",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "pipeline"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{{
				PipelineTaskName: "pipelineTask",
				StepOverrides: []v1beta1.TaskRunStepOverride{{
					Name: "stepOverride",
				}},
				ComputeResources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("2Gi")},
				},
			}},
		},
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "valid sidecar and task-level (taskRunSpecs.resources) resource requirements configured",
		spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "pipeline"},
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{{
				PipelineTaskName: "pipelineTask",
				StepOverrides: []v1beta1.TaskRunStepOverride{{
					Name: "stepOverride",
				}},
				ComputeResources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("2Gi")},
				},
				SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
					Name: "sidecar",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: corev1resources.MustParse("4Gi"),
						},
					},
				}},
			}},
		},
		withContext: config.EnableAlphaAPIFields,
	}}

	for _, ps := range tests {
		t.Run(ps.name, func(t *testing.T) {
			ctx := context.Background()
			if ps.withContext != nil {
				ctx = ps.withContext(ctx)
			}
			if err := ps.spec.Validate(ctx); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestPipelineRun_InvalidTimeouts(t *testing.T) {
	tests := []struct {
		name string
		pr   v1beta1.PipelineRun
		want *apis.FieldError
	}{{
		name: "negative pipeline timeouts",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: -48 * time.Hour},
				},
			},
		},
		want: apis.ErrInvalidValue("-48h0m0s should be >= 0", "spec.timeouts.pipeline"),
	}, {
		name: "negative pipeline tasks Timeout",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Tasks: &metav1.Duration{Duration: -48 * time.Hour},
				},
			},
		},
		want: apis.ErrInvalidValue("-48h0m0s should be >= 0", "spec.timeouts.tasks"),
	}, {
		name: "negative pipeline finally Timeout",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Finally: &metav1.Duration{Duration: -48 * time.Hour},
				},
			},
		},
		want: apis.ErrInvalidValue("-48h0m0s should be >= 0", "spec.timeouts.finally"),
	}, {
		name: "pipeline tasks Timeout > pipeline Timeout",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 25 * time.Minute},
					Tasks:    &metav1.Duration{Duration: 1 * time.Hour},
				},
			},
		},
		want: apis.ErrInvalidValue("1h0m0s should be <= pipeline duration", "spec.timeouts.tasks"),
	}, {
		name: "pipeline finally Timeout > pipeline Timeout",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 25 * time.Minute},
					Finally:  &metav1.Duration{Duration: 1 * time.Hour},
				},
			},
		},
		want: apis.ErrInvalidValue("1h0m0s should be <= pipeline duration", "spec.timeouts.finally"),
	}, {
		name: "pipeline tasks Timeout +  pipeline finally Timeout > pipeline Timeout",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 50 * time.Minute},
					Tasks:    &metav1.Duration{Duration: 30 * time.Minute},
					Finally:  &metav1.Duration{Duration: 30 * time.Minute},
				},
			},
		},
		want: apis.ErrInvalidValue("30m0s + 30m0s should be <= pipeline duration", "spec.timeouts.finally, spec.timeouts.tasks"),
	}, {
		name: "Tasks timeout = 0 but Pipeline timeout not set",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Tasks: &metav1.Duration{Duration: 0 * time.Minute},
				},
			},
		},
		want: apis.ErrInvalidValue(`0s (no timeout) should be <= default timeout duration`, "spec.timeouts.tasks"),
	}, {
		name: "Tasks timeout = 0 but Pipeline timeout is not 0",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 10 * time.Minute},
					Tasks:    &metav1.Duration{Duration: 0 * time.Minute},
				},
			},
		},
		want: apis.ErrInvalidValue(`0s (no timeout) should be <= pipeline duration`, "spec.timeouts.tasks"),
	}, {
		name: "Finally timeout = 0 but Pipeline timeout not set",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Finally: &metav1.Duration{Duration: 0 * time.Minute},
				},
			},
		},
		want: apis.ErrInvalidValue(`0s (no timeout) should be <= default timeout duration`, "spec.timeouts.finally"),
	}, {
		name: "Finally timeout = 0 but Pipeline timeout is not 0",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 10 * time.Minute},
					Finally:  &metav1.Duration{Duration: 0 * time.Minute},
				},
			},
		},
		want: apis.ErrInvalidValue(`0s (no timeout) should be <= pipeline duration`, "spec.timeouts.finally"),
	}, {
		name: "Timeout and Timeouts both are set",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeout: &metav1.Duration{Duration: 10 * time.Minute},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 10 * time.Minute},
				},
			},
		},
		want: apis.ErrDisallowedFields("spec.timeout", "spec.timeouts"),
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			err := tc.pr.Validate(ctx)
			if d := cmp.Diff(err.Error(), tc.want.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunWithTimeout_Validate(t *testing.T) {
	tests := []struct {
		name string
		pr   v1beta1.PipelineRun
		wc   func(context.Context) context.Context
	}{{
		name: "no tasksTimeout",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 0},
					Tasks:    &metav1.Duration{Duration: 0},
					Finally:  &metav1.Duration{Duration: 0},
				},
			},
		},
	}, {
		name: "Timeouts set for all three Task, Finally and Pipeline",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
					Tasks:    &metav1.Duration{Duration: 30 * time.Minute},
					Finally:  &metav1.Duration{Duration: 30 * time.Minute},
				},
			},
		},
	}, {
		name: "timeouts.tasks only",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 0 * time.Hour},
					Tasks:    &metav1.Duration{Duration: 30 * time.Minute},
				},
			},
		},
	}, {
		name: "timeouts.finally only",
		pr: v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinelinename",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "prname",
				},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 0 * time.Hour},
					Finally:  &metav1.Duration{Duration: 30 * time.Minute},
				},
			},
		},
	}}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			if err := ts.pr.Validate(ctx); err != nil {
				t.Error(err)
			}
		})
	}
}
