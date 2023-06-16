//go:build e2e
// +build e2e

/*
Copyright 2021 The Tekton Authors

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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestHermeticTaskRun make sure that the hermetic execution mode actually drops network from a TaskRun step
// it does this by first running the TaskRun normally to make sure it passes
// Then, it enables hermetic mode and makes sure the same TaskRun fails because it no longer has access to a network.
func TestHermeticTaskRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{"enable-api-fields": "alpha"}))
	t.Parallel()
	defer tearDown(ctx, t, c, namespace)

	tests := []struct {
		desc       string
		getTaskRun func(*testing.T, string, string, string) *v1.TaskRun
	}{
		{
			desc:       "run-as-root",
			getTaskRun: taskRun,
		}, {
			desc:       "run-as-nonroot",
			getTaskRun: unpriviligedTaskRun,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// first, run the task run with hermetic=false to prove that it succeeds
			regularTaskRunName := fmt.Sprintf("not-hermetic-%s", test.desc)
			regularTaskRun := test.getTaskRun(t, regularTaskRunName, namespace, "")
			t.Logf("Creating TaskRun %s, hermetic=false", regularTaskRunName)
			if _, err := c.V1TaskRunClient.Create(ctx, regularTaskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun `%s`: %s", regularTaskRunName, err)
			}
			if err := WaitForTaskRunState(ctx, c, regularTaskRunName, Succeed(regularTaskRunName), "TaskRunCompleted", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun %s to finish: %s", regularTaskRunName, err)
			}

			// now, run the task mode with hermetic mode
			// it should fail, since it shouldn't be able to access any network
			hermeticTaskRunName := fmt.Sprintf("hermetic-should-fail-%s", test.desc)
			hermeticTaskRun := test.getTaskRun(t, hermeticTaskRunName, namespace, "hermetic")
			t.Logf("Creating TaskRun %s, hermetic=true", hermeticTaskRunName)
			if _, err := c.V1TaskRunClient.Create(ctx, hermeticTaskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun `%s`: %s", regularTaskRun.Name, err)
			}
			if err := WaitForTaskRunState(ctx, c, hermeticTaskRunName, Failed(hermeticTaskRunName), "Failed", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun %s to fail: %s", hermeticTaskRunName, err)
			}
		})
	}
}

func taskRun(t *testing.T, name, namespace, executionMode string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  annotations:
    experimental.tekton.dev/execution-mode: %s
  name: %s
  namespace: %s
spec:
  taskSpec:
    steps:
    - image: gcr.io/cloud-builders/curl
      name: access-network
      
      script: |-
        #!/bin/bash
        set -ex

        # do something that requires network access
        curl google.com
  timeout: 1m0s
`, executionMode, name, namespace))
}

func unpriviligedTaskRun(t *testing.T, name, namespace, executionMode string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  annotations:
    experimental.tekton.dev/execution-mode: %s
  name: %s
  namespace: %s
spec:
  taskSpec:
    steps:
    - image: gcr.io/cloud-builders/curl
      name: curl
      
      script: |-
        #!/bin/bash
        set -ex
        curl google.com
      securityContext:
        allowPrivilegeEscalation: false
        runAsNonRoot: true
        runAsUser: 1000
  timeout: 1m0s
`, executionMode, name, namespace))
}
