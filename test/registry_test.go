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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func withRegistry(t *testing.T, c *clients, namespace string) {
	deployment := getRegistryDeployment(namespace)
	if _, err := c.KubeClient.Kube.AppsV1().Deployments(namespace).Create(deployment); err != nil {
		t.Fatalf("Failed to create the local registry deployment: %v", err)
	}
	if err := WaitForDeploymentState(c, deployment.Name, namespace, func(d *appsv1.Deployment) (bool, error) {
		var replicas int32 = 1
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		return d.Status.ReadyReplicas == replicas, nil
	}, "DeploymentPodRunning"); err != nil {
		t.Fatalf("Error waiting for Deployment %q to be ready: %v", deployment.Name, err)
	}

	service := getRegistryService(namespace)
	if _, err := c.KubeClient.Kube.CoreV1().Services(namespace).Create(service); err != nil {
		t.Fatalf("Failed to create the local registry service: %v", err)
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
