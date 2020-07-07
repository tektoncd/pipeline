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

package controller_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/controller"
)

const (
	apiVersion = "example.dev/v0"
	kind       = "Example"
)

func TestFilterRunRef(t *testing.T) {
	for _, c := range []struct {
		desc string
		in   interface{}
		want bool
	}{{
		desc: "not a Run",
		in:   struct{}{},
		want: false,
	}, {
		desc: "nil Run",
		in:   (*v1alpha1.Run)(nil),
		want: false,
	}, {
		desc: "nil ref",
		in: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: nil,
			},
		},
		want: false,
	}, {
		desc: "Run without matching apiVersion",
		in: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "not-matching",
					Kind:       kind,
				},
			},
		},
		want: false,
	}, {
		desc: "Run without matching kind",
		in: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: apiVersion,
					Kind:       "not-matching",
				},
			},
		},
		want: false,
	}, {
		desc: "Run with matching apiVersion and kind",
		in: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: apiVersion,
					Kind:       kind,
				},
			},
		},
		want: true,
	}, {
		desc: "Run with matching apiVersion and kind and name",
		in: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: apiVersion,
					Kind:       kind,
					Name:       "some-name",
				},
			},
		},
		want: true,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := controller.FilterRunRef(apiVersion, kind)(c.in)
			if got != c.want {
				t.Fatalf("FilterRunRef(%q, %q) got %t, want %t", apiVersion, kind, got, c.want)
			}
		})
	}
}
