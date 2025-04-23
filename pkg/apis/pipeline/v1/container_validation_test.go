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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
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
			ctx := context.Background()
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
		name: "ref params disallowed in conjunction with pipelineRef name",
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
			ctx := context.Background()
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

func TestStepValidateErrorWithStepResultRef(t *testing.T) {
	tests := []struct {
		name          string
		Step          v1.Step
		expectedError apis.FieldError
	}{{
		name: "Cannot reference step results in image",
		Step: v1.Step{
			Image: "$(steps.prevStep.results.resultName)",
		},
		expectedError: apis.FieldError{
			Message: "stepResult substitutions are only allowed in env, command and args. Found usage in",
			Paths:   []string{"image"},
		},
	}, {
		name: "Cannot reference step results in script",
		Step: v1.Step{
			Image:  "my-img",
			Script: "echo $(steps.prevStep.results.resultName)",
		},
		expectedError: apis.FieldError{
			Message: "stepResult substitutions are only allowed in env, command and args. Found usage in",
			Paths:   []string{"script"},
		},
	}, {
		name: "Cannot reference step results in workingDir",
		Step: v1.Step{
			Image:      "my-img",
			WorkingDir: "$(steps.prevStep.results.resultName)",
		},
		expectedError: apis.FieldError{
			Message: "stepResult substitutions are only allowed in env, command and args. Found usage in",
			Paths:   []string{"workingDir"},
		},
	}, {
		name: "Cannot reference step results in envFrom",
		Step: v1.Step{
			Image: "my-img",
			EnvFrom: []corev1.EnvFromSource{{
				Prefix: "$(steps.prevStep.results.resultName)",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(steps.prevStep.results.resultName)",
					},
				},
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(steps.prevStep.results.resultName)",
					},
				},
			}},
		},
		expectedError: apis.FieldError{
			Message: "stepResult substitutions are only allowed in env, command and args. Found usage in",
			Paths:   []string{"envFrom.configMapRef", "envFrom.prefix", "envFrom.secretRef"},
		},
	}, {
		name: "Cannot reference step results in VolumeMounts",
		Step: v1.Step{
			Image: "my-img",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(steps.prevStep.results.resultName)",
				MountPath: "$(steps.prevStep.results.resultName)",
				SubPath:   "$(steps.prevStep.results.resultName)",
			}},
		},
		expectedError: apis.FieldError{
			Message: "stepResult substitutions are only allowed in env, command and args. Found usage in",
			Paths:   []string{"volumeMounts.name", "volumeMounts.mountPath", "volumeMounts.subPath"},
		},
	}, {
		name: "Cannot reference step results in VolumeDevices",
		Step: v1.Step{
			Image: "my-img",
			VolumeDevices: []corev1.VolumeDevice{{
				Name:       "$(steps.prevStep.results.resultName)",
				DevicePath: "$(steps.prevStep.results.resultName)",
			}},
		},
		expectedError: apis.FieldError{
			Message: "stepResult substitutions are only allowed in env, command and args. Found usage in",
			Paths:   []string{"volumeDevices.name", "volumeDevices.devicePath"},
		},
	}}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := config.ToContext(context.Background(), &config.Config{
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

func TestStepValidateErrorWithArtifactsRef(t *testing.T) {
	tests := []struct {
		name          string
		Step          v1.Step
		expectedError apis.FieldError
	}{{
		name: "Cannot reference step artifacts in image",
		Step: v1.Step{
			Image: "$(steps.prevStep.outputs.aaa)",
		},
		expectedError: apis.FieldError{
			Message: "stepArtifact substitutions are only allowed in env, command, args and script. Found usage in",
			Paths:   []string{"image"},
		},
	}, {
		name: "Cannot reference step artifacts in workingDir",
		Step: v1.Step{
			Image:      "my-img",
			WorkingDir: "$(steps.prevStep.outputs.aaa)",
		},
		expectedError: apis.FieldError{
			Message: "stepArtifact substitutions are only allowed in env, command, args and script. Found usage in",
			Paths:   []string{"workingDir"},
		},
	}, {
		name: "Cannot reference step artifacts in envFrom",
		Step: v1.Step{
			Image: "my-img",
			EnvFrom: []corev1.EnvFromSource{{
				Prefix: "$(steps.prevStep.outputs.aaa)",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(steps.prevStep.outputs.aaa)",
					},
				},
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(steps.prevStep.outputs.aaa)",
					},
				},
			}},
		},
		expectedError: apis.FieldError{
			Message: "stepArtifact substitutions are only allowed in env, command, args and script. Found usage in",
			Paths:   []string{"envFrom.configMapRef", "envFrom.prefix", "envFrom.secretRef"},
		},
	}, {
		name: "Cannot reference step artifacts in VolumeMounts",
		Step: v1.Step{
			Image: "my-img",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(steps.prevStep.outputs.aaa)",
				MountPath: "$(steps.prevStep.outputs.aaa)",
				SubPath:   "$(steps.prevStep.outputs.aaa)",
			}},
		},
		expectedError: apis.FieldError{
			Message: "stepArtifact substitutions are only allowed in env, command, args and script. Found usage in",
			Paths:   []string{"volumeMounts.name", "volumeMounts.mountPath", "volumeMounts.subPath"},
		},
	}, {
		name: "Cannot reference step artifacts in VolumeDevices",
		Step: v1.Step{
			Image: "my-img",
			VolumeDevices: []corev1.VolumeDevice{{
				Name:       "$(steps.prevStep.outputs.aaa)",
				DevicePath: "$(steps.prevStep.outputs.aaa)",
			}},
		},
		expectedError: apis.FieldError{
			Message: "stepArtifact substitutions are only allowed in env, command, args and script. Found usage in",
			Paths:   []string{"volumeDevices.name", "volumeDevices.devicePath"},
		},
	}}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := config.ToContext(context.Background(), &config.Config{
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
			err := sct.sidecar.Validate(context.Background())
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
			err := sct.sidecar.Validate(context.Background())
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", sct.sidecar)
			}
			if d := cmp.Diff(sct.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Sidecar.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
