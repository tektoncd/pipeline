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
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func withRegistry(t *testing.T, c *clients, namespace string) {
	if _, err := c.KubeClient.Kube.AppsV1().Deployments(namespace).Create(getRegistryDeployment(namespace)); err != nil {
		t.Fatalf("Failed to create the local registry deployment: %v", err)
	}
	service := getRegistryService(namespace)
	if _, err := c.KubeClient.Kube.CoreV1().Services(namespace).Create(service); err != nil {
		t.Fatalf("Failed to create the local registry service: %v", err)
	}
	set := labels.Set(service.Spec.Selector)

	// Give it a little bit of time to at least create the pod
	time.Sleep(5 * time.Second)

	if pods, err := c.KubeClient.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: set.AsSelector().String()}); err != nil {
		t.Fatalf("Failed to list Pods of service[%s] error:%v", service.GetName(), err)
	} else {
		if len(pods.Items) != 1 {
			t.Fatalf("Only 1 pod for service %s should be running: %v", service, pods.Items)
		}

		if err := WaitForPodState(c, pods.Items[0].Name, namespace, func(pod *corev1.Pod) (bool, error) {
			return pod.Status.Phase == "Running", nil
		}, "PodContainersRunning"); err != nil {
			t.Fatalf("Error waiting for Pod %q to run: %v", pods.Items[0].Name, err)
		}
	}
}

func getRegistryDeployment(namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "registry",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "registry",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "registry",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "registry",
						Image: "registry",
					}},
				},
			},
		},
	}
}

func getRegistryService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "registry",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port: 5000,
			}},
			Selector: map[string]string{
				"app": "registry",
			},
		},
	}
}
