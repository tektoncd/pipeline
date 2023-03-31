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
	dfttesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestPipeline_SetDefaults(t *testing.T) {
	p := &v1.Pipeline{}
	want := &v1.Pipeline{}
	ctx := context.Background()
	p.SetDefaults(ctx)
	if d := cmp.Diff(want, p); d != "" {
		t.Errorf("Mismatch of Pipeline: empty pipeline must not change after setting defaults: %s", diff.PrintWantGot(d))
	}
}

func TestPipelineSpec_SetDefaults(t *testing.T) {
	cases := []struct {
		desc string
		ps   *v1.PipelineSpec
		want *v1.PipelineSpec
	}{{
		desc: "empty pipelineSpec must not change after setting defaults",
		ps:   &v1.PipelineSpec{},
		want: &v1.PipelineSpec{},
	}, {
		desc: "param type - default param type must be " + string(v1.ParamTypeString),
		ps: &v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "string-param",
			}},
		},
		want: &v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "string-param", Type: v1.ParamTypeString,
			}},
		},
	}, {
		desc: "param type - param type must be derived based on the default value " + string(v1.ParamTypeString),
		ps: &v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "string-param",
				Default: &v1.ParamValue{
					StringVal: "foo",
				},
			}},
		},
		want: &v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "string-param", Type: v1.ParamTypeString, Default: &v1.ParamValue{StringVal: "foo"},
			}},
		},
	}, {
		desc: "multiple tasks and final tasks",
		ps: &v1.PipelineSpec{
			Tasks: []v1.PipelineTask{
				{
					Name:    "task-with-taskRef",
					TaskRef: &v1.TaskRef{Name: "bar-task-1"},
				},
				{
					Name: "task-with-taskSpec",
					TaskSpec: &v1.EmbeddedTask{
						TaskSpec: v1.TaskSpec{
							Params: []v1.ParamSpec{{
								Name: "string-param",
							}},
						},
					},
				},
			},
			Finally: []v1.PipelineTask{
				{
					Name:    "final-task-with-taskRef",
					TaskRef: &v1.TaskRef{Name: "foo-task-1"},
				},
				{
					Name: "final-task-with-taskSpec",
					TaskSpec: &v1.EmbeddedTask{
						TaskSpec: v1.TaskSpec{
							Params: []v1.ParamSpec{{
								Name: "string-param",
							}},
						},
					},
				},
			},
		},
		want: &v1.PipelineSpec{
			Tasks: []v1.PipelineTask{
				{
					Name:    "task-with-taskRef",
					TaskRef: &v1.TaskRef{Name: "bar-task-1", Kind: v1.NamespacedTaskKind},
				},
				{
					Name: "task-with-taskSpec",
					TaskSpec: &v1.EmbeddedTask{
						TaskSpec: v1.TaskSpec{
							Params: []v1.ParamSpec{{
								Name: "string-param",
								Type: v1.ParamTypeString,
							}},
						},
					},
				},
			},
			Finally: []v1.PipelineTask{
				{
					Name:    "final-task-with-taskRef",
					TaskRef: &v1.TaskRef{Name: "foo-task-1", Kind: v1.NamespacedTaskKind},
				},
				{
					Name: "final-task-with-taskSpec",
					TaskSpec: &v1.EmbeddedTask{
						TaskSpec: v1.TaskSpec{
							Params: []v1.ParamSpec{{
								Name: "string-param",
								Type: v1.ParamTypeString,
							}},
						},
					},
				},
			},
		},
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			tc.ps.SetDefaults(ctx)
			if d := cmp.Diff(tc.want, tc.ps); d != "" {
				t.Errorf("Mismatch of pipelineSpec after setting defaults: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_SetDefaults(t *testing.T) {
	cases := []struct {
		desc     string
		pt       *v1.PipelineTask
		want     *v1.PipelineTask
		defaults map[string]string
	}{{
		desc: "pipeline task - default task kind must be " + string(v1.NamespacedTaskKind),
		pt: &v1.PipelineTask{
			Name:    "foo",
			TaskRef: &v1.TaskRef{Name: "foo-task"},
		},
		want: &v1.PipelineTask{
			Name: "foo",
			TaskRef: &v1.TaskRef{
				Name: "foo-task",
				Kind: v1.NamespacedTaskKind,
			},
		},
	}, {
		desc: "pipeline task with taskSpec - default param type must be " + string(v1.ParamTypeString),
		pt: &v1.PipelineTask{
			Name: "foo",
			TaskSpec: &v1.EmbeddedTask{
				TaskSpec: v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "string-param",
					}},
				},
			},
		},
		want: &v1.PipelineTask{
			Name: "foo",
			TaskSpec: &v1.EmbeddedTask{
				TaskSpec: v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "string-param",
						Type: v1.ParamTypeString,
					}},
				},
			},
		},
	}, {
		desc: "pipeline task with taskRef - with default resolver",
		pt: &v1.PipelineTask{
			Name:    "foo",
			TaskRef: &v1.TaskRef{},
		},
		want: &v1.PipelineTask{
			Name: "foo",
			TaskRef: &v1.TaskRef{
				Kind: v1.NamespacedTaskKind,
				ResolverRef: v1.ResolverRef{
					Resolver: "git",
				},
			},
		},
		defaults: map[string]string{
			"default-resolver-type": "git",
		},
	}, {
		desc: "pipeline task with taskRef - user-provided resolver overwrites default resolver",
		pt: &v1.PipelineTask{
			Name: "foo",
			TaskRef: &v1.TaskRef{
				ResolverRef: v1.ResolverRef{
					Resolver: "custom resolver",
				},
			},
		},
		want: &v1.PipelineTask{
			Name: "foo",
			TaskRef: &v1.TaskRef{
				Kind: v1.NamespacedTaskKind,
				ResolverRef: v1.ResolverRef{
					Resolver: "custom resolver",
				},
			},
		},
		defaults: map[string]string{
			"default-resolver-type": "git",
		},
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			if len(tc.defaults) > 0 {
				ctx = dfttesting.SetDefaults(context.Background(), t, tc.defaults)
			}
			tc.pt.SetDefaults(ctx)
			if d := cmp.Diff(tc.want, tc.pt); d != "" {
				t.Errorf("Mismatch of pipelineSpec after setting defaults: %s", diff.PrintWantGot(d))
			}
		})
	}
}
