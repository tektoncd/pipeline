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
	"strings"
	"testing"

	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

const (
	wdTaskName    = "wd-task"
	wdTaskRunName = "wd-task-run"
)

func TestWorkingDirCreated(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	task := tb.Task(wdTaskName, namespace, tb.TaskSpec(
		tb.Step("ubuntu", tb.StepWorkingDir("/workspace/HELLOMOTO"), tb.StepArgs("-c", "echo YES")),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	t.Logf("Creating TaskRun  namespace %s", namespace)
	taskRun := tb.TaskRun(wdTaskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef(wdTaskName), tb.TaskRunServiceAccountName("default"),
	))
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish successfully", namespace)
	if err := WaitForTaskRunState(c, wdTaskRunName, TaskRunSucceed(wdTaskRunName), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish successfully: %s", err)
	}

	tr, err := c.TaskRunClient.Get(wdTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}
	if tr.Status.PodName == "" {
		t.Fatal("Error getting a PodName (empty)")
	}
	p, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(tr.Status.PodName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
	}
	for _, stat := range p.Status.ContainerStatuses {
		if strings.HasPrefix(stat.Name, "working-dir-initializer") {
			if stat.State.Terminated != nil {
				req := c.KubeClient.Kube.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
				logContent, err := req.Do().Raw()
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
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	task := tb.Task(wdTaskName, namespace, tb.TaskSpec(
		tb.Step("ubuntu", tb.StepWorkingDir("/HELLOMOTO"), tb.StepArgs("-c", "echo YES")),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	t.Logf("Creating TaskRun  namespace %s", namespace)
	taskRun := tb.TaskRun(wdTaskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef(wdTaskName), tb.TaskRunServiceAccountName("default"),
	))
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to finish successfully", namespace)
	if err := WaitForTaskRunState(c, wdTaskRunName, TaskRunSucceed(wdTaskRunName), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish successfully: %s", err)
	}

	tr, err := c.TaskRunClient.Get(wdTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}

	p, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(tr.Status.PodName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
	}
	for _, stat := range p.Status.ContainerStatuses {
		if strings.HasPrefix(stat.Name, "working-dir-initializer") {
			t.Logf("Found a working dir container called `%s` in `%s`  when it should have been excluded:", stat.Name, tr.Status.PodName)
		}
	}

}
