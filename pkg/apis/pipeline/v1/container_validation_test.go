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

func TestStepValidateErrorWithStepActionRef(t *testing.T) {
	tests := []struct {
		name          string
		Step          v1.Step
		expectedError apis.FieldError
	}{
		{
			name: "Cannot use image with Ref",
			Step: v1.Step{
				Ref: &v1.Ref{
					Name: "stepAction",
				},
				Image: "foo",
			},
			expectedError: apis.FieldError{
				Message: "image cannot be used with Ref",
				Paths:   []string{"image"},
			},
		}, {
			name: "Cannot use command with Ref",
			Step: v1.Step{
				Ref: &v1.Ref{
					Name: "stepAction",
				},
				Command: []string{"foo"},
			},
			expectedError: apis.FieldError{
				Message: "command cannot be used with Ref",
				Paths:   []string{"command"},
			},
		}, {
			name: "Cannot use args with Ref",
			Step: v1.Step{
				Ref: &v1.Ref{
					Name: "stepAction",
				},
				Args: []string{"foo"},
			},
			expectedError: apis.FieldError{
				Message: "args cannot be used with Ref",
				Paths:   []string{"args"},
			},
		}, {
			name: "Cannot use script with Ref",
			Step: v1.Step{
				Ref: &v1.Ref{
					Name: "stepAction",
				},
				Script: "echo hi",
			},
			expectedError: apis.FieldError{
				Message: "script cannot be used with Ref",
				Paths:   []string{"script"},
			},
		}, {
			name: "Cannot use workingDir with Ref",
			Step: v1.Step{
				Ref: &v1.Ref{
					Name: "stepAction",
				},
				WorkingDir: "/workspace",
			},
			expectedError: apis.FieldError{
				Message: "working dir cannot be used with Ref",
				Paths:   []string{"workingDir"},
			},
		}, {
			name: "Cannot use env with Ref",
			Step: v1.Step{
				Ref: &v1.Ref{
					Name: "stepAction",
				},
				Env: []corev1.EnvVar{{
					Name:  "env1",
					Value: "value1",
				}},
			},
			expectedError: apis.FieldError{
				Message: "env cannot be used with Ref",
				Paths:   []string{"env"},
			},
		}, {
			name: "Cannot use params without Ref",
			Step: v1.Step{
				Image: "my-image",
				Params: v1.Params{{
					Name: "param",
				}},
			},
			expectedError: apis.FieldError{
				Message: "params cannot be used without Ref",
				Paths:   []string{"params"},
			},
		}, {
			name: "Cannot use volumeMounts with Ref",
			Step: v1.Step{
				Ref: &v1.Ref{
					Name: "stepAction",
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "$(params.foo)",
					MountPath: "/registry-config",
				}},
			},
			expectedError: apis.FieldError{
				Message: "volumeMounts cannot be used with Ref",
				Paths:   []string{"volumeMounts"},
			},
		}, {
			name: "Cannot use results with Ref",
			Step: v1.Step{
				Ref: &v1.Ref{
					Name: "stepAction",
				},
				Results: []v1.StepResult{{
					Name: "result",
				}},
			},
			expectedError: apis.FieldError{
				Message: "results cannot be used with Ref",
				Paths:   []string{"results"},
			},
		},
	}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := t.Context()
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

func TestStepOnError(t *testing.T) {
	tests := []struct {
		name          string
		params        []v1.ParamSpec
		step          v1.Step
		expectedError *apis.FieldError
	}{{
		name: "valid step - valid onError usage - set to continue",
		step: v1.Step{
			OnError: v1.Continue,
			Image:   "image",
			Args:    []string{"arg"},
		},
	}, {
		name: "valid step - valid onError usage - set to stopAndFail",
		step: v1.Step{
			OnError: v1.StopAndFail,
			Image:   "image",
			Args:    []string{"arg"},
		},
	}, {
		name: "valid step - valid onError usage - set to continueAndFail",
		steps: []v1.Step{{
			OnError: v1.ContinueAndFail,
			Image:   "image",
			Args:    []string{"arg"},
		}},
	}, {
		name: "valid step - valid onError usage - set to a task parameter",
		params: []v1.ParamSpec{{
			Name:    "CONTINUE",
			Default: &v1.ParamValue{Type: v1.ParamTypeString, StringVal: string(v1.Continue)},
		}},
		step: v1.Step{
			OnError: "$(params.CONTINUE)",
			Image:   "image",
			Args:    []string{"arg"},
		},
	}, {
		name: "invalid step - onError set to invalid value",
		step: v1.Step{
			OnError: "onError",
			Image:   "image",
			Args:    []string{"arg"},
		},
		expectedError: &apis.FieldError{
			Message: `invalid value: "onError"`,
			Paths:   []string{"onError"},
			Details: `Task step onError must be "continue", "stopAndFail" or "continueAndFail"`,
		},
	}}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := t.Context()
			err := st.step.Validate(ctx)
			if st.expectedError == nil && err != nil {
				t.Errorf("No error expected from Step.Validate() but got = %v", err)
			} else if st.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error from Step.Validate() = %v, but got none", st.expectedError)
				} else if d := cmp.Diff(st.expectedError.Error(), err.Error()); d != "" {
					t.Errorf("returned error from Step.Validate() does not match with the expected error: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

// TestStepIncompatibleAPIVersions exercises validation of fields in a Step
// that require a specific feature gate version in order to work.
func TestStepIncompatibleAPIVersions(t *testing.T) {
	versions := []string{"alpha", "beta", "stable"}
	isStricterThen := func(first, second string) bool {
		// assume values are in order alpha (less strict), beta, stable (strictest)
		// return true if first is stricter then second
		switch first {
		case second, "alpha":
			return false
		case "stable":
			return true
		default:
			// first is beta, true is second is alpha, false is second is stable
			return second == "alpha"
		}
	}

	for _, st := range []struct {
		name            string
		requiredVersion string
		step            v1.Step
	}{
		{
			name:            "windows script support requires alpha",
			requiredVersion: "alpha",
			step: v1.Step{
				Image: "my-image",
				Script: `
				#!win powershell -File
				script-1`,
			},
		}, {
			name:            "stdout stream support requires alpha",
			requiredVersion: "alpha",
			step: v1.Step{
				Image: "foo",
				StdoutConfig: &v1.StepOutputConfig{
					Path: "/tmp/stdout.txt",
				},
			},
		}, {
			name:            "stderr stream support requires alpha",
			requiredVersion: "alpha",
			step: v1.Step{
				Image: "foo",
				StderrConfig: &v1.StepOutputConfig{
					Path: "/tmp/stderr.txt",
				},
			},
		},
	} {
		for _, version := range versions {
			testName := fmt.Sprintf("(using %s) %s", version, st.name)
			t.Run(testName, func(t *testing.T) {
				ctx := t.Context()
				if version == "alpha" {
					ctx = cfgtesting.EnableAlphaAPIFields(ctx)
				}
				if version == "beta" {
					ctx = cfgtesting.EnableBetaAPIFields(ctx)
				}
				if version == "stable" {
					ctx = cfgtesting.EnableStableAPIFields(ctx)
				}
				err := st.step.Validate(ctx)

				// If the configured version is stricter than the required one, we expect an error
				if isStricterThen(version, st.requiredVersion) && err == nil {
					t.Fatalf("no error received even though version required is %q while feature gate is %q", st.requiredVersion, version)
				}

				// If the configured version is more permissive than the required one, we expect no error
				if isStricterThen(st.requiredVersion, version) && err != nil {
					t.Fatalf("error received despite required version and feature gate matching %q: %v", version, err)
				}
			})
		}
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

func TestStepWithStepActionReferenceValidate(t *testing.T) {
	tests := []struct {
		name string
		Step v1.Step
	}{{
		name: "valid stepAction ref",
		Step: v1.Step{
			Name: "mystep",
			Ref: &v1.Ref{
				Name: "stepAction",
			},
		},
	}, {
		name: "valid use of params with Ref",
		Step: v1.Step{
			Ref: &v1.Ref{
				Name: "stepAction",
			},
			Params: v1.Params{{
				Name: "param",
			}},
		},
	}}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := config.ToContext(t.Context(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableStepActions: true,
				},
			})
			if err := st.Step.Validate(ctx); err != nil {
				t.Errorf("Step.Validate() = %v", err)
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
