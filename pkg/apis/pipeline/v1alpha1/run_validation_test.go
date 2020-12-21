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

package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestRun_Invalid(t *testing.T) {
	for _, c := range []struct {
		name string
		run  *v1alpha1.Run
		want *apis.FieldError
	}{{
		name: "missing spec",
		run:  &v1alpha1.Run{},
		want: apis.ErrMissingField("spec"),
	}, {
		name: "invalid metadata",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{Name: "run.name"},
		},
		want: &apis.FieldError{
			Message: "Invalid resource name: special character . must not be present",
			Paths:   []string{"metadata.name"},
		},
	}, {
		name: "missing ref",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: nil,
			},
		},
		want: apis.ErrMissingField("spec"),
	}, {
		name: "missing apiVersion",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "",
				},
			},
		},
		want: apis.ErrMissingField("spec.ref.apiVersion"),
	}, {
		name: "missing kind",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "",
				},
			},
		},
		want: apis.ErrMissingField("spec.ref.kind"),
	}, {
		name: "non-unique params",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Params: []v1beta1.Param{{
					Name:  "foo",
					Value: *v1beta1.NewArrayOrString("foo"),
				}, {
					Name:  "foo",
					Value: *v1beta1.NewArrayOrString("foo"),
				}},
			},
		},
		want: apis.ErrMultipleOneOf("spec.params"),
	}} {
		t.Run(c.name, func(t *testing.T) {
			err := c.run.Validate(context.Background())
			if d := cmp.Diff(err.Error(), c.want.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestRun_Valid(t *testing.T) {
	for _, c := range []struct {
		name string
		run  *v1alpha1.Run
	}{{
		name: "no params",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
					Name:       "blah",
				},
			},
		},
	}, {
		name: "unnamed",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
			},
		},
	}, {
		name: "unique params",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Params: []v1beta1.Param{{
					Name:  "foo",
					Value: *v1beta1.NewArrayOrString("foo"),
				}, {
					Name:  "bar",
					Value: *v1beta1.NewArrayOrString("bar"),
				}},
			},
		},
	}} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.run.Validate(context.Background()); err != nil {
				t.Fatalf("validating valid Run: %v", err)
			}
		})
	}
}
