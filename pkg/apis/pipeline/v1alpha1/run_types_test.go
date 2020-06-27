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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func TestGetParams(t *testing.T) {
	for _, c := range []struct {
		desc string
		spec v1alpha1.RunSpec
		name string
		want *v1beta1.Param
	}{{
		desc: "no params",
		spec: v1alpha1.RunSpec{},
		name: "anything",
		want: nil,
	}, {
		desc: "found",
		spec: v1alpha1.RunSpec{
			Params: []v1beta1.Param{{
				Name:  "first",
				Value: v1beta1.NewArrayOrString("blah"),
			}, {
				Name:  "foo",
				Value: v1beta1.NewArrayOrString("bar"),
			}},
		},
		name: "foo",
		want: &v1beta1.Param{
			Name:  "foo",
			Value: v1beta1.NewArrayOrString("bar"),
		},
	}, {
		desc: "not found",
		spec: v1alpha1.RunSpec{
			Params: []v1beta1.Param{{
				Name:  "first",
				Value: v1beta1.NewArrayOrString("blah"),
			}, {
				Name:  "foo",
				Value: v1beta1.NewArrayOrString("bar"),
			}},
		},
		name: "bar",
		want: nil,
	}, {
		// This shouldn't happen since it's invalid, but just in
		// case, GetParams just returns the first param it finds with
		// the specified name.
		desc: "multiple with same name",
		spec: v1alpha1.RunSpec{
			Params: []v1beta1.Param{{
				Name:  "first",
				Value: v1beta1.NewArrayOrString("blah"),
			}, {
				Name:  "foo",
				Value: v1beta1.NewArrayOrString("bar"),
			}, {
				Name:  "foo",
				Value: v1beta1.NewArrayOrString("second bar"),
			}},
		},
		name: "foo",
		want: &v1beta1.Param{
			Name:  "foo",
			Value: v1beta1.NewArrayOrString("bar"),
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := c.spec.GetParam(c.name)
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("Diff(-want,+got): %v", d)
			}
		})
	}
}
