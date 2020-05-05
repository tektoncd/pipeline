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

package builder_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
)

// This is a "hack" to make the example "look" like tests
var t *testing.T

func ExampleTask() {
	// You can declare re-usable modifiers
	myStep := tb.Step("myimage")
	// … and use them in a Task definition
	myTask := tb.Task("my-task", tb.TaskSpec(
		tb.Step("myotherimage", tb.StepCommand("/mycmd")),
		myStep,
	))
	// … and another one.
	myOtherTask := tb.Task("my-other-task",
		tb.TaskSpec(myStep,
			tb.TaskResources(tb.TaskResourcesInput("workspace", resource.PipelineResourceTypeGit)),
		),
	)
	expectedTask := &v1beta1.Task{
		// […]
	}
	expectedOtherTask := &v1beta1.Task{
		// […]
	}
	// […]
	if d := cmp.Diff(expectedTask, myTask); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
	if d := cmp.Diff(expectedOtherTask, myOtherTask); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}

func ExampleClusterTask() {
	myClusterTask := tb.ClusterTask("my-task", tb.ClusterTaskSpec(
		tb.Step("myotherimage", tb.StepCommand("/mycmd")),
	))
	expectedClusterTask := &v1beta1.Task{
		// […]
	}
	// […]
	if d := cmp.Diff(expectedClusterTask, myClusterTask); d != "" {
		t.Fatalf("ClusterTask diff -want, +got: %v", d)
	}
}

func ExampleTaskRun() {
	// A simple definition, with a Task reference
	myTaskRun := tb.TaskRun("my-taskrun", tb.TaskRunSpec(
		tb.TaskRunTaskRef("my-task"),
	))
	// … or a more complex one with inline TaskSpec
	myTaskRunWithSpec := tb.TaskRun("my-taskrun-with-spec", tb.TaskRunSpec(
		tb.TaskRunParam("myarg", "foo"),
		tb.TaskRunResources(
			tb.TaskRunResourcesInput("workspace", tb.TaskResourceBindingRef("git-resource")),
		),
		tb.TaskRunTaskSpec(
			tb.TaskResources(tb.TaskResourcesInput("workspace", resource.PipelineResourceTypeGit)),
			tb.TaskParam("myarg", v1beta1.ParamTypeString, tb.ParamSpecDefault("mydefault")),
			tb.Step("myimage", tb.StepCommand("/mycmd"),
				tb.StepArgs("--my-arg=$(inputs.params.myarg)"),
			),
		),
	))
	expectedTaskRun := &v1beta1.TaskRun{
		// […]
	}
	expectedTaskRunWithSpec := &v1beta1.TaskRun{
		// […]
	}
	// […]
	if d := cmp.Diff(expectedTaskRun, myTaskRun); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
	if d := cmp.Diff(expectedTaskRunWithSpec, myTaskRunWithSpec); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}

func ExamplePipeline() {
	pipeline := tb.Pipeline("tomatoes",
		tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
	)
	expectedPipeline := &v1beta1.Pipeline{
		// […]
	}
	// […]
	if d := cmp.Diff(expectedPipeline, pipeline); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}

func ExamplePipelineRun() {
	pipelineRun := tb.PipelineRun("pear",
		tb.PipelineRunSpec("tomatoes", tb.PipelineRunServiceAccountName("inexistent")),
	)
	expectedPipelineRun := &v1beta1.PipelineRun{
		// […]
	}
	// […]
	if d := cmp.Diff(expectedPipelineRun, pipelineRun); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}

func ExamplePipelineResource() {
	gitResource := tb.PipelineResource("git-resource", tb.PipelineResourceSpec(
		v1beta1.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foo.git"),
	))
	imageResource := tb.PipelineResource("image-resource", tb.PipelineResourceSpec(
		v1beta1.PipelineResourceTypeImage, tb.PipelineResourceSpecParam("URL", "gcr.io/kristoff/sven"),
	))
	expectedGitResource := resource.PipelineResource{
		// […]
	}
	expectedImageResource := resource.PipelineResource{
		// […]
	}
	// […]
	if d := cmp.Diff(expectedGitResource, gitResource); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
	if d := cmp.Diff(expectedImageResource, imageResource); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}
