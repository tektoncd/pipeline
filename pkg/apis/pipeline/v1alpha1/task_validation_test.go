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
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var validResource = v1alpha1.TaskResource{
	Name: "source",
	Type: "git",
}

var validImageResource = v1alpha1.TaskResource{
	Name: "source",
	Type: "image",
}

var validBuildSteps = []corev1.Container{{
	Name:  "mystep",
	Image: "myimage",
}}

var invalidBuildSteps = []corev1.Container{{
	Name:  "replaceImage",
	Image: "myimage",
}}

func TestTaskSpecValidate(t *testing.T) {
	type fields struct {
		Inputs            *v1alpha1.Inputs
		Outputs           *v1alpha1.Outputs
		BuildSteps        []corev1.Container
		StepTemplate      *corev1.Container
		ContainerTemplate *corev1.Container
	}
	tests := []struct {
		name   string
		fields fields
	}{{
		name: "valid inputs",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{validResource},
				Params: []v1alpha1.ParamSpec{
					{
						Name:        "task",
						Description: "param",
						Default:     "default",
					},
				},
			},
			BuildSteps: validBuildSteps,
		},
	}, {
		name: "valid outputs",
		fields: fields{
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			BuildSteps: validBuildSteps,
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
			BuildSteps: validBuildSteps,
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
			BuildSteps: validBuildSteps,
		},
	}, {
		name: "valid template variable",
		fields: fields{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "foo",
					Type: v1alpha1.PipelineResourceTypeImage,
				}},
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
				}, {
					Name: "foo-is-baz",
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{validResource},
			},
			BuildSteps: []corev1.Container{{
				Name:       "mystep",
				Image:      "${inputs.resources.foo.url}",
				Args:       []string{"--flag=${inputs.params.baz} && ${input.params.foo-is-baz}"},
				WorkingDir: "/foo/bar/${outputs.resources.source}",
			}},
		},
	}, {
		name: "step template included in validation",
		fields: fields{
			BuildSteps: []corev1.Container{{
				Name:    "astep",
				Command: []string{"echo"},
				Args:    []string{"hello"},
			}},
			StepTemplate: &corev1.Container{
				Image: "some-image",
			},
		},
	}, {
		name: "deprecated (#977) container template included in validation",
		fields: fields{
			BuildSteps: []corev1.Container{{
				Name:    "astep",
				Command: []string{"echo"},
				Args:    []string{"hello"},
			}},
			ContainerTemplate: &corev1.Container{
				Image: "some-image",
			},
		},
	}, {
		name: "deprecated (#977) container template supported with step template",
		fields: fields{
			BuildSteps: []corev1.Container{{
				Name:    "astep",
				Command: []string{"echo"},
				Args:    []string{"hello"},
			}},
			StepTemplate: &corev1.Container{
				Env: []corev1.EnvVar{{
					Name:  "somevar",
					Value: "someval",
				}},
			},
			ContainerTemplate: &corev1.Container{
				Image: "some-image",
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1alpha1.TaskSpec{
				Inputs:            tt.fields.Inputs,
				Outputs:           tt.fields.Outputs,
				Steps:             tt.fields.BuildSteps,
				StepTemplate:      tt.fields.StepTemplate,
				ContainerTemplate: tt.fields.ContainerTemplate,
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
		Inputs     *v1alpha1.Inputs
		Outputs    *v1alpha1.Outputs
		BuildSteps []corev1.Container
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
					{
						Name: "source",
						Type: "what",
					},
					validResource,
				},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{
					validResource,
				},
			},
			BuildSteps: validBuildSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value: what`,
			Paths:   []string{"taskspec.Inputs.Resources.source.Type"},
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
					{
						Name: "who",
						Type: "what",
					},
					validResource,
				},
			},
			BuildSteps: validBuildSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value: what`,
			Paths:   []string{"taskspec.Outputs.Resources.who.Type"},
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
			BuildSteps: validBuildSteps,
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
			BuildSteps: validBuildSteps,
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
			BuildSteps: []corev1.Container{},
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
			BuildSteps: invalidBuildSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value "replaceImage"`,
			Paths:   []string{"taskspec.steps.name"},
			Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}, {
		name: "inexistent input param variable",
		fields: fields{
			BuildSteps: []corev1.Container{{
				Name:  "mystep",
				Image: "myimage",
				Args:  []string{"--flag=${inputs.params.inexistent}"},
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "--flag=${inputs.params.inexistent}" for step arg[0]`,
			Paths:   []string{"taskspec.steps.arg[0]"},
		},
	}, {
		name: "inexistent input resource variable",
		fields: fields{
			BuildSteps: []corev1.Container{{
				Name:  "mystep",
				Image: "myimage:${inputs.resources.inputs}",
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "myimage:${inputs.resources.inputs}" for step image`,
			Paths:   []string{"taskspec.steps.image"},
		},
	}, {
		name: "inexistent output param variable",
		fields: fields{
			BuildSteps: []corev1.Container{{
				Name:       "mystep",
				Image:      "myimage",
				WorkingDir: "/foo/bar/${outputs.resources.inexistent}",
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "/foo/bar/${outputs.resources.inexistent}" for step workingDir`,
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
						Default:     "default",
					},
				},
			},
			BuildSteps: []corev1.Container{{
				Name:  "mystep",
				Image: "myimage",
				Args:  []string{"${inputs.params.foo} && ${inputs.params.inexistent}"},
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "${inputs.params.foo} && ${inputs.params.inexistent}" for step arg[0]`,
			Paths:   []string{"taskspec.steps.arg[0]"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1alpha1.TaskSpec{
				Inputs:  tt.fields.Inputs,
				Outputs: tt.fields.Outputs,
				Steps:   tt.fields.BuildSteps,
			}
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
