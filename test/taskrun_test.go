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

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"

	// Mysteriously by k8s libs, or they fail to create `KubeClient`s from config. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	hwTaskName    = "helloworld"
	hwTaskRunName = "helloworld-run"
)

func getHelloWorldTask(namespace string) *v1alpha1.Task {
	return &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			BuildSpec: buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{
					corev1.Container{
						Name:  "helloworld-busybox",
						Image: "busybox",
						Args: []string{
							"echo", "hello world",
						},
					},
				},
			},
		},
	}
}

func getHelloWorldTaskRun(namespace string) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwTaskRunName,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: hwTaskName,
			},
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
		},
	}
}

// TestTaskRun is an integration test that will verify a very simple "hello world" TaskRun can be
// executed.
func TestTaskRun(t *testing.T) {
	t.Skip("Will fail until #59 is completed :D")

	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(logger, c.KubeClient, namespace) }, logger)
	defer tearDown(logger, c.KubeClient, namespace)

	// Create task
	if _, err := c.TaskClient.Create(getHelloWorldTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", hwTaskName, err)
	}

	// Create TaskRun
	if _, err := c.TaskRunClient.Create(getHelloWorldTaskRun(namespace)); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", hwTaskRunName, err)
	}

	// Verify status of TaskRun (wait for it)
	if err := WaitForTaskRunState(c, hwTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		if len(tr.Status.Conditions) > 0 {
			// TODO: use actual conditions
			return true, nil
		}
		return false, nil
	}, "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", hwTaskRunName, err)
	}

	// The Build created by the TaskRun will have the same name
	b, err := c.BuildClient.Get(hwTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Expected there to be a Build with the same name as TaskRun %s but got error: %s", hwTaskRunName, err)
	}
	cluster := b.Status.Cluster
	if cluster == nil || cluster.PodName == "" {
		t.Fatalf("Expected build status to have a podname but it didn't!")
	}
	podName := cluster.PodName
	pods := c.KubeClient.Kube.CoreV1().Pods(namespace)
	fmt.Printf("Retrieved pods for podname %s: %s\n", podName, pods)
	// TODO(#59): Verify logs from the pod, should output hello world
}
