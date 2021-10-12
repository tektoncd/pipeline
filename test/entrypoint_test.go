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
	"context"
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/test/parse"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

const epTaskRunName = "ep-task-run"

// TestEntrypointRunningStepsInOrder is an integration test that will
// verify attempt to the get the entrypoint of a container image
// that doesn't have a cmd defined. In addition to making sure the steps
// are executed in the order specified
func TestEntrypointRunningStepsInOrder(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating TaskRun in namespace %s", namespace)
	if _, err := c.TaskRunClient.Create(ctx, parse.MustParseTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    steps:
    - image: busybox
      workingDir: /workspace
      script: 'sleep 3 && touch foo'
    - image: ubuntu
      workingDir: /workspace
      script: 'ls foo'
`, epTaskRunName, namespace)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish successfully", namespace)
	if err := WaitForTaskRunState(ctx, c, epTaskRunName, TaskRunSucceed(epTaskRunName), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish successfully: %s", err)
	}

}
