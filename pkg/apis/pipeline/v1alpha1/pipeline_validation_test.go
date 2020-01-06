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
	corev1 "k8s.io/api/core/v1"
)

func TestPipeline_Validate(t *testing.T) {
	tests := []struct {
		name            string
		p               *v1alpha1.Pipeline
		failureExpected bool
	}{{
		name: "valid metadata",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo-task"),
		)),
		failureExpected: false,
	}, {
		name: "period in name",
		p: tb.Pipeline("pipe.line", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo-task"),
		)),
		failureExpected: true,
	}, {
		name: "pipeline name too long",
		p: tb.Pipeline("asdf123456789012345678901234567890123456789012345678901234567890", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo-task"),
		)),
		failureExpected: true,
	}, {
		name: "pipeline spec invalid (duplicate tasks)",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo-task"),
			tb.PipelineTask("foo", "foo-task"),
		)),
		failureExpected: true,
	}, {
		name: "pipeline spec empty task name",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("", "foo-task"),
		)),
		failureExpected: true,
	}, {
		name: "pipeline spec invalid task name",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("_foo", "foo-task"),
		)),
		failureExpected: true,
	}, {
		name: "pipeline spec invalid taskref name",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "_foo-task"),
		)),
		failureExpected: true,
	}, {
		name: "pipeline spec missing tasfref and taskspec",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("", ""),
			tb.PipelineTask("", "", tb.PipelineTaskSpec(&v1alpha1.TaskSpec{})),
		)),
		failureExpected: true,
	}, {
		name: "pipeline spec with taskref and taskspec",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo-task", tb.PipelineTaskSpec(&v1alpha1.TaskSpec{
				Steps: []v1alpha1.Step{{Container: corev1.Container{
					Name:  "foo",
					Image: "bar",
				}}},
			},
			)))),
		failureExpected: true,
	}, {
		name: "pipeline spec invalid taskspec",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("", "", tb.PipelineTaskSpec(&v1alpha1.TaskSpec{})),
		)),
		failureExpected: true,
	}, {
		name: "pipeline spec valid taskspec",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("", "", tb.PipelineTaskSpec(&v1alpha1.TaskSpec{
				Steps: []v1alpha1.Step{{Container: corev1.Container{
					Name:  "foo",
					Image: "bar",
				}}},
			},
			)))),
		failureExpected: false,
	}, {
		name: "no duplicate tasks",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo-task"),
			tb.PipelineTask("bar", "bar-task"),
		)),
		failureExpected: false,
	}, {
		// Adding this case because `task.Resources` is a pointer, explicitly making sure this is handled
		name: "task without resources",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineDeclaredResource("wonderful-resource", v1alpha1.PipelineResourceTypeImage),
			tb.PipelineTask("bar", "bar-task"),
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskInputResource("wow-image", "wonderful-resource")),
		)),
		failureExpected: false,
	}, {
		name: "valid resource declarations and usage",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
			tb.PipelineDeclaredResource("wonderful-resource", v1alpha1.PipelineResourceTypeImage),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskInputResource("some-workspace", "great-resource"),
				tb.PipelineTaskOutputResource("some-image", "wonderful-resource"),
				tb.PipelineTaskCondition("some-condition",
					tb.PipelineTaskConditionResource("some-workspace", "great-resource"))),
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskInputResource("wow-image", "wonderful-resource", tb.From("bar")),
				tb.PipelineTaskCondition("some-condition-2",
					tb.PipelineTaskConditionResource("wow-image", "wonderful-resource", "bar"))),
		)),
		failureExpected: false,
	}, {
		name: "valid condition only resource",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskCondition("some-condition",
					tb.PipelineTaskConditionResource("some-workspace", "great-resource"))),
		)),
		failureExpected: false,
	}, {
		name: "valid parameter variables",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("baz", v1alpha1.ParamTypeString),
			tb.PipelineParamSpec("foo-is-baz", v1alpha1.ParamTypeString),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskParam("a-param", "$(baz) and $(foo-is-baz)")),
		)),
		failureExpected: false,
	}, {
		name: "valid array parameter variables",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("baz", v1alpha1.ParamTypeArray, tb.ParamSpecDefault("some", "default")),
			tb.PipelineParamSpec("foo-is-baz", v1alpha1.ParamTypeArray),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskParam("a-param", "$(baz)", "and", "$(foo-is-baz)")),
		)),
		failureExpected: false,
	}, {
		name: "pipeline parameter nested in task parameter",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("baz", v1alpha1.ParamTypeString),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskParam("a-param", "$(input.workspace.$(baz))")),
		)),
		failureExpected: false,
	}, {
		name: "from is on first task",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskInputResource("the-resource", "great-resource", tb.From("bar"))),
		)),
		failureExpected: true,
	}, {
		name: "from task doesnt exist",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
			tb.PipelineTask("baz", "baz-task"),
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskInputResource("the-resource", "great-resource", tb.From("bar"))),
		)),
		failureExpected: true,
	}, {
		name: "output resources missing from declaration",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskInputResource("the-resource", "great-resource"),
				tb.PipelineTaskOutputResource("the-magic-resource", "missing-resource")),
		)),
		failureExpected: true,
	}, {
		name: "input resources missing from declaration",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskInputResource("the-resource", "missing-resource"),
				tb.PipelineTaskOutputResource("the-magic-resource", "great-resource")),
		)),
		failureExpected: true,
	}, {
		name: "invalid condition only resource",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskCondition("some-condition",
					tb.PipelineTaskConditionResource("some-workspace", "missing-resource"))),
		)),
		failureExpected: true,
	}, {
		name: "invalid from in condition",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo-task"),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskCondition("some-condition",
					tb.PipelineTaskConditionResource("some-workspace", "missing-resource", "foo"))),
		)),
		failureExpected: true,
	}, {
		name: "from resource isn't output by task",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineDeclaredResource("great-resource", v1alpha1.PipelineResourceTypeGit),
			tb.PipelineDeclaredResource("wonderful-resource", v1alpha1.PipelineResourceTypeImage),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskInputResource("some-workspace", "great-resource")),
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskInputResource("wow-image", "wonderful-resource", tb.From("bar"))),
		)),
		failureExpected: true,
	}, {
		name: "not defined parameter variable",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskParam("a-param", "$(params.does-not-exist)")))),
		failureExpected: true,
	}, {
		name: "not defined parameter variable with defined",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("foo", v1alpha1.ParamTypeString, tb.ParamSpecDefault("something")),
			tb.PipelineTask("foo", "foo-task",
				tb.PipelineTaskParam("a-param", "$(params.foo) and $(params.does-not-exist)")))),
		failureExpected: true,
	}, {
		name: "invalid parameter type",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("baz", "invalidtype", tb.ParamSpecDefault("some", "default")),
			tb.PipelineParamSpec("foo-is-baz", v1alpha1.ParamTypeArray),
			tb.PipelineTask("bar", "bar-task"),
		)),
		failureExpected: true,
	}, {
		name: "array parameter mismatching default type",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("baz", v1alpha1.ParamTypeArray, tb.ParamSpecDefault("astring")),
			tb.PipelineTask("bar", "bar-task"),
		)),
		failureExpected: true,
	}, {
		name: "string parameter mismatching default type",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("baz", v1alpha1.ParamTypeString, tb.ParamSpecDefault("anarray", "elements")),
			tb.PipelineTask("bar", "bar-task"),
		)),
		failureExpected: true,
	}, {
		name: "array parameter used as string",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("baz", v1alpha1.ParamTypeArray, tb.ParamSpecDefault("anarray", "elements")),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskParam("a-param", "$(params.baz)")),
		)),
		failureExpected: true,
	}, {
		name: "array parameter string template not isolated",
		p: tb.Pipeline("pipeline", "namespace", tb.PipelineSpec(
			tb.PipelineParamSpec("baz", v1alpha1.ParamTypeArray, tb.ParamSpecDefault("anarray", "elements")),
			tb.PipelineTask("bar", "bar-task",
				tb.PipelineTaskParam("a-param", "first", "value: $(params.baz)", "last")),
		)),
		failureExpected: true,
	}, {
		name: "invalid dependency graph between the tasks",
		p: tb.Pipeline("foo", "namespace", tb.PipelineSpec(
			tb.PipelineTask("foo", "foo", tb.RunAfter("bar")),
			tb.PipelineTask("bar", "bar", tb.RunAfter("foo")),
		)),
		failureExpected: true,
	}, {
		name: "unused pipeline spec workspaces do not cause an error",
		p: tb.Pipeline("name", "namespace", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("foo"),
			tb.PipelineWorkspaceDeclaration("bar"),
			tb.PipelineTask("foo", "foo"),
		)),
		failureExpected: false,
	}, {
		name: "workspace bindings relying on a non-existent pipeline workspace cause an error",
		p: tb.Pipeline("name", "namespace", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("foo"),
			tb.PipelineTask("taskname", "taskref",
				tb.PipelineTaskWorkspaceBinding("taskWorkspaceName", "pipelineWorkspaceName")),
		)),
		failureExpected: true,
	}, {
		name: "multiple workspaces sharing the same name are not allowed",
		p: tb.Pipeline("name", "namespace", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("foo"),
			tb.PipelineWorkspaceDeclaration("foo"),
		)),
		failureExpected: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate(context.Background())
			if (!tt.failureExpected) && (err != nil) {
				t.Errorf("Pipeline.Validate() returned error: %v", err)
			}

			if tt.failureExpected && (err == nil) {
				t.Error("Pipeline.Validate() did not return error, wanted error")
			}
		})
	}
}
