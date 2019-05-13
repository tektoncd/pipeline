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
	"testing"

	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterResource(t *testing.T) {
	secretName := "hw-secret"
	configName := "hw-config"
	resourceName := "helloworld-cluster"
	taskName := "helloworld-cluster-task"
	taskRunName := "helloworld-cluster-taskrun"

	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating secret %s", secretName)
	if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(getClusterResourceTaskSecret(namespace, secretName)); err != nil {
		t.Fatalf("Failed to create Secret `%s`: %s", secretName, err)
	}

	t.Logf("Creating configMap %s", configName)
	if _, err := c.KubeClient.Kube.CoreV1().ConfigMaps(namespace).Create(getClusterConfigMap(namespace, configName)); err != nil {
		t.Fatalf("Failed to create configMap `%s`: %s", configName, err)
	}

	t.Logf("Creating cluster PipelineResource %s", resourceName)
	if _, err := c.PipelineResourceClient.Create(getClusterResource(namespace, resourceName, secretName)); err != nil {
		t.Fatalf("Failed to create cluster Pipeline Resource `%s`: %s", resourceName, err)
	}

	t.Logf("Creating Task %s", taskName)
	if _, err := c.TaskClient.Create(getClusterResourceTask(namespace, taskName, resourceName, configName)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	t.Logf("Creating TaskRun %s", taskRunName)
	if _, err := c.TaskRunClient.Create(getClusterResourceTaskRun(namespace, taskRunName, taskName, resourceName)); err != nil {
		t.Fatalf("Failed to create Taskrun `%s`: %s", taskRunName, err)
	}

	// Verify status of TaskRun (wait for it)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", taskRunName, err)
	}
}

func getClusterResource(namespace, name, sname string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(name, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeCluster,
		tb.PipelineResourceSpecParam("Name", "helloworld-cluster"),
		tb.PipelineResourceSpecParam("Url", "https://1.1.1.1"),
		tb.PipelineResourceSpecParam("username", "test-user"),
		tb.PipelineResourceSpecParam("password", "test-password"),
		tb.PipelineResourceSpecSecretParam("cadata", sname, "cadatakey"),
		tb.PipelineResourceSpecSecretParam("token", sname, "tokenkey"),
	))
}

func getClusterResourceTaskSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"cadatakey": []byte("Y2EtY2VydAo="), //ca-cert
			"tokenkey":  []byte("dG9rZW4K"),     //token
		},
	}
}

func getClusterResourceTask(namespace, name, resName, configName string) *v1alpha1.Task {
	return tb.Task(name, namespace, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource("target-cluster", v1alpha1.PipelineResourceTypeCluster)),
		tb.TaskVolume("config-vol", tb.VolumeSource(corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configName,
				},
			},
		})),
		tb.Step("check-file-existence", "ubuntu",
			tb.Command("cat"), tb.Args("/workspace/helloworld-cluster/kubeconfig"),
		),
		tb.Step("check-config-data", "ubuntu", tb.Command("cat"), tb.Args("/config/test.data"),
			tb.VolumeMount("config-vol", "/config"),
		),
		tb.Step("check-contents", "ubuntu",
			tb.Command("bash"), tb.Args("-c", "cmp -b /workspace/helloworld-cluster/kubeconfig /config/test.data"),
			tb.VolumeMount("config-vol", "/config"),
		),
	))
}

func getClusterResourceTaskRun(namespace, name, taskName, resName string) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name: taskName,
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					Name: "target-cluster",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: resName,
					},
				}},
			},
		},
	}
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
  name: helloworld-cluster
contexts:
- context:
    cluster: helloworld-cluster
    user: test-user
  name: helloworld-cluster
current-context: helloworld-cluster
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
