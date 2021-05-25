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
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
		getTaskRun func(string, string, string) *v1beta1.TaskRun
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
			regularTaskRun := test.getTaskRun(regularTaskRunName, namespace, "")
			t.Logf("Creating TaskRun %s, hermetic=false", regularTaskRunName)
			if _, err := c.TaskRunClient.Create(ctx, regularTaskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun `%s`: %s", regularTaskRunName, err)
			}
			if err := WaitForTaskRunState(ctx, c, regularTaskRunName, Succeed(regularTaskRunName), "TaskRunCompleted"); err != nil {
				t.Fatalf("Error waiting for TaskRun %s to finish: %s", regularTaskRunName, err)
			}

			// now, run the task mode with hermetic mode
			// it should fail, since it shouldn't be able to access any network
			hermeticTaskRunName := fmt.Sprintf("hermetic-should-fail-%s", test.desc)
			hermeticTaskRun := test.getTaskRun(hermeticTaskRunName, namespace, "hermetic")
			t.Logf("Creating TaskRun %s, hermetic=true", hermeticTaskRunName)
			if _, err := c.TaskRunClient.Create(ctx, hermeticTaskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun `%s`: %s", regularTaskRun.Name, err)
			}
			if err := WaitForTaskRunState(ctx, c, hermeticTaskRunName, Failed(hermeticTaskRunName), "Failed"); err != nil {
				t.Fatalf("Error waiting for TaskRun %s to fail: %s", hermeticTaskRunName, err)
			}
		})
	}
}

func taskRun(name, namespace, executionMode string) *v1beta1.TaskRun {
	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: name,
			Namespace: namespace,
			Annotations: map[string]string{
				"experimental.tekton.dev/execution-mode": executionMode,
			},
		},
		Spec: v1beta1.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: time.Minute},
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{
					{
						Container: corev1.Container{
							Name:  "access-network",
							Image: "ubuntu",
						},
						Script: `#!/bin/bash
set -ex
apt-get update
apt-get install -y curl`,
					},
				},
			},
		},
	}
}

func unpriviligedTaskRun(name, namespace, executionMode string) *v1beta1.TaskRun {
	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: name,
			Namespace: namespace,
			Annotations: map[string]string{
				"experimental.tekton.dev/execution-mode": executionMode,
			},
		},
		Spec: v1beta1.TaskRunSpec{
			Timeout: &metav1.Duration{Duration: time.Minute},
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{
					{
						Container: corev1.Container{
							Name:  "curl",
							Image: "gcr.io/cloud-builders/curl",
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                pointer.Int64Ptr(1000),
								RunAsNonRoot:             pointer.BoolPtr(true),
								AllowPrivilegeEscalation: pointer.BoolPtr(false),
							},
						},
						Script: `#!/bin/bash
set -ex
curl google.com`,
					},
				},
			},
		},
	}
}
