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
	"testing"

	knativetest "github.com/knative/pkg/test"

	tb "github.com/tektoncd/pipeline/test/builder"
)

const (
	epTaskName    = "ep-task"
	epTaskRunName = "ep-task-run"
)

// TestEntrypointRunningStepsInOrder is an integration test that will
// verify attempt to the get the entrypoint of a container image
// that doesn't have a cmd defined. In addition to making sure the steps
// are executed in the order specified
func TestEntrypointRunningStepsInOrder(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := tb.Task(epTaskName, namespace, tb.TaskSpec(
		tb.Step("step1", "ubuntu", tb.Args("-c", "sleep 3 && touch foo")),
		tb.Step("step2", "ubuntu", tb.Args("-c", "ls", "foo")),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := tb.TaskRun(epTaskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef(epTaskName), tb.TaskRunServiceAccount("default"),
	))
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish successfully", namespace)
	if err := WaitForTaskRunState(c, epTaskRunName, TaskRunSucceed(epTaskRunName), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish successfully: %s", err)
	}

}
