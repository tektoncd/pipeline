// +build e2e

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

package test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	knativetest "knative.dev/pkg/test"
)

func TestPipelineRunConditionalResource(t *testing.T) {
	c, namespace := setup(t)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	condResource := tb.Condition("file-exists", namespace,
		tb.ConditionSpec(
			tb.ConditionParamSpec("path", v1alpha1.ParamTypeString),
			tb.ConditionResource("workspace", v1alpha1.PipelineResourceTypeGit),
			tb.ConditionSpecCheck("", "alpine", tb.Command("/bin/sh"), tb.Args("-c", "cat $(resources.workspace.path)/$(params.path)| grep \"Cloud Native\"")),
		),
	)

	if _, err := c.ConditionClient.Create(condResource); err != nil {
		t.Fatalf("Failed to create simple repo PipelineResource: %s", err)
	}

	repoResource := tb.PipelineResource("pipeline-git", namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/tektoncd/pipeline"),
		tb.PipelineResourceSpecParam("Revision", "master"),
	))
	if _, err := c.PipelineResourceClient.Create(repoResource); err != nil {
		t.Fatalf("Failed to create simple repo PipelineResource: %s", err)
	}

	conditionalTask := tb.Task("list-files", namespace, tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
		),
		tb.Step("run-ls", "ubuntu", tb.StepCommand("/bin/bash"), tb.StepArgs("-c", "ls -al $(inputs.resources.workspace.path)")),
	))

	if _, err := c.TaskClient.Create(conditionalTask); err != nil {
		t.Fatalf("Failed to create echo Task: %s", err)
	}

	pipeline := tb.Pipeline("list-files-pipeline", namespace, tb.PipelineSpec(
		tb.PipelineDeclaredResource("source-repo", "git"),
		tb.PipelineParamSpec("path", v1alpha1.ParamTypeString, tb.ParamSpecDefault("README.md")),
		tb.PipelineTask("list-files-1", "list-files",
			tb.PipelineTaskCondition("file-exists",
				tb.PipelineTaskConditionParam("path", "$(params.path)"),
				tb.PipelineTaskConditionResource("workspace", "source-repo")),
			tb.PipelineTaskInputResource("workspace", "source-repo"),
		),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create list-files-pipelin: %s", err)
	}
	pipelineRun := tb.PipelineRun("demo-condtional-pr", namespace, tb.PipelineRunSpec("list-files-pipeline",
		tb.PipelineRunServiceAccountName("default"),
		tb.PipelineRunResourceBinding("source-repo", tb.PipelineResourceBindingRef("pipeline-git")),
	))
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create demo-condtional-pr PipelineRun: %s", err)
	}
	t.Logf("Waiting for Conditional pipeline to complete")
	if err := WaitForPipelineRunState(c, "demo-condtional-pr", pipelineRunTimeout, PipelineRunSucceed("demo-condtional-pr"), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %s", err)
	}

}
