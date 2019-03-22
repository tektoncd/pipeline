// +build e2e

/*
Copyright 2018 Knative Authors LLC
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
	"fmt"
	"testing"

	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

const (
	embedTaskName    = "helloworld"
	embedTaskRunName = "helloworld-run"

	// TODO(#127) Currently not reliable to retrieve this output
	taskOutput = "do you want to build a snowman"
)

func getEmbeddedTask(namespace string, args []string) *v1alpha1.Task {
	return tb.Task(embedTaskName, namespace,
		tb.TaskSpec(
			tb.TaskInputs(tb.InputsResource("docs", v1alpha1.PipelineResourceTypeGit)),
			tb.Step("read", "ubuntu",
				tb.Command("/bin/bash"),
				tb.Args("-c", "cat /workspace/docs/LICENSE"),
			),
			tb.Step("helloworld-busybox", "busybox", tb.Command(args...)),
		))
}

func getEmbeddedTaskRun(namespace string) *v1alpha1.TaskRun {
	testSpec := &v1alpha1.PipelineResourceSpec{
		Type: v1alpha1.PipelineResourceTypeGit,
		Params: []v1alpha1.Param{{
			Name:  "URL",
			Value: "https://github.com/knative/docs",
		}},
	}
	return tb.TaskRun(embedTaskRunName, namespace,
		tb.TaskRunSpec(
			tb.TaskRunInputs(
				tb.TaskRunInputsResource("docs", tb.TaskResourceBindingResourceSpec(testSpec)),
			),
			tb.TaskRunTaskRef(embedTaskName)))
}

// TestTaskRun_EmbeddedResource is an integration test that will verify a very simple "hello world" TaskRun can be
// executed with an embedded resource spec.
func TestTaskRun_EmbeddedResource(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	if _, err := c.TaskClient.Create(getEmbeddedTask(namespace, []string{"/bin/sh", "-c", fmt.Sprintf("echo %s", taskOutput)})); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", embedTaskName, err)
	}
	if _, err := c.TaskRunClient.Create(getEmbeddedTaskRun(namespace)); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", embedTaskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", embedTaskRunName, namespace)
	if err := WaitForTaskRunState(c, embedTaskRunName, TaskRunSucceed(embedTaskRunName), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", embedTaskRunName, err)
	}

	// TODO(#127) Currently we have no reliable access to logs from the TaskRun so we'll assume successful
	// completion of the TaskRun means the TaskRun did what it was intended.
}
