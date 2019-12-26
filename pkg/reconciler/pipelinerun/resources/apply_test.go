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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestApplyParameters(t *testing.T) {
	tests := []struct {
		name     string
		original *v1alpha1.Pipeline
		run      *v1alpha1.PipelineRun
		expected *v1alpha1.Pipeline
	}{{
		name: "single parameter",
		original: tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1alpha1.ParamTypeString),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(params.first-param)"),
					tb.PipelineTaskParam("first-task-second-param", "$(params.second-param)"),
					tb.PipelineTaskParam("first-task-third-param", "static value"),
				))),
		run: tb.PipelineRun("test-pipeline-run", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunParam("second-param", "second-value"))),
		expected: tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1alpha1.ParamTypeString),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "default-value"),
					tb.PipelineTaskParam("first-task-second-param", "second-value"),
					tb.PipelineTaskParam("first-task-third-param", "static value"),
				))),
	}, {
		name: "pipeline parameter nested inside task parameter",
		original: tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(input.workspace.$(params.first-param))"),
					tb.PipelineTaskParam("first-task-second-param", "$(input.workspace.$(params.second-param))"),
				))),
		run: tb.PipelineRun("test-pipeline-run", "foo",
			tb.PipelineRunSpec("test-pipeline")),
		expected: tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "$(input.workspace.default-value)"),
					tb.PipelineTaskParam("first-task-second-param", "$(input.workspace.default-value)"),
				))),
	}, {
		name: "parameters in task condition",
		original: tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1alpha1.ParamTypeString),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskCondition("task-condition",
						tb.PipelineTaskConditionParam("cond-first-param", "$(params.first-param)"),
						tb.PipelineTaskConditionParam("cond-second-param", "$(params.second-param)"),
					),
				))),
		run: tb.PipelineRun("test-pipeline-run", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunParam("second-param", "second-value"))),
		expected: tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value")),
				tb.PipelineParamSpec("second-param", v1alpha1.ParamTypeString),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskCondition("task-condition",
						tb.PipelineTaskConditionParam("cond-first-param", "default-value"),
						tb.PipelineTaskConditionParam("cond-second-param", "second-value"),
					),
				))),
	}, {
		name: "array parameter",
		original: tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeArray, tb.ParamSpecDefault(
					"default", "array", "value")),
				tb.PipelineParamSpec("second-param", v1alpha1.ParamTypeArray),
				tb.PipelineParamSpec("fourth-param", v1alpha1.ParamTypeArray),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "firstelement", "$(params.first-param)"),
					tb.PipelineTaskParam("first-task-second-param", "first", "$(params.second-param)"),
					tb.PipelineTaskParam("first-task-third-param", "static value"),
					tb.PipelineTaskParam("first-task-fourth-param", "first", "$(params.fourth-param)"),
				))),
		run: tb.PipelineRun("test-pipeline-run", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunParam("second-param", "second-value", "array"),
				tb.PipelineRunParam("fourth-param", "fourth-value", "array"))),
		expected: tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeArray, tb.ParamSpecDefault(
					"default", "array", "value")),
				tb.PipelineParamSpec("second-param", v1alpha1.ParamTypeArray),
				tb.PipelineParamSpec("fourth-param", v1alpha1.ParamTypeArray),
				tb.PipelineTask("first-task-1", "first-task",
					tb.PipelineTaskParam("first-task-first-param", "firstelement", "default", "array", "value"),
					tb.PipelineTaskParam("first-task-second-param", "first", "second-value", "array"),
					tb.PipelineTaskParam("first-task-third-param", "static value"),
					tb.PipelineTaskParam("first-task-fourth-param", "first", "fourth-value", "array"),
				))),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyParameters(&tt.original.Spec, tt.run)
			if d := cmp.Diff(got, &tt.expected.Spec); d != "" {
				t.Errorf("ApplyParameters() got diff %s", d)
			}
		})
	}
}
