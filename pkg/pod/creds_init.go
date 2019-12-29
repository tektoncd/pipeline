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
	"fmt"

	"github.com/tektoncd/pipeline/pkg/credentials"
	"github.com/tektoncd/pipeline/pkg/credentials/dockercreds"
	"github.com/tektoncd/pipeline/pkg/credentials/gitcreds"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// credsInit returns an init container that initializes credentials based on
// annotated secrets available to the service account.
//
// If no such secrets are found, it returns a nil container, and no creds init
// process is necessary.
//
// If it finds secrets, it also returns a set of Volumes to attach to the Pod
// to provide those secrets to this initialization.
func credsInit(credsImage string, serviceAccountName, namespace string, kubeclient kubernetes.Interface, volumeMounts []corev1.VolumeMount, implicitEnvVars []corev1.EnvVar) (*corev1.Container, []corev1.Volume, error) {
	if serviceAccountName == "" {
		serviceAccountName = "default"
	}

	sa, err := kubeclient.CoreV1().ServiceAccounts(namespace).Get(serviceAccountName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	builders := []credentials.Builder{dockercreds.NewBuilder(), gitcreds.NewBuilder()}

	var volumes []corev1.Volume
	args := []string{}
	for _, secretEntry := range sa.Secrets {
		secret, err := kubeclient.CoreV1().Secrets(namespace).Get(secretEntry.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}

		matched := false
		for _, b := range builders {
			if sa := b.MatchingAnnotations(secret); len(sa) > 0 {
				matched = true
				args = append(args, sa...)
			}
		}

		if matched {
			name := names.SimpleNameGenerator.RestrictLength(fmt.Sprintf("tekton-internal-secret-volume-%s", secret.Name))
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      name,
				MountPath: credentials.VolumeName(secret.Name),
			})
			volumes = append(volumes, corev1.Volume{
				Name: name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.Name,
					},
				},
			})
		}
	}

	if len(args) == 0 {
		// There are no creds to initialize.
		return nil, nil, nil
	}

	return &corev1.Container{
		Name:         "credential-initializer",
		Image:        credsImage,
		Command:      []string{"/ko-app/creds-init"},
		Args:         args,
		Env:          implicitEnvVars,
		VolumeMounts: volumeMounts,
	}, volumes, nil
}
