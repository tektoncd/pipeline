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
	"testing"

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resources "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestClusterResource(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	secretName := "hw-secret"
	configName := "hw-config"
	resourceName := "helloworld-cluster"
	taskName := "helloworld-cluster-task"
	taskRunName := "helloworld-cluster-taskrun"

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating secret %s", secretName)
	if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(ctx, getClusterResourceTaskSecret(namespace, secretName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Secret `%s`: %s", secretName, err)
	}

	t.Logf("Creating configMap %s", configName)
	if _, err := c.KubeClient.Kube.CoreV1().ConfigMaps(namespace).Create(ctx, getClusterConfigMap(namespace, configName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create configMap `%s`: %s", configName, err)
	}

	t.Logf("Creating cluster PipelineResource %s", resourceName)
	if _, err := c.PipelineResourceClient.Create(ctx, getClusterResource(resourceName, secretName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create cluster Pipeline Resource `%s`: %s", resourceName, err)
	}

	t.Logf("Creating Task %s", taskName)
	if _, err := c.TaskClient.Create(ctx, getClusterResourceTask(namespace, taskName, configName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	t.Logf("Creating TaskRun %s", taskRunName)
	if _, err := c.TaskRunClient.Create(ctx, getClusterResourceTaskRun(namespace, taskRunName, taskName, resourceName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Taskrun `%s`: %s", taskRunName, err)
	}

	// Verify status of TaskRun (wait for it)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", taskRunName, err)
	}
}

func getClusterResource(name, sname string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(name, tb.PipelineResourceSpec(
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

func getClusterResourceTask(namespace, name, configName string) *v1beta1.Task {
	return &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name: "target-cluster",
					Type: resources.PipelineResourceTypeCluster,
				}}},
			},
			Volumes: []corev1.Volume{{
				Name: "config-vol",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configName,
						},
					},
				},
			}},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "check-file-existence",
				Image:   "ubuntu",
				Command: []string{"cat"},
				Args:    []string{"$(resources.inputs.target-cluster.path)/kubeconfig"},
			}}, {Container: corev1.Container{
				Name:    "check-config-data",
				Image:   "ubuntu",
				Command: []string{"cat"},
				Args:    []string{"/config/test.data"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "config-vol",
					MountPath: "/config",
				}},
			}}, {Container: corev1.Container{
				Name:    "check-contents",
				Image:   "ubuntu",
				Command: []string{"bash"},
				Args:    []string{"-c", "cmp -b $(resources.inputs.target-cluster.path)/kubeconfig /config/test.data"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "config-vol",
					MountPath: "/config",
				}},
			}},
			},
		},
	}
}

func getClusterResourceTaskRun(namespace, name, taskName, resName string) *v1beta1.TaskRun {
	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: taskName},
			Resources: &v1beta1.TaskRunResources{
				Inputs: []v1beta1.TaskResourceBinding{{
					PipelineResourceBinding: v1beta1.PipelineResourceBinding{
						Name:        "target-cluster",
						ResourceRef: &v1beta1.PipelineResourceRef{Name: resName},
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
