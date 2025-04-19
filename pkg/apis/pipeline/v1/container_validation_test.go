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

package v1_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func enableConciseResolverSyntax(ctx context.Context) context.Context {
	return config.ToContext(ctx, &config.Config{
		FeatureFlags: &config.FeatureFlags{
			EnableConciseResolverSyntax: true,
			EnableAPIFields:             config.BetaAPIFields,
		},
	})
}

func TestRef_Valid(t *testing.T) {
	tests := []struct {
		name string
		ref  *v1.Ref
		wc   func(context.Context) context.Context
	}{{
		name: "nil ref",
	}, {
		name: "simple ref",
		ref:  &v1.Ref{Name: "refname"},
	}, {
		name: "ref name - concise syntax",
		ref:  &v1.Ref{Name: "foo://baz:ver", ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wc:   enableConciseResolverSyntax,
	}, {
		name: "beta feature: valid resolver",
		ref:  &v1.Ref{ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wc:   cfgtesting.EnableBetaAPIFields,
	}, {
		name: "beta feature: valid resolver with alpha flag",
		ref:  &v1.Ref{ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wc:   cfgtesting.EnableAlphaAPIFields,
	}, {
		name: "beta feature: valid resolver with params",
		ref: &v1.Ref{ResolverRef: v1.ResolverRef{Resolver: "git", Params: v1.Params{{
			Name: "repo",
			Value: v1.ParamValue{
				Type:      v1.ParamTypeString,
				StringVal: "https://github.com/tektoncd/pipeline.git",
			},
		}, {
			Name: "branch",
			Value: v1.ParamValue{
				Type:      v1.ParamTypeString,
				StringVal: "baz",
			},
		}}}},
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := t.Context()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			if err := ts.ref.Validate(ctx); err != nil {
				t.Errorf("Ref.Validate() error = %v", err)
			}
		})
	}
}

func TestRef_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		ref     *v1.Ref
		wantErr *apis.FieldError
		wc      func(context.Context) context.Context
	}{{
		name:    "missing ref name",
		ref:     &v1.Ref{},
		wantErr: apis.ErrMissingField("name"),
	}, {
		name: "ref params disallowed without resolver",
		ref: &v1.Ref{
			ResolverRef: v1.ResolverRef{
				Params: v1.Params{},
			},
		},
		wantErr: apis.ErrMissingField("resolver"),
	}, {
		name: "ref with resolver and k8s style name",
		ref: &v1.Ref{
			Name: "foo",
			ResolverRef: v1.ResolverRef{
				Resolver: "git",
			},
		},
		wantErr: apis.ErrInvalidValue(`invalid URI for request`, "name"),
		wc:      enableConciseResolverSyntax,
	}, {
		name: "ref with url-like name without resolver",
		ref: &v1.Ref{
			Name: "https://foo.com/bar",
		},
		wantErr: apis.ErrMissingField("resolver"),
		wc:      enableConciseResolverSyntax,
	}, {
		name: "ref params disallowed in conjunction with pipelineref name",
		ref: &v1.Ref{
			Name: "https://foo/bar",
			ResolverRef: v1.ResolverRef{
				Resolver: "git",
				Params:   v1.Params{{Name: "foo", Value: v1.ParamValue{StringVal: "bar"}}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("name", "params"),
		wc:      enableConciseResolverSyntax,
	}, {
		name: "ref with url-like name without enable-concise-resolver-syntax",
		ref:  &v1.Ref{Name: "https://foo.com/bar"},
		wantErr: apis.ErrMissingField("resolver").Also(&apis.FieldError{
			Message: `feature flag enable-concise-resolver-syntax should be set to true to use concise resolver syntax`,
		}),
	}, {
		name: "ref without enable-concise-resolver-syntax",
		ref:  &v1.Ref{Name: "https://foo.com/bar", ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wantErr: &apis.FieldError{
			Message: `feature flag enable-concise-resolver-syntax should be set to true to use concise resolver syntax`,
		},
	}, {
		name: "invalid ref name",
		ref:  &v1.Ref{Name: "_foo"},
		wantErr: &apis.FieldError{
			Message: `invalid value: name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
			Paths:   []string{"name"},
		},
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := t.Context()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			err := ts.ref.Validate(ctx)
			if d := cmp.Diff(ts.wantErr.Error(), err.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestStepValidate(t *testing.T) {
	tests := []struct {
		name string
		Step v1.Step
	}{{
		name: "unnamed step",
		Step: v1.Step{
			Image: "myimage",
		},
	}, {
		name: "valid step with script",
		Step: v1.Step{
			Image: "my-image",
			Script: `
				#!/usr/bin/env bash
				hello world`,
		},
	}, {
		name: "valid step with script and args",
		Step: v1.Step{
			Image: "my-image",
			Args:  []string{"arg"},
			Script: `
				#!/usr/bin/env  bash
				hello $1`,
		},
	}, {
		name: "valid step with volumeMount under /tekton/home",
		Step: v1.Step{
			Image: "myimage",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "foo",
				MountPath: "/tekton/home",
			}},
		},
	}}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := cfgtesting.EnableAlphaAPIFields(t.Context())
			if err := st.Step.Validate(ctx); err != nil {
				t.Errorf("Step.Validate() = %v", err)
			}
		})
	}
}

func TestStepValidateError(t *testing.T) {
	tests := []struct {
		name          string
		Step          v1.Step
		expectedError apis.FieldError
	}{{
		name: "invalid step with missing Image",
		Step: v1.Step{},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"Image"},
		},
	}, {
		name: "invalid step name",
		Step: v1.Step{
			Name:  "replaceImage",
			Image: "myimage",
		},
		expectedError: apis.FieldError{
			Message: `invalid value "replaceImage"`,
			Paths:   []string{"name"},
			Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}, {
		name: "step with script and command",
		Step: v1.Step{
			Image:   "myimage",
			Command: []string{"command"},
			Script:  "script",
		},
		expectedError: apis.FieldError{
			Message: "script cannot be used with command",
			Paths:   []string{"script"},
		},
	}, {
		name: "step volume mounts under /tekton/",
		Step: v1.Step{
			Image: "myimage",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "foo",
				MountPath: "/tekton/foo",
			}},
		},
		expectedError: apis.FieldError{
			Message: `volumeMount cannot be mounted under /tekton/ (volumeMount "foo" mounted at "/tekton/foo")`,
			Paths:   []string{"volumeMounts[0].mountPath"},
		},
	}, {
		name: "step volume mount name starts with tekton-internal-",
		Step: v1.Step{
			Image: "myimage",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "tekton-internal-foo",
				MountPath: "/this/is/fine",
			}},
		},
		expectedError: apis.FieldError{
			Message: `volumeMount name "tekton-internal-foo" cannot start with "tekton-internal-"`,
			Paths:   []string{"volumeMounts[0].name"},
		},
	}, {
		name: "negative timeout string",
		Step: v1.Step{
			Timeout: &metav1.Duration{Duration: -10 * time.Second},
		},
		expectedError: apis.FieldError{
			Message: "invalid value: -10s",
			Paths:   []string{"negative timeout"},
		},
	}}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := cfgtesting.EnableAlphaAPIFields(t.Context())
			err := st.Step.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", st.Step)
			}
			if d := cmp.Diff(st.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Step.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestStepValidateSuccessWithArtifactsRefFlagEnabled(t *testing.T) {
	tests := []struct {
		name string
		Step v1.Step
	}{
		{
			name: "reference step artifacts in Env",
			Step: v1.Step{
				Image: "busybox",
				Env:   []corev1.EnvVar{{Name: "AAA", Value: "$(steps.aaa.outputs.image)"}},
			},
		},
		{
			name: "reference step artifacts path in Env",
			Step: v1.Step{
				Image: "busybox",
				Env:   []corev1.EnvVar{{Name: "AAA", Value: "$(step.artifacts.path)"}},
			},
		},
		{
			name: "reference step artifacts in Script",
			Step: v1.Step{
				Image:  "busybox",
				Script: "echo $(steps.aaa.inputs.bbb)",
			},
		},
		{
			name: "reference step artifacts path in Script",
			Step: v1.Step{
				Image:  "busybox",
				Script: "echo 123 >> $(step.artifacts.path)",
			},
		},
		{
			name: "reference step artifacts in Command",
			Step: v1.Step{
				Image:   "busybox",
				Command: []string{"echo", "$(steps.aaa.outputs.bbbb)"},
			},
		},
		{
			name: "reference step artifacts path in Command",
			Step: v1.Step{
				Image:   "busybox",
				Command: []string{"echo", "$(step.artifacts.path)"},
			},
		},
		{
			name: "reference step artifacts in Args",
			Step: v1.Step{
				Image: "busybox",
				Args:  []string{"echo", "$(steps.aaa.outputs.bbbb)"},
			},
		},
		{
			name: "reference step artifacts path in Args",
			Step: v1.Step{
				Image: "busybox",
				Args:  []string{"echo", "$(step.artifacts.path)"},
			},
		},
	}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := config.ToContext(t.Context(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableStepActions: true,
					EnableArtifacts:   true,
				},
			})
			ctx = apis.WithinCreate(ctx)
			err := st.Step.Validate(ctx)
			if err != nil {
				t.Fatalf("Expected no errors, got err for %v", err)
			}
		})
	}
}

func TestStepValidateSuccessWithArtifactsRefFlagNotEnabled(t *testing.T) {
	tests := []struct {
		name string
		Step v1.Step
	}{
		{
			name: "script without reference to a step artifact",
			Step: v1.Step{
				Image:  "busybox",
				Script: "echo 123",
			},
		},
	}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := config.ToContext(t.Context(), &config.Config{
				FeatureFlags: nil,
			})
			ctx = apis.WithinCreate(ctx)
			err := st.Step.Validate(ctx)
			if err != nil {
				t.Fatalf("Expected no errors, got err for %v", err)
			}
		})
	}
}

func TestStepValidateErrorWithArtifactsRefFlagNotEnabled(t *testing.T) {
	tests := []struct {
		name          string
		Step          v1.Step
		expectedError apis.FieldError
	}{
		{
			name: "Cannot reference step artifacts in Env without setting enable-artifacts to true",
			Step: v1.Step{
				Env: []corev1.EnvVar{{Name: "AAA", Value: "$(steps.aaa.outputs.image)"}},
			},
			expectedError: apis.FieldError{
				Message: fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts),
			},
		},
		{
			name: "Cannot reference step artifacts path in Env without setting enable-artifacts to true",
			Step: v1.Step{
				Env: []corev1.EnvVar{{Name: "AAA", Value: "$(step.artifacts.path)"}},
			},
			expectedError: apis.FieldError{
				Message: fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts),
			},
		},
		{
			name: "Cannot reference step artifacts in Script without setting enable-artifacts to true",
			Step: v1.Step{
				Script: "echo $(steps.aaa.inputs.bbb)",
			},
			expectedError: apis.FieldError{
				Message: fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts),
			},
		},
		{
			name: "Cannot reference step artifacts path in Script without setting enable-artifacts to true",
			Step: v1.Step{
				Script: "echo 123 >> $(step.artifacts.path)",
			},
			expectedError: apis.FieldError{
				Message: fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts),
			},
		},
		{
			name: "Cannot reference step artifacts in Command without setting enable-artifacts to true",
			Step: v1.Step{
				Command: []string{"echo", "$(steps.aaa.outputs.bbbb)"},
			},
			expectedError: apis.FieldError{
				Message: fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts),
			},
		},
		{
			name: "Cannot reference step artifacts path in Command without setting enable-artifacts to true",
			Step: v1.Step{
				Command: []string{"echo", "$(step.artifacts.path)"},
			},
			expectedError: apis.FieldError{
				Message: fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts),
			},
		},
		{
			name: "Cannot reference step artifacts in Args without setting enable-artifacts to true",
			Step: v1.Step{
				Args: []string{"echo", "$(steps.aaa.outputs.bbbb)"},
			},
			expectedError: apis.FieldError{
				Message: fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts),
			},
		},
		{
			name: "Cannot reference step artifacts path in Args without setting enable-artifacts to true",
			Step: v1.Step{
				Args: []string{"echo", "$(step.artifacts.path)"},
			},
			expectedError: apis.FieldError{
				Message: fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts),
			},
		},
	}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := config.ToContext(t.Context(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableStepActions: true,
				},
			})
			ctx = apis.WithinCreate(ctx)
			err := st.Step.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", st.Step)
			}
			if d := cmp.Diff(st.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Step.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestSidecarValidate(t *testing.T) {
	tests := []struct {
		name    string
		sidecar v1.Sidecar
	}{{
		name: "valid sidecar",
		sidecar: v1.Sidecar{
			Name:  "my-sidecar",
			Image: "my-image",
		},
	}}

	for _, sct := range tests {
		t.Run(sct.name, func(t *testing.T) {
			err := sct.sidecar.Validate(t.Context())
			if err != nil {
				t.Errorf("Sidecar.Validate() returned error for valid Sidecar: %v", err)
			}
		})
	}
}

func TestSidecarValidateError(t *testing.T) {
	tests := []struct {
		name          string
		sidecar       v1.Sidecar
		expectedError apis.FieldError
	}{{
		name: "cannot use reserved sidecar name",
		sidecar: v1.Sidecar{
			Name:  "tekton-log-results",
			Image: "my-image",
		},
		expectedError: apis.FieldError{
			Message: fmt.Sprintf("Invalid: cannot use reserved sidecar name %v ", pipeline.ReservedResultsSidecarName),
			Paths:   []string{"name"},
		},
	}, {
		name: "missing image",
		sidecar: v1.Sidecar{
			Name: "tekton-invalid-side-car",
		},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"image"},
		},
	}, {
		name: "invalid command with script",
		sidecar: v1.Sidecar{
			Name:    "tekton-invalid-side-car",
			Image:   "ubuntu",
			Command: []string{"command foo"},
			Script: `
				#!/usr/bin/env  bash
				echo foo`,
		},
		expectedError: apis.FieldError{
			Message: "script cannot be used with command",
			Paths:   []string{"script"},
		},
	}}

	for _, sct := range tests {
		t.Run(sct.name, func(t *testing.T) {
			err := sct.sidecar.Validate(t.Context())
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", sct.sidecar)
			}
			if d := cmp.Diff(sct.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Sidecar.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
