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
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/credentials"
	"github.com/tektoncd/pipeline/pkg/credentials/dockercreds"
	"github.com/tektoncd/pipeline/pkg/credentials/gitcreds"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	credsInitHomeMountPrefix = "tekton-creds-init-home"
	sshKnownHosts            = "known_hosts"
)

// credsInit reads secrets available to the given service account and
// searches for annotations matching a specific format (documented in
// docs/auth.md). Matching secrets are turned into Volumes for the Pod
// and VolumeMounts to be given to each Step. Additionally, a list of
// entrypointer arguments are returned, each with a meaning specific to
// the credential type it describes: git credentials expect one set of
// args while docker credentials expect another.
//
// Any errors encountered during this process are returned to the
// caller. If no matching annotated secrets are found, nil lists with a
// nil error are returned.
func credsInit(ctx context.Context, serviceAccountName, namespace string, kubeclient kubernetes.Interface) ([]string, []corev1.Volume, []corev1.VolumeMount, error) {
	// service account if not specified in pipeline/task spec, read it from the ConfigMap
	// and defaults to `default` if its missing from the ConfigMap as well
	if serviceAccountName == "" {
		serviceAccountName = config.DefaultServiceAccountValue
	}

	sa, err := kubeclient.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	builders := []credentials.Builder{dockercreds.NewBuilder(), gitcreds.NewBuilder()}

	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume
	args := []string{}
	for _, secretEntry := range sa.Secrets {
		secret, err := kubeclient.CoreV1().Secrets(namespace).Get(ctx, secretEntry.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, nil, err
		}

		if err := checkGitSSHSecret(ctx, secret); err != nil {
			return nil, nil, nil, err
		}

		matched := false
		for _, b := range builders {
			if sa := b.MatchingAnnotations(secret); len(sa) > 0 {
				matched = true
				args = append(args, sa...)
			}
		}

		if matched {
			name := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("tekton-internal-secret-volume-%s", secret.Name))
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
		return nil, nil, nil, nil
	}

	return args, volumes, volumeMounts, nil
}

// getCredsInitVolume returns a Volume and VolumeMount for /tekton/creds. Each call
// will return a new volume and volume mount with randomized name.
func getCredsInitVolume() (corev1.Volume, corev1.VolumeMount) {
	name := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(credsInitHomeMountPrefix)
	v := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: corev1.StorageMediumMemory,
		}},
	}
	vm := corev1.VolumeMount{
		Name:      name,
		MountPath: pipeline.CredsDir,
	}
	return v, vm
}

// checkGitSSHSecret requires `known_host` field must be included in Git SSH Secret when feature flag
// `require-git-ssh-secret-known-hosts` is true.
func checkGitSSHSecret(ctx context.Context, secret *corev1.Secret) error {
	cfg := config.FromContextOrDefaults(ctx)

	if secret.Type == corev1.SecretTypeSSHAuth && cfg.FeatureFlags.RequireGitSSHSecretKnownHosts {
		if _, ok := secret.Data[sshKnownHosts]; !ok {
			return fmt.Errorf("TaskRun validation failed. Git SSH Secret must have \"known_hosts\" included " +
				"when feature flag \"require-git-ssh-secret-known-hosts\" is set to true")
		}
	}
	return nil
}
