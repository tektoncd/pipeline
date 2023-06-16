//go:build e2e
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestWorkingDirCreated(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	wdTaskName := helpers.ObjectNameForTest(t)
	wdTaskRunName := helpers.ObjectNameForTest(t)

	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: ubuntu
    workingDir: /workspace/HELLOMOTO
    args: ['-c', 'echo YES']
`, wdTaskName, namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	t.Logf("Creating TaskRun  namespace %s", namespace)
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
  serviceAccountName: default
`, wdTaskRunName, namespace, wdTaskName))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish successfully", namespace)
	if err := WaitForTaskRunState(ctx, c, wdTaskRunName, TaskRunSucceed(wdTaskRunName), "TaskRunSuccess", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish successfully: %s", err)
	}

	tr, err := c.V1TaskRunClient.Get(ctx, wdTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}
	if tr.Status.PodName == "" {
		t.Fatal("Error getting a PodName (empty)")
	}
	p, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, tr.Status.PodName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
	}
	for _, stat := range p.Status.ContainerStatuses {
		if strings.HasPrefix(stat.Name, "working-dir-initializer") {
			if stat.State.Terminated != nil {
				req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
				logContent, err := req.Do(ctx).Raw()
				if err != nil {
					t.Fatalf("Error getting pod logs for pod `%s` and container `%s` in namespace `%s`", tr.Status.PodName, stat.Name, namespace)
				}
				if string(logContent) != "" {
					t.Logf("Found some content in workingdir pod: %s, `%s` when it should be empty", tr.Status.PodName, logContent)
				}
			}
		}
	}
}

func TestWorkingDirIgnoredNonSlashWorkspace(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	wdTaskName := helpers.ObjectNameForTest(t)
	wdTaskRunName := helpers.ObjectNameForTest(t)

	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: ubuntu
    workingDir: /HELLOMOTO
    args: ['-c', 'echo YES']
`, wdTaskName, namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	t.Logf("Creating TaskRun  namespace %s", namespace)
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
  serviceAccountName: default
`, wdTaskRunName, namespace, wdTaskName))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish successfully", namespace)
	if err := WaitForTaskRunState(ctx, c, wdTaskRunName, TaskRunSucceed(wdTaskRunName), "TaskRunSuccess", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish successfully: %s", err)
	}

	tr, err := c.V1TaskRunClient.Get(ctx, wdTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}

	p, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, tr.Status.PodName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
	}
	for _, stat := range p.Status.ContainerStatuses {
		if strings.HasPrefix(stat.Name, "working-dir-initializer") {
			t.Logf("Found a working dir container called `%s` in `%s`  when it should have been excluded:", stat.Name, tr.Status.PodName)
		}
	}
}
