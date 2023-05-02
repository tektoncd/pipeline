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
	"testing"
	"time"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
	_ "knative.dev/pkg/test/helpers"
)

// TestStepOutput verifies that step output streams can be copied to local files and task results.
func TestStepOutput(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	clients, namespace := setup(ctx, t, requireAnyGate(map[string]string{"enable-api-fields": "alpha"}))

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, clients, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, clients, namespace)

	wantResultName := "step-cat-stdout"
	wantResultValue := "hello world"
	taskRun := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: helpers.ObjectNameForTest(t), Namespace: namespace},
		Spec: v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:  "echo",
					Image: "busybox",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "data",
						MountPath: "/data",
					}},
					Script: fmt.Sprintf("echo -n %s", wantResultValue),
					StdoutConfig: &v1.StepOutputConfig{
						Path: "/data/step-echo-stdout",
					},
				}, {
					Name:  "cat",
					Image: "busybox",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "data",
						MountPath: "/data",
					}},
					Script: "cat /data/step-echo-stdout",
					StdoutConfig: &v1.StepOutputConfig{
						Path: fmt.Sprintf("$(results.%s.path)", wantResultName),
					},
				}},
				Volumes: []corev1.Volume{{
					Name: "data",
				}},
				Results: []v1.TaskResult{{
					Name: wantResultName,
				}},
			},
		},
	}

	t.Logf("Creating TaskRun %q in namespace %q", taskRun.Name, taskRun.Namespace)
	if _, err := clients.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun %q: %s", taskRun.Name, err)
	}

	t.Logf("Waiting for TaskRun %q to finish", taskRun.Name)
	if err := WaitForTaskRunState(ctx, clients, taskRun.Name, Succeed(taskRun.Name), "TaskRunSucceed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun %q to finish: %v", taskRun.Name, err)
	}

	tr, err := clients.V1TaskRunClient.Get(ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting Taskrun %q: %v", taskRun.Name, err)
	}
	var found bool
	for _, result := range tr.Status.Results {
		if result.Name == wantResultName {
			found = true
			if got, want := result.Value.StringVal, wantResultValue; got != want {
				t.Errorf("Result %s: got %q, wanted %q", wantResultName, got, want)
			}
		}
	}
	if !found {
		t.Errorf("Result %s not found", wantResultName)
	}
}

// TestStepOutputWithWorkspace verifies that step output streams can be copied to local files and task results
// when a workspace is defined for the task.
func TestStepOutputWithWorkspace(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	clients, namespace := setup(ctx, t, requireAnyGate(map[string]string{"enable-api-fields": "alpha"}))

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, clients, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, clients, namespace)

	wantResultName := "step-cat-stdout"
	wantResultValue := "hello world"
	taskRun := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: helpers.ObjectNameForTest(t), Namespace: namespace},
		Spec: v1.TaskRunSpec{
			Workspaces: []v1.WorkspaceBinding{{
				Name:     "data",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}},
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:   "echo",
					Image:  "busybox",
					Script: fmt.Sprintf("echo -n %s", wantResultValue),
					StdoutConfig: &v1.StepOutputConfig{
						Path: "/data/step-echo-stdout",
					},
				}, {
					Name:   "cat",
					Image:  "busybox",
					Script: "cat /data/step-echo-stdout",
					StdoutConfig: &v1.StepOutputConfig{
						Path: fmt.Sprintf("$(results.%s.path)", wantResultName),
					},
				}},
				Workspaces: []v1.WorkspaceDeclaration{{
					Name:      "data",
					MountPath: "/data",
				}},
				Results: []v1.TaskResult{{
					Name: wantResultName,
				}},
			},
		},
	}

	t.Logf("Creating TaskRun %q in namespace %q", taskRun.Name, taskRun.Namespace)
	if _, err := clients.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun %q: %s", taskRun.Name, err)
	}

	t.Logf("Waiting for TaskRun %q to finish", taskRun.Name)
	if err := WaitForTaskRunState(ctx, clients, taskRun.Name, Succeed(taskRun.Name), "TaskRunSucceed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun %q to finish: %v", taskRun.Name, err)
	}

	tr, err := clients.V1TaskRunClient.Get(ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting Taskrun %q: %v", taskRun.Name, err)
	}
	var found bool
	for _, result := range tr.Status.Results {
		if result.Name == wantResultName {
			found = true
			if got, want := result.Value.StringVal, wantResultValue; got != want {
				t.Errorf("Result %s: got %q, wanted %q", wantResultName, got, want)
			}
		}
	}
	if !found {
		t.Errorf("Result %s not found", wantResultName)
	}
}
