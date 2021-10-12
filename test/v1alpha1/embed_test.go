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
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/test/parse"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

const (
	embedTaskName    = "helloworld"
	embedTaskRunName = "helloworld-run"

	// TODO(#127) Currently not reliable to retrieve this output
	taskOutput = "do you want to build a snowman"
)

// TestTaskRun_EmbeddedResource is an integration test that will verify a very simple "hello world" TaskRun can be
// executed with an embedded resource spec.
func TestTaskRun_EmbeddedResource(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	if _, err := c.TaskClient.Create(ctx, getEmbeddedTask(t, []string{"/bin/sh", "-c", fmt.Sprintf("echo %s", taskOutput)}), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", embedTaskName, err)
	}
	if _, err := c.TaskRunClient.Create(ctx, getEmbeddedTaskRun(t, namespace), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", embedTaskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", embedTaskRunName, namespace)
	if err := WaitForTaskRunState(ctx, c, embedTaskRunName, TaskRunSucceed(embedTaskRunName), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", embedTaskRunName, err)
	}

	// TODO(#127) Currently we have no reliable access to logs from the TaskRun so we'll assume successful
	// completion of the TaskRun means the TaskRun did what it was intended.
}

func getEmbeddedTask(t *testing.T, args []string) *v1alpha1.Task {
	var argsForYaml []string
	for _, s := range args {
		argsForYaml = append(argsForYaml, fmt.Sprintf("'%s'", s))
	}
	return parse.MustParseAlphaTask(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  inputs:
    resources:
    - name: docs
      type: git
  steps:
  - image: ubuntu
    command: ['/bin/bash']
    args: ['-c', 'cat /workspace/docs/LICENSE']
  - image: busybox
    command: %s
`, embedTaskName, fmt.Sprintf("[%s]", strings.Join(argsForYaml, ", "))))
}

func getEmbeddedTaskRun(t *testing.T, namespace string) *v1alpha1.TaskRun {
	return parse.MustParseAlphaTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  inputs:
    resources:
    - name: docs
      resourceSpec:
        type: git
        params:
        - name: URL
          value: https://github.com/knative/docs
  taskRef:
    name: %s
`, embedTaskRunName, namespace, embedTaskName))
}
