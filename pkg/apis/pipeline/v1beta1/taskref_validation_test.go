/*
Copyright 2019 The Tekton Authors

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

func TestTaskRef_Valid(t *testing.T) {
	tests := []struct {
		name    string
		taskRef *v1beta1.TaskRef
		wc      func(context.Context) context.Context
	}{{
		name: "nil taskref",
	}, {
		name:    "simple taskref",
		taskRef: &v1beta1.TaskRef{Name: "taskrefname"},
	}, {
		name:    "beta feature: valid resolver",
		taskRef: &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}},
		wc:      cfgtesting.EnableBetaAPIFields,
	}, {
		name:    "beta feature: valid resolver with alpha flag",
		taskRef: &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}},
		wc:      cfgtesting.EnableAlphaAPIFields,
	}, {
		name: "beta feature: valid resolver with params",
		taskRef: &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git", Params: v1beta1.Params{{
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
	}, {
		name: "valid bundle",
		taskRef: &v1beta1.TaskRef{
			Name:   "bundled-task",
			Bundle: "gcr.io/my-bundle"},
		wc: enableTektonOCIBundles(t),
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
		taskRef *v1beta1.TaskRef
		wantErr *apis.FieldError
		wc      func(context.Context) context.Context
	}{{
		name:    "missing taskref name",
		taskRef: &v1beta1.TaskRef{},
		wantErr: apis.ErrMissingField("name"),
	}, {
		name: "use of bundle without the feature flag set",
		taskRef: &v1beta1.TaskRef{
			Name:   "my-task",
			Bundle: "docker.io/foo",
		},
		wantErr: apis.ErrGeneric("bundle requires \"enable-tekton-oci-bundles\" feature gate to be true but it is false"),
	}, {
		name: "bundle missing name",
		taskRef: &v1beta1.TaskRef{
			Bundle: "docker.io/foo",
		},
		wantErr: apis.ErrMissingField("name"),
		wc:      enableTektonOCIBundles(t),
	}, {
		name: "invalid bundle reference",
		taskRef: &v1beta1.TaskRef{
			Name:   "my-task",
			Bundle: "invalid reference",
		},
		wantErr: apis.ErrInvalidValue("invalid bundle reference", "bundle", "could not parse reference: invalid reference"),
		wc:      enableTektonOCIBundles(t),
	}, {
		name: "taskref params disallowed without resolver",
		taskRef: &v1beta1.TaskRef{
			ResolverRef: v1beta1.ResolverRef{
				Params: v1beta1.Params{},
			},
		},
		wantErr: apis.ErrMissingField("resolver"),
	}, {
		name: "taskref resolver disallowed in conjunction with taskref name",
		taskRef: &v1beta1.TaskRef{
			Name: "foo",
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "git",
			},
		},
		wantErr: apis.ErrMultipleOneOf("name", "resolver"),
	}, {
		name: "taskref resolver disallowed in conjunction with taskref bundle",
		taskRef: &v1beta1.TaskRef{
			Bundle: "bar",
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "git",
			},
		},
		wantErr: apis.ErrMultipleOneOf("bundle", "resolver"),
		wc:      enableTektonOCIBundles(t),
	}, {
		name: "taskref params disallowed in conjunction with taskref name",
		taskRef: &v1beta1.TaskRef{
			Name: "bar",
			ResolverRef: v1beta1.ResolverRef{
				Params: v1beta1.Params{{
					Name: "foo",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeString,
						StringVal: "bar",
					},
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("name", "params").Also(apis.ErrMissingField("resolver")),
	}, {
		name: "taskref params disallowed in conjunction with taskref bundle",
		taskRef: &v1beta1.TaskRef{
			Bundle: "bar",
			ResolverRef: v1beta1.ResolverRef{
				Params: v1beta1.Params{{
					Name: "foo",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeString,
						StringVal: "bar",
					},
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("bundle", "params").Also(apis.ErrMissingField("resolver")),
		wc:      enableTektonOCIBundles(t),
	}, {
		name:    "invalid taskref name",
		taskRef: &v1beta1.TaskRef{Name: "_foo"},
		wantErr: &apis.FieldError{
			Message: `invalid value: name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
			Paths:   []string{"name"},
		},
	}, {
		name: "taskref param object requires beta",
		taskRef: &v1beta1.TaskRef{
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "some-resolver",
				Params: v1beta1.Params{{
					Name: "foo",
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{"bar": "baz"},
					},
				}},
			},
		},
		wc: cfgtesting.EnableStableAPIFields,
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
