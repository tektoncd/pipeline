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

func TestTaskRef_Valid(t *testing.T) {
	tests := []struct {
		name    string
		taskRef *v1.TaskRef
		wc      func(context.Context) context.Context
	}{{
		name: "nil taskref",
	}, {
		name:    "simple taskref",
		taskRef: &v1.TaskRef{Name: "taskrefname"},
	}, {
		name:    "beta feature: valid resolver",
		taskRef: &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wc:      config.EnableBetaAPIFields,
	}, {
		name:    "beta feature: valid resolver with alpha flag",
		taskRef: &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git"}},
		wc:      config.EnableAlphaAPIFields,
	}, {
		name: "beta feature: valid resolver with params",
		taskRef: &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git", Params: v1.Params{{
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
		wc: config.EnableBetaAPIFields,
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			if err := ts.taskRef.Validate(ctx); err != nil {
				t.Errorf("TaskRef.Validate() error = %v", err)
			}
		})
	}
}

func TestTaskRef_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		taskRef *v1.TaskRef
		wantErr *apis.FieldError
		wc      func(context.Context) context.Context
	}{{
		name:    "missing taskref name",
		taskRef: &v1.TaskRef{},
		wantErr: apis.ErrMissingField("name"),
	}, {
		name: "taskref resolver disallowed without beta feature gate",
		taskRef: &v1.TaskRef{
			ResolverRef: v1.ResolverRef{
				Resolver: "git",
			},
		},
		wc:      config.EnableStableAPIFields,
		wantErr: apis.ErrGeneric("resolver requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\""),
	}, {
		name: "taskref params disallowed without beta feature gate",
		taskRef: &v1.TaskRef{
			ResolverRef: v1.ResolverRef{
				Params: v1.Params{},
			},
		},
		wc:      config.EnableStableAPIFields,
		wantErr: apis.ErrMissingField("resolver").Also(apis.ErrGeneric("resolver params requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\"")),
	}, {
		name: "taskref params disallowed without resolver",
		taskRef: &v1.TaskRef{
			ResolverRef: v1.ResolverRef{
				Params: v1.Params{},
			},
		},
		wantErr: apis.ErrMissingField("resolver"),
		wc:      config.EnableBetaAPIFields,
	}, {
		name: "taskref resolver disallowed in conjunction with taskref name",
		taskRef: &v1.TaskRef{
			Name: "foo",
			ResolverRef: v1.ResolverRef{
				Resolver: "git",
			},
		},
		wantErr: apis.ErrMultipleOneOf("name", "resolver"),
		wc:      config.EnableBetaAPIFields,
	}, {
		name: "taskref params disallowed in conjunction with taskref name",
		taskRef: &v1.TaskRef{
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
		wantErr: apis.ErrMultipleOneOf("name", "params").Also(apis.ErrMissingField("resolver")),
		wc:      config.EnableBetaAPIFields,
	}, {
		name: "taskref param object requires beta",
		taskRef: &v1.TaskRef{
			ResolverRef: v1.ResolverRef{
				Resolver: "some-resolver",
				Params: v1.Params{{
					Name: "foo",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"bar": "baz"},
					},
				}},
			},
		},
		wc: config.EnableStableAPIFields,
		wantErr: apis.ErrGeneric("resolver requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\"").Also(
			apis.ErrGeneric("resolver params requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\"")).Also(
			apis.ErrGeneric("object type parameter requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\"")),
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()
			if ts.wc != nil {
				ctx = ts.wc(ctx)
			}
			err := ts.taskRef.Validate(ctx)
			if d := cmp.Diff(ts.wantErr.Error(), err.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
