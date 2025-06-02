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
	"testing"

	"github.com/google/go-cmp/cmp"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestTask_SetDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   *v1.Task
		want *v1.Task
	}{{
		name: "remote Ref uses default resolver",
		in: &v1.Task{
			Spec: v1.TaskSpec{
				Steps: []v1.Step{{
					Name: "foo",
					Ref:  &v1.Ref{},
				}},
			},
		},
		want: &v1.Task{
			Spec: v1.TaskSpec{
				Steps: []v1.Step{{
					Name: "foo",
					Ref:  &v1.Ref{ResolverRef: v1.ResolverRef{Resolver: "git"}},
				}},
			},
		},
	}, {
		name: "remote Ref has resolver not overwritten by default resolver",
		in: &v1.Task{
			Spec: v1.TaskSpec{
				Steps: []v1.Step{{
					Name: "foo",
					Ref:  &v1.Ref{ResolverRef: v1.ResolverRef{Resolver: "bundle"}},
				}},
			},
		},
		want: &v1.Task{
			Spec: v1.TaskSpec{
				Steps: []v1.Step{{
					Name: "foo",
					Ref:  &v1.Ref{ResolverRef: v1.ResolverRef{Resolver: "bundle"}},
				}},
			},
		},
	}, {
		name: "local Ref not adding default resolver",
		in: &v1.Task{
			Spec: v1.TaskSpec{
				Steps: []v1.Step{{
					Name: "foo",
					Ref:  &v1.Ref{Name: "local"},
				}},
			},
		},
		want: &v1.Task{
			Spec: v1.TaskSpec{
				Steps: []v1.Step{{
					Name: "foo",
					Ref:  &v1.Ref{Name: "local"},
				}},
			},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := cfgtesting.SetDefaults(t.Context(), t, map[string]string{
				"default-resolver-type": "git",
			})
			got := tc.in
			got.SetDefaults(ctx)
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("SetDefaults %s", diff.PrintWantGot(d))
			}
		})
	}
}
