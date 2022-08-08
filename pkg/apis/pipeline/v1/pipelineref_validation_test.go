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
	"github.com/tektoncd/pipeline/pkg/apis/config"
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
		name: "pipelineref resolver disallowed without alpha feature gate",
		ref: &v1.PipelineRef{
			ResolverRef: v1.ResolverRef{
				Resolver: "foo",
			},
		},
		wantErr: apis.ErrGeneric("resolver requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
	}, {
		name: "pipelineref resolver disallowed in conjunction with pipelineref name",
		ref: &v1.PipelineRef{
			Name: "foo",
			ResolverRef: v1.ResolverRef{
				Resolver: "bar",
			},
		},
		wantErr:     apis.ErrMultipleOneOf("name", "resolver"),
		withContext: config.EnableAlphaAPIFields,
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
		name: "alpha feature: valid resolver",
		ref:  &v1.PipelineRef{ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wc:   config.EnableAlphaAPIFields,
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
