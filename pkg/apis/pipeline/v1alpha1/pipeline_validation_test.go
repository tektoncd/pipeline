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

package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestPipelineSpec_Validate_Error(t *testing.T) {
	tests := []struct {
		name string
		p    *v1alpha1.Pipeline
	}{
		{
			name: "duplicate tasks",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineTask("foo", "foo-task"),
				tb.PipelineTask("foo", "foo-task"),
			)),
		},
		{
			name: "from is on first task",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskInputResource("the-resource", "great-resource", tb.From("bar"))),
			)),
		},
		{
			name: "from task doesnt exist",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
				tb.PipelineTask("baz", "baz-task"),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskInputResource("the-resource", "great-resource", tb.From("bar"))),
			)),
		},
		{
			name: "unused resources declared",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
				tb.PipelineDeclaredResource("extra-resource", v1alpha1.PipelineResourceTypeImage),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskInputResource("the-resource", "great-resource")),
			)),
		},
		{
			name: "output resources missing from declaration",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskInputResource("the-resource", "great-resource"),
					tb.PipelineTaskOutputResource("the-magic-resource", "missing-resource")),
			)),
		},
		{
			name: "input resources missing from declaration",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskInputResource("the-resource", "missing-resource"),
					tb.PipelineTaskOutputResource("the-magic-resource", "great-resource")),
			)),
		},
		{
			name: "from resource isn't output by task",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
				tb.PipelineDeclaredResource("wonderful-resource", v1alpha1.PipelineResourceTypeImage),
				tb.PipelineTask("bar", "bar-task",
					tb.PipelineTaskInputResource("some-workspace", "great-resource")),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskInputResource("wow-image", "wonderful-resource", tb.From("bar"))),
			)),
		},
		{
			name: "not defined parameter variable",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskParam("a-param", "${params.does-not-exist}")))),
		},
		{
			name: "not defined parameter variable with defined",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineParam("foo", tb.PipelineParamDefault("something")),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskParam("a-param", "${params.foo} and ${params.does-not-exist}")))),
		},
		{
			name: "invalid dependency graph between the tasks",
			p: tb.Pipeline("foo", "namespace", tb.PipelineSpec(
				tb.PipelineTask("foo", "foo", tb.RunAfter("bar")),
				tb.PipelineTask("bar", "bar", tb.RunAfter("foo")),
			)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.Spec.Validate(context.Background()); err == nil {
				t.Error("PipelineSpec.Validate() did not return error, wanted error")
			}
		})
	}
}

func TestPipelineSpec_Validate_Valid(t *testing.T) {
	tests := []struct {
		name string
		p    *v1alpha1.Pipeline
	}{
		{
			name: "no duplicate tasks",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineTask("foo", "foo-task"),
				tb.PipelineTask("bar", "bar-task"),
			)),
		},
		{
			// Adding this case because `task.Resources` is a pointer, explicitly making sure this is handled
			name: "task without resources",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineDeclaredResource("wonderful-resource", v1alpha1.PipelineResourceTypeImage),
				tb.PipelineTask("bar", "bar-task"),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskInputResource("wow-image", "wonderful-resource")),
			)),
		},
		{
			name: "valid resource declarations and usage",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
				tb.PipelineDeclaredResource("wonderful-resource", v1alpha1.PipelineResourceTypeImage),
				tb.PipelineTask("bar", "bar-task",
					tb.PipelineTaskInputResource("some-workspace", "great-resource"),
					tb.PipelineTaskOutputResource("some-image", "wonderful-resource")),
				tb.PipelineTask("foo", "foo-task",
					tb.PipelineTaskInputResource("wow-image", "wonderful-resource", tb.From("bar"))),
			)),
		},
		{
			name: "valid parameter variables",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineParam("baz"),
				tb.PipelineParam("foo-is-baz"),
				tb.PipelineTask("bar", "bar-task",
					tb.PipelineTaskParam("a-param", "${baz} and ${foo-is-baz}")),
			)),
		},
		{
			name: "pipeline parameter nested in task parameter",
			p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
				tb.PipelineParam("baz"),
				tb.PipelineTask("bar", "bar-task",
					tb.PipelineTaskParam("a-param", "${input.workspace.${baz}}")),
			)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.Spec.Validate(context.Background()); err != nil {
				t.Errorf("PipelineSpec.Validate() returned error: %v", err)
			}
		})
	}
}
