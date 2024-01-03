/*
Copyright 2022 The Tekton Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	"knative.dev/pkg/apis"
)

func TestPipelineRef_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		ref         *v1.PipelineRef
		wantErr     *apis.FieldError
		withContext func(context.Context) context.Context
	}{{
		name:    "pipelineRef without Pipeline Name",
		ref:     &v1.PipelineRef{},
		wantErr: apis.ErrMissingField("name"),
	}, {
		name: "pipelineref resolver disallowed without beta feature gate",
		ref: &v1.PipelineRef{
			ResolverRef: v1.ResolverRef{
				Resolver: "foo",
			},
		},
		withContext: cfgtesting.EnableStableAPIFields,
		wantErr:     apis.ErrGeneric("resolver requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\""),
	}, {
		name: "pipelineref params disallowed without beta feature gate",
		ref: &v1.PipelineRef{
			ResolverRef: v1.ResolverRef{
				Params: v1.Params{},
			},
		},
		withContext: cfgtesting.EnableStableAPIFields,
		wantErr:     apis.ErrMissingField("resolver").Also(apis.ErrGeneric("resolver params requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\"")),
	}, {
		name: "pipelineref params disallowed without resolver",
		ref: &v1.PipelineRef{
			ResolverRef: v1.ResolverRef{
				Params: v1.Params{},
			},
		},
		wantErr:     apis.ErrMissingField("resolver"),
		withContext: cfgtesting.EnableBetaAPIFields,
	}, {
		name: "pipelineref resolver disallowed in conjunction with pipelineref name",
		ref: &v1.PipelineRef{
			Name: "foo",
			ResolverRef: v1.ResolverRef{
				Resolver: "bar",
			},
		},
		wantErr:     apis.ErrMultipleOneOf("name", "resolver"),
		withContext: cfgtesting.EnableBetaAPIFields,
	}, {
		name: "pipelineref params disallowed in conjunction with pipelineref name",
		ref: &v1.PipelineRef{
			Name: "bar",
			ResolverRef: v1.ResolverRef{
				Params: v1.Params{{
					Name: "foo",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "bar",
					},
				}},
			},
		},
		wantErr:     apis.ErrMultipleOneOf("name", "params").Also(apis.ErrMissingField("resolver")),
		withContext: cfgtesting.EnableBetaAPIFields,
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
		ref  *v1.PipelineRef
		wc   func(context.Context) context.Context
	}{{
		name: "no pipelineRef",
		ref:  nil,
	}, {
		name: "beta feature: valid resolver",
		ref:  &v1.PipelineRef{ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wc:   cfgtesting.EnableBetaAPIFields,
	}, {
		name: "beta feature: valid resolver with alpha flag",
		ref:  &v1.PipelineRef{ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wc:   cfgtesting.EnableAlphaAPIFields,
	}, {
		name: "alpha feature: valid resolver with params",
		ref: &v1.PipelineRef{ResolverRef: v1.ResolverRef{Resolver: "git", Params: v1.Params{{
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
