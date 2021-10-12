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

	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/parse"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestClusterResource(t *testing.T) {
	secretName := "hw-secret"
	configName := "hw-config"
	resourceName := "helloworld-cluster"
	taskName := "helloworld-cluster-task"
	taskRunName := "helloworld-cluster-taskrun"

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating secret %s", secretName)
	if _, err := c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, getClusterResourceTaskSecret(namespace, secretName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Secret `%s`: %s", secretName, err)
	}

	t.Logf("Creating configMap %s", configName)
	if _, err := c.KubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, getClusterConfigMap(namespace, configName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create configMap `%s`: %s", configName, err)
	}

	t.Logf("Creating cluster PipelineResource %s", resourceName)
	if _, err := c.PipelineResourceClient.Create(ctx, getClusterResource(t, namespace, resourceName, secretName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create cluster Pipeline Resource `%s`: %s", resourceName, err)
	}

	t.Logf("Creating Task %s", taskName)
	if _, err := c.TaskClient.Create(ctx, getClusterResourceTask(t, taskName, configName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	t.Logf("Creating TaskRun %s", taskRunName)
	if _, err := c.TaskRunClient.Create(ctx, getClusterResourceTaskRun(t, namespace, taskRunName, taskName, resourceName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Taskrun `%s`: %s", taskRunName, err)
	}

	// Verify status of TaskRun (wait for it)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", taskRunName, err)
	}
}

func getClusterResource(t *testing.T, namespace, name, sname string) *resourcev1alpha1.PipelineResource {
	return parse.MustParsePipelineResource(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  type: cluster
  params:
  - name: Name
    value: helloworld-cluster
  - name: Url
    value: https://1.1.1.1
  - name: username
    value: test-user
  - name: password
    value: test-password
  secrets:
  - fieldName: cadata
    secretKey: cadatakey
    secretName: %s
  - fieldName: token
    secretKey: tokenkey
    secretName: %s
`, name, namespace, sname, sname))
}

func getClusterResourceTaskSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"cadatakey": []byte("Y2EtY2VydAo="), // ca-cert
			"tokenkey":  []byte("dG9rZW4K"),     // token
		},
	}
}

func getClusterResourceTask(t *testing.T, name, configName string) *v1alpha1.Task {
	return parse.MustParseAlphaTask(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  inputs:
    resources:
    - name: target-cluster
      type: cluster
  volumes:
  - name: config-vol
    configMap:
      name: %s
  steps:
  - name: check-file-existence
    image: ubuntu
    command: ['cat']
    args: ['/workspace/target-cluster/kubeconfig']
  - name: check-config-data
    image: ubuntu
    command: ['cat']
    args: ['/config/test.data']
    volumeMounts:
    - name: config-vol
      mountPath: /config
  - name: check-contents
    image: ubuntu
    command: ['bash']
    args: ['-c', 'cmp -b /workspace/target-cluster/kubeconfig /config/test.data']
    volumeMounts:
    - name: config-vol
      mountPath: /config
`, name, configName))
}

func getClusterResourceTaskRun(t *testing.T, namespace, name, taskName, resName string) *v1alpha1.TaskRun {
	return parse.MustParseAlphaTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
  inputs:
    resources:
    - name: target-cluster
      resourceRef:
        name: %s
`, name, namespace, taskName, resName))
}

func getClusterConfigMap(namespace, name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: map[string]string{
			"test.data": `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: WTJFdFkyVnlkQW89
    server: https://1.1.1.1
  name: target-cluster
contexts:
- context:
    cluster: target-cluster
    user: test-user
  name: target-cluster
current-context: target-cluster
kind: Config
preferences: {}
users:
- name: test-user
  user:
    token: dG9rZW4K
`,
		},
	}
}
