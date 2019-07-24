/*
Copyright 2019 The Knative Authors

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

// This file contains functions that construct boilerplate CRD definitions.

package test

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nginxPort  = 80
	nginxName  = "nginx"
	nginxImage = "nginx:1.7.9"
)

// ServiceAccount returns ServiceAccount object in given namespace
func ServiceAccount(name string, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// ClusterRoleBinding returns ClusterRoleBinding for given subject and role
func ClusterRoleBinding(name string, namespace string, serviceAccount string, role string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     role,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

// CoreV1ObjectReference returns a corev1.ObjectReference for the given name, kind and apiversion
func CoreV1ObjectReference(kind, apiversion, name string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:       kind,
		APIVersion: apiversion,
		Name:       name,
	}
}

// NginxPod returns nginx pod defined in given namespace
func NginxPod(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nginxName,
			Namespace:   namespace,
			Annotations: map[string]string{"sidecar.istio.io/inject": "true"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  nginxName,
					Image: nginxImage,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: nginxPort,
						},
					},
				},
			},
		},
	}
}
