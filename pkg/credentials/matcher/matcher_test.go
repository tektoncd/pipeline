/*
Copyright 2025 The Tekton Authors

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

package matcher_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/pipeline/pkg/credentials/common"
	"github.com/tektoncd/pipeline/pkg/credentials/matcher"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestGetSecretType(t *testing.T) {
	tests := []struct {
		name   string
		secret *corev1.Secret
		want   string
	}{{
		name: "basic auth secret",
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-secret",
				Annotations: map[string]string{
					"tekton.dev/git-0": "https://github.com",
				},
			},
			Type: corev1.SecretType(common.SecretTypeBasicAuth),
		},
		want: common.SecretTypeBasicAuth,
	}, {
		name: "docker config json secret",
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-secret",
				Annotations: map[string]string{
					"tekton.dev/docker-0": "https://gcr.io",
				},
			},
			Type: corev1.SecretType(common.SecretTypeDockerConfigJson),
		},
		want: common.SecretTypeDockerConfigJson,
	}, {
		name: "ssh auth secret",
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-secret",
				Annotations: map[string]string{
					"tekton.dev/git-0": "https://github.com",
				},
			},
			Type: corev1.SecretType(common.SecretTypeSSHAuth),
		},
		want: common.SecretTypeSSHAuth,
	}, {
		name: "opaque secret",
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-secret",
				Annotations: map[string]string{
					"tekton.dev/git-0": "https://github.com",
				},
			},
			Type: corev1.SecretType(common.SecretTypeOpaque),
		},
		want: common.SecretTypeOpaque,
	}, {
		name:   "nil secret",
		secret: nil,
		want:   "",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matcher.GetSecretType(tc.secret)
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
