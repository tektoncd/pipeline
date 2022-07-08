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
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestPipelineRef_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		ref         *v1beta1.PipelineRef
		wantErr     *apis.FieldError
		withContext func(context.Context) context.Context
	}{{
		name: "use of bundle without the feature flag set",
		ref: &v1beta1.PipelineRef{
			Name:   "my-pipeline",
			Bundle: "docker.io/foo",
		},
		wantErr: apis.ErrGeneric("bundle requires \"enable-tekton-oci-bundles\" feature gate to be true but it is false"),
	}, {
		name: "bundle missing name",
		ref: &v1beta1.PipelineRef{
			Bundle: "docker.io/foo",
		},
		wantErr:     apis.ErrMissingField("name"),
		withContext: enableTektonOCIBundles(t),
	}, {
		name: "invalid bundle reference",
		ref: &v1beta1.PipelineRef{
			Name:   "my-pipeline",
			Bundle: "not a valid reference",
		},
		wantErr:     apis.ErrInvalidValue("invalid bundle reference", "bundle", "could not parse reference: not a valid reference"),
		withContext: enableTektonOCIBundles(t),
	}, {
		name:    "pipelineRef without Pipeline Name",
		ref:     &v1beta1.PipelineRef{},
		wantErr: apis.ErrMissingField("name"),
	}, {
		name: "pipelineref resolver disallowed without alpha feature gate",
		ref: &v1beta1.PipelineRef{
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "foo",
			},
		},
		wantErr: apis.ErrGeneric("resolver requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
	}, {
		name: "pipelineref resource disallowed without alpha feature gate",
		ref: &v1beta1.PipelineRef{
			ResolverRef: v1beta1.ResolverRef{
				Resource: []v1beta1.ResolverParam{},
			},
		},
		wantErr: apis.ErrMissingField("resolver").Also(apis.ErrGeneric("resource requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\"")),
	}, {
		name: "pipelineref resource disallowed without resolver",
		ref: &v1beta1.PipelineRef{
			ResolverRef: v1beta1.ResolverRef{
				Resource: []v1beta1.ResolverParam{},
			},
		},
		wantErr:     apis.ErrMissingField("resolver"),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "pipelineref resolver disallowed in conjunction with pipelineref name",
		ref: &v1beta1.PipelineRef{
			Name: "foo",
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "bar",
			},
		},
		wantErr:     apis.ErrMultipleOneOf("name", "resolver"),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "pipelineref resolver disallowed in conjunction with pipelineref bundle",
		ref: &v1beta1.PipelineRef{
			Bundle: "foo",
			ResolverRef: v1beta1.ResolverRef{
				Resolver: "baz",
			},
		},
		wantErr:     apis.ErrMultipleOneOf("bundle", "resolver"),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "pipelineref resource disallowed in conjunction with pipelineref name",
		ref: &v1beta1.PipelineRef{
			Name: "bar",
			ResolverRef: v1beta1.ResolverRef{
				Resource: []v1beta1.ResolverParam{{
					Name:  "foo",
					Value: "bar",
				}},
			},
		},
		wantErr:     apis.ErrMultipleOneOf("name", "resource").Also(apis.ErrMissingField("resolver")),
		withContext: config.EnableAlphaAPIFields,
	}, {
		name: "pipelineref resource disallowed in conjunction with pipelineref bundle",
		ref: &v1beta1.PipelineRef{
			Bundle: "bar",
			ResolverRef: v1beta1.ResolverRef{
				Resource: []v1beta1.ResolverParam{{
					Name:  "foo",
					Value: "bar",
				}},
			},
		},
		wantErr:     apis.ErrMultipleOneOf("bundle", "resource").Also(apis.ErrMissingField("resolver")),
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
		ref  *v1beta1.PipelineRef
		wc   func(context.Context) context.Context
	}{{
		name: "no pipelineRef",
		ref:  nil,
	}, {
		name: "alpha feature: valid resolver",
		ref:  &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}},
		wc:   config.EnableAlphaAPIFields,
	}, {
		name: "alpha feature: valid resolver with resource parameters",
		ref: &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git", Resource: []v1beta1.ResolverParam{{
			Name:  "repo",
			Value: "https://github.com/tektoncd/pipeline.git",
		}, {
			Name:  "branch",
			Value: "baz",
		}}}},
		wc: config.EnableAlphaAPIFields,
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

func enableTektonOCIBundles(t *testing.T) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		s := config.NewStore(logtesting.TestLogger(t))
		s.OnConfigChanged(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
			Data: map[string]string{
				"enable-tekton-oci-bundles": "true",
			},
		})
		return s.ToContext(ctx)
	}
}
