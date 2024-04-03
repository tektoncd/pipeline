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

	"github.com/google/go-cmp/cmp"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"knative.dev/pkg/apis"
)

func TestPipelineRef_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		ref         *v1beta1.PipelineRef
		wantErr     *apis.FieldError
		withContext func(context.Context) context.Context
	}{{
		name:    "pipelineRef without Pipeline Name",
		ref:     &v1beta1.PipelineRef{},
		wantErr: apis.ErrMissingField("name"),
	}, {
		name: "invalid pipelineref name",
		ref:  &v1beta1.PipelineRef{Name: "_foo"},
		wantErr: &apis.FieldError{
			Message: `invalid value: name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
			Paths:   []string{"name"},
		},
	}, {
		name: "pipelineref resolver disallowed without beta feature gate",
		ref: &v1beta1.PipelineRef{
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "foo",
			},
		},
		withContext: cfgtesting.EnableStableAPIFields,
		wantErr:     apis.ErrGeneric("resolver requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\""),
	}, {
		name: "pipelineref params disallowed without beta feature gate",
		ref: &v1beta1.PipelineRef{
			ResolverRef: v1beta1.ResolverRef{
				Params: v1beta1.Params{},
			},
		},
		withContext: cfgtesting.EnableStableAPIFields,
		wantErr:     apis.ErrMissingField("resolver").Also(apis.ErrGeneric("resolver params requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\"")),
	}, {
		name: "pipelineref params disallowed without resolver",
		ref: &v1beta1.PipelineRef{
			ResolverRef: v1beta1.ResolverRef{
				Params: v1beta1.Params{},
			},
		},
		wantErr:     apis.ErrMissingField("resolver"),
		withContext: cfgtesting.EnableBetaAPIFields,
	}, {
		name: "pipelineRef with resolver and k8s style name",
		ref: &v1beta1.PipelineRef{
			Name: "foo",
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "git",
			},
		},
		wantErr:     apis.ErrInvalidValue(`parse "foo": invalid URI for request`, "name"),
		withContext: enableConciseResolverSyntax,
	}, {
		name: "pipelineRef with url-like name without resolver",
		ref: &v1beta1.PipelineRef{
			Name: "https://foo.com/bar",
		},
		wantErr:     apis.ErrMissingField("resolver"),
		withContext: enableConciseResolverSyntax,
	}, {
		name: "pipelineRef params disallowed in conjunction with pipelineref name",
		ref: &v1beta1.PipelineRef{
			Name: "https://foo/bar",
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "git",
				Params:   []v1beta1.Param{{Name: "foo", Value: v1beta1.ParamValue{StringVal: "bar"}}},
			},
		},
		wantErr:     apis.ErrMultipleOneOf("name", "params"),
		withContext: enableConciseResolverSyntax,
	}, {
		name: "pipelineRef with url-like name without enable-concise-resolver-syntax",
		ref:  &v1beta1.PipelineRef{Name: "https://foo.com/bar"},
		wantErr: apis.ErrMissingField("resolver").Also(&apis.FieldError{
			Message: `feature flag enable-concise-resolver-syntax should be set to true to use concise resolver syntax`,
		}),
	}, {
		name: "pipelineRef without enable-concise-resolver-syntax",
		ref:  &v1beta1.PipelineRef{Name: "https://foo.com/bar", ResolverRef: v1beta1.ResolverRef{Resolver: "git"}},
		wantErr: &apis.FieldError{
			Message: `feature flag enable-concise-resolver-syntax should be set to true to use concise resolver syntax`,
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.withContext != nil {
				ctx = tc.withContext(ctx)
			}
			err := tc.ref.Validate(ctx)
			if d := cmp.Diff(tc.wantErr.Error(), err.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRef_Valid(t *testing.T) {
	tests := []struct {
		name string
		ref  *v1beta1.PipelineRef
		wc   func(context.Context) context.Context
	}{{
		name: "no pipelineRef",
		ref:  nil,
	}, {
		name: "beta feature: valid resolver",
		ref:  &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}},
		wc:   cfgtesting.EnableAlphaAPIFields,
	}, {
		name: "beta feature: valid resolver with alpha flag",
		ref:  &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}},
		wc:   cfgtesting.EnableAlphaAPIFields,
	}, {
		name: "beta feature: valid resolver with params with alpha flag",
		ref: &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git", Params: v1beta1.Params{{
			Name: "repo",
			Value: v1beta1.ParamValue{
				Type:      v1beta1.ParamTypeString,
				StringVal: "https://github.com/tektoncd/pipeline.git",
			},
		}, {
			Name: "branch",
			Value: v1beta1.ParamValue{
				Type:      v1beta1.ParamTypeString,
				StringVal: "baz",
			},
		}}}},
		wc: cfgtesting.EnableAlphaAPIFields,
	}}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			if err := ts.ref.Validate(ctx); err != nil {
				t.Error(err)
			}
		})
	}
}
