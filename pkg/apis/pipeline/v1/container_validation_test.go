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
			Details: `Task step onError must be either "continue" or "stopAndFail"`,
		},
	}}
	for _, st := range tests {
		t.Run(st.name, func(t *testing.T) {
			ctx := context.Background()
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
				ctx := context.Background()
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
