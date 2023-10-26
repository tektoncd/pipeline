/*
Copyright 2023 The Tekton Authors
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
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestStepActionValidate(t *testing.T) {
	tests := []struct {
		name string
		sa   *v1alpha1.StepAction
		wc   func(context.Context) context.Context
	}{{
		name: "valid step action",
		sa: &v1alpha1.StepAction{
			ObjectMeta: metav1.ObjectMeta{Name: "stepaction"},
			Spec: v1alpha1.StepActionSpec{
				Image: "my-image",
				Script: `
				#!/usr/bin/env  bash
				echo hello`,
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.sa.Validate(ctx)
			if err != nil {
				t.Errorf("StepAction.Validate() returned error for valid StepAction: %v", err)
			}
		})
	}
}

func TestStepActionSpecValidate(t *testing.T) {
	type fields struct {
		Image   string
		Command []string
		Args    []string
		Script  string
		Env     []corev1.EnvVar
	}
	tests := []struct {
		name   string
		fields fields
	}{{
		name: "step action with command",
		fields: fields{
			Image:   "myimage",
			Command: []string{"ls"},
			Args:    []string{"-lh"},
		},
	}, {
		name: "step action with script",
		fields: fields{
			Image:  "myimage",
			Script: "echo hi",
		},
	}, {
		name: "step action with env",
		fields: fields{
			Image:  "myimage",
			Script: "echo hi",
			Env: []corev1.EnvVar{{
				Name:  "HOME",
				Value: "/tekton/home",
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &v1alpha1.StepActionSpec{
				Image:   tt.fields.Image,
				Command: tt.fields.Command,
				Args:    tt.fields.Args,
				Script:  tt.fields.Script,
				Env:     tt.fields.Env,
			}
			if err := sa.Validate(context.Background()); err != nil {
				t.Errorf("StepActionSpec.Validate() = %v", err)
			}
		})
	}
}

func TestStepActionValidateError(t *testing.T) {
	type fields struct {
		Image   string
		Command []string
		Args    []string
		Script  string
		Env     []corev1.EnvVar
	}
	tests := []struct {
		name          string
		fields        fields
		expectedError apis.FieldError
	}{{
		name: "inexistent image field",
		fields: fields{
			Args: []string{"--flag=$(params.inexistent)"},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"spec.Image"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &v1alpha1.StepAction{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: v1alpha1.StepActionSpec{
					Image:   tt.fields.Image,
					Command: tt.fields.Command,
					Args:    tt.fields.Args,
					Script:  tt.fields.Script,
					Env:     tt.fields.Env,
				},
			}
			ctx := context.Background()
			err := sa.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", sa)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("StepActionSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestStepActionSpecValidateError(t *testing.T) {
	type fields struct {
		Image   string
		Command []string
		Args    []string
		Script  string
		Env     []corev1.EnvVar
	}
	tests := []struct {
		name          string
		fields        fields
		expectedError apis.FieldError
	}{{
		name: "inexistent image field",
		fields: fields{
			Args: []string{"--flag=$(params.inexistent)"},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"Image"},
		},
	}, {
		name: "command and script both used.",
		fields: fields{
			Image:   "my-image",
			Command: []string{"ls"},
			Script:  "echo hi",
		},
		expectedError: apis.FieldError{
			Message: `script cannot be used with command`,
			Paths:   []string{"script"},
		},
	}, {
		name: "windows script without alpha.",
		fields: fields{
			Image:  "my-image",
			Script: "#!win",
		},
		expectedError: apis.FieldError{
			Message: `windows script support requires "enable-api-fields" feature gate to be "alpha" but it is "beta"`,
			Paths:   []string{},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := v1alpha1.StepActionSpec{
				Image:   tt.fields.Image,
				Command: tt.fields.Command,
				Args:    tt.fields.Args,
				Script:  tt.fields.Script,
				Env:     tt.fields.Env,
			}
			ctx := context.Background()
			err := sa.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", sa)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("StepActionSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
