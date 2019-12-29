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

package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

var validResource = v1alpha1.TaskResource{
	ResourceDeclaration: v1alpha1.ResourceDeclaration{
		Name: "source",
		Type: "git",
	},
}

var validImageResource = v1alpha1.TaskResource{
	ResourceDeclaration: v1alpha1.ResourceDeclaration{
		Name: "source",
		Type: "image",
	},
}

var invalidResource = v1alpha1.TaskResource{
	ResourceDeclaration: v1alpha1.ResourceDeclaration{
		Name: "source",
		Type: "what",
	},
}

var optionalResource = v1alpha1.TaskResource{
	ResourceDeclaration: v1alpha1.ResourceDeclaration{
		Name:     "source",
		Type:     "git",
		Optional: true,
	},
}

var requiredResource = v1alpha1.TaskResource{
	ResourceDeclaration: v1alpha1.ResourceDeclaration{
		Name:     "source",
		Type:     "git",
		Optional: false,
	},
}

var validSteps = []v1alpha1.Step{{Container: corev1.Container{
	Name:  "mystep",
	Image: "myimage",
}}}

var invalidSteps = []v1alpha1.Step{{Container: corev1.Container{
	Name:  "replaceImage",
	Image: "myimage",
}}}

func TestTaskSpecValidate(t *testing.T) {
	type fields struct {
		Inputs       *v1alpha1.Inputs
		Outputs      *v1alpha1.Outputs
		Steps        []v1alpha1.Step
		StepTemplate *corev1.Container
		Workspaces   []v1alpha1.WorkspaceDeclaration
	}
	tests := []struct {
		name   string
		fields fields
	}{{
		name: "unnamed steps",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Image: "myimage",
			}}, {Container: corev1.Container{
				Image: "myotherimage",
			}}},
		},
	}, {
		name: "valid inputs (type implied)",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
				Params: []v1alpha1.ParamSpec{{
					Name:        "task",
					Description: "param",
					Default:     builder.ArrayOrString("default"),
				}},
			},
			Steps: validSteps,
		},
	}, {
		name: "valid inputs type explicit",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
				Params: []v1alpha1.ParamSpec{{
					Name:        "task",
					Type:        v1alpha1.ParamTypeString,
					Description: "param",
					Default:     builder.ArrayOrString("default"),
				}},
			},
			Steps: validSteps,
		},
	}, {
		name: "valid inputs (required)",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{requiredResource},
			},
			Steps: validSteps,
		},
	}, {
		name: "valid inputs (optional)",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{optionalResource},
			},
			Steps: validSteps,
		},
	}, {
		name: "valid outputs",
		fields: fields{
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: validSteps,
		},
	}, {
		name: "both valid",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: validSteps,
		},
	}, {
		name: "output image resoure",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validImageResource},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validImageResource},
			},
			Steps: validSteps,
		},
	}, {
		name: "valid outputs (required)",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{requiredResource},
			},
			Steps: validSteps,
		},
	}, {
		name: "valid outputs (optional)",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{optionalResource},
			},
			Steps: validSteps,
		},
	}, {
		name: "valid template variable",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validImageResource},
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
				}, {
					Name: "foo-is-baz",
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "mystep",
				Image:      "$(inputs.resources.source.url)",
				Args:       []string{"--flag=$(inputs.params.baz) && $(input.params.foo-is-baz)"},
				WorkingDir: "/foo/bar/$(outputs.resources.source)",
			}}},
		},
	}, {
		name: "valid array template variable",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validImageResource},
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
					Type: v1alpha1.ParamTypeArray,
				}, {
					Name: "foo-is-baz",
					Type: v1alpha1.ParamTypeArray,
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "mystep",
				Image:      "$(inputs.resources.source.url)",
				Command:    []string{"$(inputs.param.foo-is-baz)"},
				Args:       []string{"$(inputs.params.baz)", "middle string", "$(input.params.foo-is-baz)"},
				WorkingDir: "/foo/bar/$(outputs.resources.source)",
			}}},
		},
	}, {
		name: "step template included in validation",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "astep",
				Command: []string{"echo"},
				Args:    []string{"hello"},
			}}},
			StepTemplate: &corev1.Container{
				Image: "some-image",
			},
		},
	}, {
		name: "valid step with script",
		fields: fields{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Image: "my-image",
				},
				Script: `
				#!/usr/bin/env bash
				hello world`,
			}},
		},
	}, {
		name: "valid step with script and args",
		fields: fields{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Image: "my-image",
					Args:  []string{"arg"},
				},
				Script: `
				#!/usr/bin/env  bash
				hello $1`,
			}},
		},
	}, {
		name: "valid step with volumeMount under /tekton/home",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Image: "myimage",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "foo",
					MountPath: "/tekton/home",
				}},
			}}},
		},
	}, {
		name: "valid workspace",
		fields: fields{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Image: "my-image",
					Args:  []string{"arg"},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:        "foo-workspace",
				Description: "my great workspace",
				MountPath:   "some/path",
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1alpha1.TaskSpec{
				Inputs:       tt.fields.Inputs,
				Outputs:      tt.fields.Outputs,
				Steps:        tt.fields.Steps,
				StepTemplate: tt.fields.StepTemplate,
				Workspaces:   tt.fields.Workspaces,
			}
			ctx := context.Background()
			ts.SetDefaults(ctx)
			if err := ts.Validate(ctx); err != nil {
				t.Errorf("TaskSpec.Validate() = %v", err)
			}
		})
	}
}

func TestTaskSpecValidateError(t *testing.T) {
	type fields struct {
		Inputs       *v1alpha1.Inputs
		Outputs      *v1alpha1.Outputs
		Steps        []v1alpha1.Step
		Volumes      []corev1.Volume
		StepTemplate *corev1.Container
		Workspaces   []v1alpha1.WorkspaceDeclaration
	}
	tests := []struct {
		name          string
		fields        fields
		expectedError apis.FieldError
	}{{
		name: "nil",
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{""},
		},
	}, {
		name: "no build",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"steps"},
		},
	}, {
		name: "one invalid input",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{
					invalidResource,
					validResource,
				},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{
					validResource,
				},
			},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value: what`,
			Paths:   []string{"taskspec.Inputs.Resources.source.Type"},
		},
	}, {
		name: "invalid input type",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
				Params: []v1alpha1.ParamSpec{{
					Name:        "validparam",
					Type:        v1alpha1.ParamTypeString,
					Description: "parameter",
					Default:     builder.ArrayOrString("default"),
				}, {
					Name:        "param-with-invalid-type",
					Type:        "invalidtype",
					Description: "invalidtypedesc",
					Default:     builder.ArrayOrString("default"),
				}},
			},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value: invalidtype`,
			Paths:   []string{"taskspec.inputs.params.param-with-invalid-type.type"},
		},
	}, {
		name: "input mismatching default/type 1",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
				Params: []v1alpha1.ParamSpec{{
					Name:        "task",
					Type:        v1alpha1.ParamTypeArray,
					Description: "param",
					Default:     builder.ArrayOrString("default"),
				}},
			},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `"array" type does not match default value's type: "string"`,
			Paths:   []string{"taskspec.inputs.params.task.type", "taskspec.inputs.params.task.default.type"},
		},
	}, {
		name: "input mismatching default/type 2",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
				Params: []v1alpha1.ParamSpec{{
					Name:        "task",
					Type:        v1alpha1.ParamTypeString,
					Description: "param",
					Default:     builder.ArrayOrString("default", "array"),
				}},
			},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `"string" type does not match default value's type: "array"`,
			Paths:   []string{"taskspec.inputs.params.task.type", "taskspec.inputs.params.task.default.type"},
		},
	}, {
		name: "one invalid output",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{
					validResource,
				},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{
					invalidResource,
					validResource,
				},
			},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value: what`,
			Paths:   []string{"taskspec.Outputs.Resources.source.Type"},
		},
	}, {
		name: "duplicated inputs",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{
					validResource,
					validResource,
				},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{
					validResource,
				},
			},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: "expected exactly one, got both",
			Paths:   []string{"taskspec.Inputs.Resources.Name"},
		},
	}, {
		name: "duplicated outputs",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{
					validResource,
				},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{
					validResource,
					validResource,
				},
			},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: "expected exactly one, got both",
			Paths:   []string{"taskspec.Outputs.Resources.Name"},
		},
	}, {
		name: "invalid build",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: []v1alpha1.Step{},
		},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"steps"},
		},
	}, {
		name: "invalid build step name",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: invalidSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value "replaceImage"`,
			Paths:   []string{"taskspec.steps.name"},
			Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}, {
		name: "inexistent input param variable",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "mystep",
				Image: "myimage",
				Args:  []string{"--flag=$(inputs.params.inexistent)"},
			}}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "--flag=$(inputs.params.inexistent)" for step arg[0]`,
			Paths:   []string{"taskspec.steps.arg[0]"},
		},
	}, {
		name: "array used in unaccepted field",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validImageResource},
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
					Type: v1alpha1.ParamTypeArray,
				}, {
					Name: "foo-is-baz",
					Type: v1alpha1.ParamTypeArray,
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "mystep",
				Image:      "$(inputs.params.baz)",
				Command:    []string{"$(inputs.param.foo-is-baz)"},
				Args:       []string{"$(inputs.params.baz)", "middle string", "$(input.resources.foo.url)"},
				WorkingDir: "/foo/bar/$(outputs.resources.source)",
			}}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(inputs.params.baz)" for step image`,
			Paths:   []string{"taskspec.steps.image"},
		},
	}, {
		name: "array not properly isolated",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validImageResource},
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
					Type: v1alpha1.ParamTypeArray,
				}, {
					Name: "foo-is-baz",
					Type: v1alpha1.ParamTypeArray,
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "mystep",
				Image:      "someimage",
				Command:    []string{"$(inputs.param.foo-is-baz)"},
				Args:       []string{"not isolated: $(inputs.params.baz)", "middle string", "$(input.resources.foo.url)"},
				WorkingDir: "/foo/bar/$(outputs.resources.source)",
			}}},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(inputs.params.baz)" for step arg[0]`,
			Paths:   []string{"taskspec.steps.arg[0]"},
		},
	}, {
		name: "array not properly isolated",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validImageResource},
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
					Type: v1alpha1.ParamTypeArray,
				}, {
					Name: "foo-is-baz",
					Type: v1alpha1.ParamTypeArray,
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "mystep",
				Image:      "someimage",
				Command:    []string{"$(inputs.param.foo-is-baz)"},
				Args:       []string{"not isolated: $(inputs.params.baz)", "middle string", "$(input.resources.foo.url)"},
				WorkingDir: "/foo/bar/$(outputs.resources.source)",
			}}},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(inputs.params.baz)" for step arg[0]`,
			Paths:   []string{"taskspec.steps.arg[0]"},
		},
	}, {
		name: "inexistent input resource variable",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "mystep",
				Image: "myimage:$(inputs.resources.inputs)",
			}}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "myimage:$(inputs.resources.inputs)" for step image`,
			Paths:   []string{"taskspec.steps.image"},
		},
	}, {
		name: "inferred array not properly isolated",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validImageResource},
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
					Default: &v1alpha1.ArrayOrString{
						Type:     v1alpha1.ParamTypeArray,
						ArrayVal: []string{"implied", "array", "type"},
					},
				}, {
					Name: "foo-is-baz",
					Default: &v1alpha1.ArrayOrString{
						Type:     v1alpha1.ParamTypeArray,
						ArrayVal: []string{"implied", "array", "type"},
					},
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "mystep",
				Image:      "someimage",
				Command:    []string{"$(inputs.param.foo-is-baz)"},
				Args:       []string{"not isolated: $(inputs.params.baz)", "middle string", "$(input.resources.foo.url)"},
				WorkingDir: "/foo/bar/$(outputs.resources.source)",
			}}},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(inputs.params.baz)" for step arg[0]`,
			Paths:   []string{"taskspec.steps.arg[0]"},
		},
	}, {
		name: "inexistent input resource variable",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "mystep",
				Image: "myimage:$(inputs.resources.inputs)",
			}}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "myimage:$(inputs.resources.inputs)" for step image`,
			Paths:   []string{"taskspec.steps.image"},
		},
	}, {
		name: "inexistent output param variable",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "mystep",
				Image:      "myimage",
				WorkingDir: "/foo/bar/$(outputs.resources.inexistent)",
			}}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "/foo/bar/$(outputs.resources.inexistent)" for step workingDir`,
			Paths:   []string{"taskspec.steps.workingDir"},
		},
	}, {
		name: "Inexistent param variable with existing",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Params: []v1alpha1.ParamSpec{
					{
						Name:        "foo",
						Description: "param",
						Default:     builder.ArrayOrString("default"),
					},
				},
			},
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "mystep",
				Image: "myimage",
				Args:  []string{"$(inputs.params.foo) && $(inputs.params.inexistent)"},
			}}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(inputs.params.foo) && $(inputs.params.inexistent)" for step arg[0]`,
			Paths:   []string{"taskspec.steps.arg[0]"},
		},
	}, {
		name: "Multiple volumes with same name",
		fields: fields{
			Steps: validSteps,
			Volumes: []corev1.Volume{{
				Name: "workspace",
			}, {
				Name: "workspace",
			}},
		},
		expectedError: apis.FieldError{
			Message: `multiple volumes with same name "workspace"`,
			Paths:   []string{"volumes.name"},
		},
	}, {
		name: "step with script and command",
		fields: fields{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Image:   "myimage",
					Command: []string{"command"},
				},
				Script: "script",
			}},
		},
		expectedError: apis.FieldError{
			Message: "step 0 script cannot be used with command",
			Paths:   []string{"steps.script"},
		},
	}, {
		name: "step volume mounts under /tekton/",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Image: "myimage",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "foo",
					MountPath: "/tekton/foo",
				}},
			}}},
		},
		expectedError: apis.FieldError{
			Message: `step 0 volumeMount cannot be mounted under /tekton/ (volumeMount "foo" mounted at "/tekton/foo")`,
			Paths:   []string{"steps.volumeMounts.mountPath"},
		},
	}, {
		name: "step volume mount name starts with tekton-internal-",
		fields: fields{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Image: "myimage",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "tekton-internal-foo",
					MountPath: "/this/is/fine",
				}},
			}}},
		},
		expectedError: apis.FieldError{
			Message: `step 0 volumeMount name "tekton-internal-foo" cannot start with "tekton-internal-"`,
			Paths:   []string{"steps.volumeMounts.name"},
		},
	}, {
		name: "declared workspaces names are not unique",
		fields: fields{
			Steps: validSteps,
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "same-workspace",
			}, {
				Name: "same-workspace",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace name \"same-workspace\" must be unique",
			Paths:   []string{"workspaces.name"},
		},
	}, {
		name: "declared workspaces clash with each other",
		fields: fields{
			Steps: validSteps,
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:      "some-workspace",
				MountPath: "/foo",
			}, {
				Name:      "another-workspace",
				MountPath: "/foo",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/foo\" must be unique",
			Paths:   []string{"workspaces.mountpath"},
		},
	}, {
		name: "workspace mount path already in volumeMounts",
		fields: fields{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Image:   "myimage",
					Command: []string{"command"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "my-mount",
						MountPath: "/foo",
					}},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:      "some-workspace",
				MountPath: "/foo",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/foo\" must be unique",
			Paths:   []string{"workspaces.mountpath"},
		},
	}, {
		name: "workspace default mount path already in volumeMounts",
		fields: fields{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Image:   "myimage",
					Command: []string{"command"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "my-mount",
						MountPath: "/workspace/some-workspace/",
					}},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "some-workspace",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/workspace/some-workspace\" must be unique",
			Paths:   []string{"workspaces.mountpath"},
		},
	}, {
		name: "workspace mount path already in stepTemplate",
		fields: fields{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "my-mount",
					MountPath: "/foo",
				}},
			},
			Steps: validSteps,
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:      "some-workspace",
				MountPath: "/foo",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/foo\" must be unique",
			Paths:   []string{"workspaces.mountpath"},
		},
	}, {
		name: "workspace default mount path already in stepTemplate",
		fields: fields{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "my-mount",
					MountPath: "/workspace/some-workspace",
				}},
			},
			Steps: validSteps,
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "some-workspace",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/workspace/some-workspace\" must be unique",
			Paths:   []string{"workspaces.mountpath"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1alpha1.TaskSpec{
				Inputs:       tt.fields.Inputs,
				Outputs:      tt.fields.Outputs,
				Steps:        tt.fields.Steps,
				Volumes:      tt.fields.Volumes,
				StepTemplate: tt.fields.StepTemplate,
				Workspaces:   tt.fields.Workspaces,
			}
			ctx := context.Background()
			ts.SetDefaults(ctx)
			err := ts.Validate(context.Background())
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", ts)
			}
			if d := cmp.Diff(tt.expectedError, *err, cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskSpec.Validate() errors diff -want, +got: %v", d)
			}
		})
	}
}
