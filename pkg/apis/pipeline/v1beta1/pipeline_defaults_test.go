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

	"github.com/tektoncd/pipeline/test/diff"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func TestPipeline_SetDefaults(t *testing.T) {
	p := &v1beta1.Pipeline{}
	want := &v1beta1.Pipeline{}
	ctx := context.Background()
	p.SetDefaults(ctx)
	if d := cmp.Diff(want, p); d != "" {
		t.Errorf("Mismatch of Pipeline: empty pipeline must not change after setting defaults: %s", diff.PrintWantGot(d))
	}
}

func TestPipelineSpec_SetDefaults(t *testing.T) {
	cases := []struct {
		desc string
		ps   *v1beta1.PipelineSpec
		want *v1beta1.PipelineSpec
	}{{
		desc: "empty pipelineSpec must not change after setting defaults",
		ps:   &v1beta1.PipelineSpec{},
		want: &v1beta1.PipelineSpec{},
	}, {
		desc: "pipeline task - default task kind must be " + string(v1beta1.NamespacedTaskKind),
		ps: &v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
			}},
		},
		want: &v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task", Kind: v1beta1.NamespacedTaskKind},
			}},
		},
	}, {
		desc: "final pipeline task - default task kind must be " + string(v1beta1.NamespacedTaskKind),
		ps: &v1beta1.PipelineSpec{
			Finally: []v1beta1.PipelineTask{{
				Name: "final-task", TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
			}},
		},
		want: &v1beta1.PipelineSpec{
			Finally: []v1beta1.PipelineTask{{
				Name: "final-task", TaskRef: &v1beta1.TaskRef{Name: "foo-task", Kind: v1beta1.NamespacedTaskKind},
			}},
		},
	}, {
		desc: "param type - default param type must be " + string(v1beta1.ParamTypeString),
		ps: &v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{{
				Name: "string-param",
			}},
		},
		want: &v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{{
				Name: "string-param", Type: v1beta1.ParamTypeString,
			}},
		},
	}, {
		desc: "pipeline task with taskSpec - default param type must be " + string(v1beta1.ParamTypeString),
		ps: &v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "foo", TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{
						Params: []v1beta1.ParamSpec{{
							Name: "string-param",
						}},
					},
				},
			}},
		},
		want: &v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "foo", TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{
						Params: []v1beta1.ParamSpec{{
							Name: "string-param",
							Type: v1beta1.ParamTypeString,
						}},
					},
				},
			}},
		},
	}, {
		desc: "final pipeline task with taskSpec - default param type must be " + string(v1beta1.ParamTypeString),
		ps: &v1beta1.PipelineSpec{
			Finally: []v1beta1.PipelineTask{{
				Name: "foo", TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{
						Params: []v1beta1.ParamSpec{{
							Name: "string-param",
						}},
					},
				},
			}},
		},
		want: &v1beta1.PipelineSpec{
			Finally: []v1beta1.PipelineTask{{
				Name: "foo", TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{
						Params: []v1beta1.ParamSpec{{
							Name: "string-param",
							Type: v1beta1.ParamTypeString,
						}},
					},
				},
			}},
		},
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			tc.ps.SetDefaults(ctx)
			if d := cmp.Diff(tc.want, tc.ps); d != "" {
				t.Errorf("Mismatch of pipelineSpec after setting defaults: %s: %s", tc.desc, diff.PrintWantGot(d))
			}
		})
	}
}
