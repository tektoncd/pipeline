/*
Copyright 2018 The Knative Authors
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
	"fmt"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/knative/build-pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

func ExampleTask() {
	// You can declare re-usable modifiers
	myStep := tb.Step("my-step", "myimage")
	// … and use them in a Task definition
	myTask := tb.Task("my-task", "namespace", tb.TaskSpec(
		tb.Step("simple-step", "myotherimage", tb.Command("/mycmd")),
		myStep,
	))
	// … and another one.
	myOtherTask := tb.Task("my-other-task", "namespace",
		tb.TaskSpec(myStep,
			tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit)),
		),
	)
	fmt.Println("Task", myTask, myOtherTask)
}

func ExampleClusterTask() {
	myClusterTask := tb.ClusterTask("my-task", tb.ClusterTaskSpec(
		tb.Step("simple-step", "myotherimage", tb.Command("/mycmd")),
	))
	fmt.Println("ClusterTask", myClusterTask)
}

func ExampleTaskRun() {
	// A simple definition, with a Task reference
	myTaskRun := tb.TaskRun("my-taskrun", "namespace", tb.TaskRunSpec(
		tb.TaskRunTaskRef("my-task"),
	))
	// … or a more complex one with inline TaskSpec
	myTaskRunWithSpec := tb.TaskRun("my-taskrun-with-spec", "namespace", tb.TaskRunSpec(
		tb.TaskRunInputs(
			tb.TaskRunInputsParam("myarg", "foo"),
			tb.TaskRunInputsResource("workspace", tb.ResourceBindingRef("git-resource", "a1")),
		),
		tb.TaskRunTaskSpec(
			tb.TaskInputs(
				tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
				tb.InputsParam("myarg", tb.ParamDefault("mydefault")),
			),
			tb.Step("mycontainer", "myimage", tb.Command("/mycmd"),
				tb.Args("--my-arg=${inputs.params.myarg}"),
			),
		),
	))
	fmt.Println("TaskRun", myTaskRun, myTaskRunWithSpec)
}

func ExamplePipeline() {
	pipeline := tb.Pipeline("tomatoes", "namespace",
		tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
	)
	fmt.Println("Pipeline", pipeline)
}

func ExamplePipelineRun() {
	pipelineRun := tb.PipelineRun("pear", "namespace",
		tb.PipelineRunSpec("tomatoes", tb.PipelineRunServiceAccount("inexistent")),
	)
	fmt.Println("PipelineRun", pipelineRun)
}

func ExamplePipelineResource() {
	gitResource := tb.PipelineResource("git-resource", "namespace", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit, tb.PipelineSpecParam("URL", "https://foo.git"),
	))
	anotherGitResource := tb.PipelineResource("another-git-resource", "namespace", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit, tb.PipelineSpecParam("URL", "https://foobar.git"),
	))
	imageResource := tb.PipelineResource("image-resource", "namespace", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeImage, tb.PipelineSpecParam("URL", "gcr.io/kristoff/sven"),
	))
	fmt.Println("PipelineResource", gitResource, anotherGitResource, imageResource)
}

func ExampleBuildSpec() {
	toolsMount := corev1.VolumeMount{
		Name:      "tools-volume",
		MountPath: "/tools",
	}
	volume := corev1.Volume{
		Name:         "tools-volume",
		VolumeSource: corev1.VolumeSource{
			// …
		},
	}
	buildSpec := tb.BuildSpec(
		tb.BuildStep("simple-step", "foo", tb.Command("/myentrypoint"),
			tb.VolumeMount(toolsMount),
		),
		tb.BuildVolume(volume),
	)
	fmt.Println("BuildSpec", buildSpec)
}
