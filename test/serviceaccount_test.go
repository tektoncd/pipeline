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

	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestPipelineRunWithServiceAccounts(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	saPerTask := map[string]string{
		"task1": "sa1",
		"task2": "sa2",
		"task3": "sa3",
	}

	// Create Secrets and Service Accounts
	secrets := []corev1.Secret{{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "secret1",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"foo1": []byte("bar1"),
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "secret2",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"foo2": []byte("bar2"),
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "secret3",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"foo2": []byte("bar3"),
		},
	}}
	serviceAccounts := []corev1.ServiceAccount{{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "sa1",
		},
		Secrets: []corev1.ObjectReference{{
			Name: "secret1",
		}},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "sa2",
		},
		Secrets: []corev1.ObjectReference{{
			Name: "secret2",
		}},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "sa3",
		},
		Secrets: []corev1.ObjectReference{{
			Name: "secret3",
		}},
	}}
	for _, secret := range secrets {
		if _, err := c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, &secret, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create secret `%s`: %s", secret.Name, err)
		}
	}
	for _, serviceAccount := range serviceAccounts {
		if _, err := c.KubeClient.CoreV1().ServiceAccounts(namespace).Create(ctx, &serviceAccount, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create SA `%s`: %s", serviceAccount.Name, err)
		}
	}

	// Create a Pipeline with multiple tasks
	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: task1
    taskSpec:
      steps:
      - image: ubuntu
        script: echo task1
  - name: task2
    taskSpec:
      steps:
      - image: ubuntu
        script: echo task2
  - name: task3
    taskSpec:
      steps:
      - image: ubuntu
        script: echo task3
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}

	// Create a PipelineRun that uses those ServiceAccount
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  taskRunTemplate:
    serviceAccountName: sa1
  taskRunSpecs:
  - pipelineTaskName: task2
    serviceAccountName: sa2
  - pipelineTaskName: task3
    serviceAccountName: sa3
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))

	_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunSucceed(pipelineRun.Name), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	// Verify it used those serviceAccount
	taskRuns, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}
	for _, taskRun := range taskRuns.Items {
		sa := taskRun.Spec.ServiceAccountName
		taskName := taskRun.Labels["tekton.dev/pipelineTask"]
		expectedSA := saPerTask[taskName]
		if sa != expectedSA {
			t.Fatalf("TaskRun %s expected SA %s, got %s", taskRun.Name, expectedSA, sa)
		}
		pods, err := c.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/taskRun=%s", taskRun.Name)})
		if err != nil {
			t.Fatalf("Error listing Pods for TaskRun %s: %s", taskRun.Name, err)
		}
		if len(pods.Items) != 1 {
			t.Fatalf("TaskRun %s should have only 1 pod association, got %+v", taskRun.Name, pods.Items)
		}
		podSA := pods.Items[0].Spec.ServiceAccountName
		if podSA != expectedSA {
			t.Fatalf("TaskRun's pod %s expected SA %s, got %s", pods.Items[0].Name, expectedSA, podSA)
		}
	}
}

func TestPipelineRunWithServiceAccountNameAndTaskRunSpec(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create Secrets and Service Accounts
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "secret1",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"foo1": []byte("bar1"),
		},
	}
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "sa1",
		},
		Secrets: []corev1.ObjectReference{{
			Name: "secret1",
		}},
	}
	if _, err := c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create secret `%s`: %s", secret.Name, err)
	}
	if _, err := c.KubeClient.CoreV1().ServiceAccounts(namespace).Create(ctx, serviceAccount, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create SA `%s`: %s", serviceAccount.Name, err)
	}

	// Create a Pipeline with multiple tasks
	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: task1
    taskSpec:
      metadata: {}
      steps:
      - image: ubuntu
        script: echo task1
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}

	dnsPolicy := corev1.DNSClusterFirst
	// Create a PipelineRun that uses those ServiceAccount
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  taskRunTemplate:
    serviceAccountName: sa1
  taskRunSpecs:
  - pipelineTaskName: task1
    taskPodTemplate:
      dnsPolicy: %s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name, dnsPolicy))

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunSucceed(pipelineRun.Name), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	// Verify it used those serviceAccount
	taskRuns, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}
	for _, taskRun := range taskRuns.Items {
		sa := taskRun.Spec.ServiceAccountName
		expectedSA := "sa1"
		if sa != expectedSA {
			t.Fatalf("TaskRun %s expected SA %s, got %s", taskRun.Name, expectedSA, sa)
		}
		pods, err := c.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/taskRun=%s", taskRun.Name)})
		if err != nil {
			t.Fatalf("Error listing Pods for TaskRun %s: %s", taskRun.Name, err)
		}
		if len(pods.Items) != 1 {
			t.Fatalf("TaskRun %s should have only 1 pod association, got %+v", taskRun.Name, pods.Items)
		}
		podSA := pods.Items[0].Spec.ServiceAccountName
		if podSA != expectedSA {
			t.Fatalf("TaskRun's pod %s expected SA %s, got %s", pods.Items[0].Name, expectedSA, podSA)
		}
	}
}
