/*
Copyright 2018 The Knative Authors

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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
)

var validResource = TaskResource{
	Name: "source",
	Type: "git",
}

var validBuildSteps = []corev1.Container{{
	Name:  "mystep",
	Image: "myimage",
}}

var invalidBuildSteps = []corev1.Container{{
	Name:  "replaceImage",
	Image: "myimage",
}}

func TestTaskSpec_Validate(t *testing.T) {
	type fields struct {
		Inputs     *Inputs
		Outputs    *Outputs
		BuildSteps []corev1.Container
	}
	tests := []struct {
		name   string
		fields fields
	}{{
		name: "valid inputs",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{validResource},
				Params: []TaskParam{
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
			Outputs: &Outputs{
				Resources: []TaskResource{validResource},
			},
			BuildSteps: validBuildSteps,
		},
	}, {
		name: "both valid",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{validResource},
			},
			Outputs: &Outputs{
				Resources: []TaskResource{validResource},
			},
			BuildSteps: validBuildSteps,
		},
	}, {
		name: "valid template variable",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{{
					Name: "foo",
					Type: PipelineResourceTypeImage,
				}},
				Params: []TaskParam{{
					Name: "baz",
				}, {
					Name: "foo-is-baz",
				}},
			},
			Outputs: &Outputs{
				Resources: []TaskResource{validResource},
			},
			BuildSteps: []corev1.Container{{
				Name:       "mystep",
				Image:      "${inputs.resources.foo.url}",
				Args:       []string{"--flag=${inputs.params.baz} && ${input.params.foo-is-baz}"},
				WorkingDir: "/foo/bar/${outputs.resources.source.name}",
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TaskSpec{
				Inputs:  tt.fields.Inputs,
				Outputs: tt.fields.Outputs,
				Steps:   tt.fields.BuildSteps,
			}
			if err := ts.Validate(context.Background()); err != nil {
				t.Errorf("TaskSpec.Validate() = %v", err)
			}
		})
	}
}

func TestTaskSpec_ValidateError(t *testing.T) {
	type fields struct {
		Inputs     *Inputs
		Outputs    *Outputs
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
			Inputs: &Inputs{
				Resources: []TaskResource{validResource},
			},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"steps"},
		},
	}, {
		name: "one invalid input",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{
					{
						Name: "source",
						Type: "what",
					},
					validResource,
				},
			},
			Outputs: &Outputs{
				Resources: []TaskResource{
					validResource,
				},
			},
			BuildSteps: validBuildSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value "what"`,
			Paths:   []string{"taskspec.Inputs.Resources.source.Type"},
		},
	}, {
		name: "one invalid output",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{
					validResource,
				},
			},
			Outputs: &Outputs{
				Resources: []TaskResource{
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
			Message: `invalid value "what"`,
			Paths:   []string{"taskspec.Outputs.Resources.who.Type"},
		},
	}, {
		name: "duplicated inputs",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{
					validResource,
					validResource,
				},
			},
			Outputs: &Outputs{
				Resources: []TaskResource{
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
			Inputs: &Inputs{
				Resources: []TaskResource{
					validResource,
				},
			},
			Outputs: &Outputs{
				Resources: []TaskResource{
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
			Inputs: &Inputs{
				Resources: []TaskResource{validResource},
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
			Inputs: &Inputs{
				Resources: []TaskResource{validResource},
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
			Inputs: &Inputs{
				Params: []TaskParam{
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
	}, {
		name: "invalid resource variable usage",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{{
					Name: "foo",
					Type: PipelineResourceTypeImage,
				}},
			},
			BuildSteps: []corev1.Container{{
				Name:  "mystep",
				Image: "myimage",
				Args:  []string{"${inputs.resources.foo}"},
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "${inputs.resources.foo}" for step arg[0]`,
			Paths:   []string{"taskspec.steps.arg[0]"},
		},
	},
		{
			name: "inexistent output resource variable",
			fields: fields{
				Inputs: &Inputs{
					Resources: []TaskResource{{
						Name: "foo",
						Type: PipelineResourceTypeImage,
					}},
				},
				BuildSteps: []corev1.Container{{
					Name:  "mystep",
					Image: "myimage",
					Args:  []string{"${outputs.resources.foo.url}"},
				}},
			},
			expectedError: apis.FieldError{
				Message: `non-existent variable in "${outputs.resources.foo.url}" for step arg[0]`,
				Paths:   []string{"taskspec.steps.arg[0]"},
			},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TaskSpec{
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

func TestGetResourceVariables(t *testing.T) {
	tests := []struct {
		name         string
		pathPrefix   string
		resources    []TaskResource
		expectedVars map[string]struct{}
	}{
		{
			name:         "empty resources",
			pathPrefix:   "taskspec.outputs.resources.",
			resources:    []TaskResource{},
			expectedVars: map[string]struct{}{},
		},
		{
			name:       "single resource",
			pathPrefix: "taskspec.outputs.resources.",
			resources:  []TaskResource{validResource},
			expectedVars: map[string]struct{}{
				"source.name":     struct{}{},
				"source.type":     struct{}{},
				"source.url":      struct{}{},
				"source.revision": struct{}{},
			},
		},
		{
			name:       "multiple resources",
			pathPrefix: "taskspec.outputs.resources.",
			resources: []TaskResource{
				validResource,
				TaskResource{
					Name: "foo",
					Type: PipelineResourceTypeImage,
				},
			},
			expectedVars: map[string]struct{}{
				"source.name":     struct{}{},
				"source.type":     struct{}{},
				"source.url":      struct{}{},
				"source.revision": struct{}{},
				"foo.name":        struct{}{},
				"foo.type":        struct{}{},
				"foo.url":         struct{}{},
				"foo.digest":      struct{}{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars, err := getResourceVariables(tt.resources, tt.pathPrefix)
			if err != nil {
				t.Errorf("getResourceVariables() = %v", err)
			}
			if d := cmp.Diff(tt.expectedVars, vars); d != "" {
				t.Errorf("getResourceVariables results diff -want, +got: %v", d)
			}
		})
	}
}
