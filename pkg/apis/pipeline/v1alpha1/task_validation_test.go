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
	"testing"

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

func TestTaskSpec_Validate(t *testing.T) {
	type fields struct {
		Inputs  *Inputs
		Outputs *Outputs
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
		},
	}, {
		name: "valid outputs",
		fields: fields{
			Outputs: &Outputs{
				Resources: []TaskResource{validResource},
			},
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
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TaskSpec{
				Inputs:  tt.fields.Inputs,
				Outputs: tt.fields.Outputs,
				Steps:   validBuildSteps,
			}
			if err := ts.Validate(); err != nil {
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
		name   string
		fields fields
	}{{
		name: "nil",
	}, {
		name: "no build",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{validResource},
			},
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
	}, {
		name: "invalid build",
		fields: fields{
			Inputs: &Inputs{
				Resources: []TaskResource{validResource},
			},
			BuildSteps: []corev1.Container{},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TaskSpec{
				Inputs:  tt.fields.Inputs,
				Outputs: tt.fields.Outputs,
				Steps:   tt.fields.BuildSteps,
			}
			if err := ts.Validate(); err == nil {
				t.Errorf("TaskSpec.Validate() did not return error.")
			}
		})
	}
}
