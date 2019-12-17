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

package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

const (
	serviceAccountName = "my-service-account"
	namespace          = "namespacey-mcnamespace"
)

func TestCredsInit(t *testing.T) {
	volumeMounts := []corev1.VolumeMount{{
		Name: "implicit-volume-mount",
	}}
	envVars := []corev1.EnvVar{{
		Name:  "FOO",
		Value: "bar",
	}}

	for _, c := range []struct {
		desc string
		want *corev1.Container
		objs []runtime.Object
	}{{
		desc: "service account exists with no secrets; nothing to initialize",
		objs: []runtime.Object{
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace}},
		},
		want: nil,
	}, {
		desc: "service account has no annotated secrets; nothing to initialize",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "my-creds",
				}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "my-creds",
					Namespace:   namespace,
					Annotations: map[string]string{
						// No matching annotations.
					},
				},
			},
		},
		want: nil,
	}, {
		desc: "service account has annotated secret; initialize creds",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "my-creds",
				}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-creds",
					Namespace: namespace,
					Annotations: map[string]string{
						"tekton.dev/docker-0": "https://us.gcr.io",
						"tekton.dev/docker-1": "https://docker.io",
						"tekton.dev/git-0":    "github.com",
						"tekton.dev/git-1":    "gitlab.com",
					},
				},
				Type: "kubernetes.io/basic-auth",
				Data: map[string][]byte{
					"username": []byte("foo"),
					"password": []byte("BestEver"),
				},
			},
		},
		want: &corev1.Container{
			Name:    "credential-initializer",
			Image:   images.CredsImage,
			Command: []string{"/ko-app/creds-init"},
			Args: []string{
				"-basic-docker=my-creds=https://docker.io",
				"-basic-docker=my-creds=https://us.gcr.io",
				"-basic-git=my-creds=github.com",
				"-basic-git=my-creds=gitlab.com",
			},
			Env: envVars,
			VolumeMounts: append(volumeMounts, corev1.VolumeMount{
				Name:      "tekton-internal-secret-volume-my-creds",
				MountPath: "/tekton/creds-secrets/my-creds",
			}),
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			kubeclient := fakek8s.NewSimpleClientset(c.objs...)
			got, volumes, err := credsInit(images.CredsImage, serviceAccountName, namespace, kubeclient, volumeMounts, envVars)
			if err != nil {
				t.Fatalf("credsInit: %v", err)
			}
			if got == nil && len(volumes) > 0 {
				t.Errorf("Got nil creds-init container, with non-empty volumes: %v", volumes)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("Diff(-want, +got): %s", d)
			}
		})
	}
}
