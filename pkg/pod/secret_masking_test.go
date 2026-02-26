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

package pod

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"sort"
	"strings"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

func TestCollectSecretsForMasking(t *testing.T) {
	ctx := context.Background()
	optional := true

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("supersecretpassword"),
			"token":    []byte("mytoken123"),
			"short":    []byte("ab"),
		},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-key": []byte("apikey456"),
		},
	}

	kubeclient := fakek8s.NewSimpleClientset(secret1, secret2)

	testCases := []struct {
		name            string
		steps           []v1.Step
		volumes         []corev1.Volume
		taskSpecVolumes []corev1.Volume
		expected        []string
		expectErr       bool
	}{
		{
			name: "env secretKeyRef",
			steps: []v1.Step{{
				Name: "step1",
				Env: []corev1.EnvVar{{
					Name: "MY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
							Key:                  "password",
						},
					},
				}},
			}},
			expected: []string{"supersecretpassword"},
		},
		{
			name: "envFrom secretRef",
			steps: []v1.Step{{
				Name: "step1",
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
					},
				}},
			}},
			expected: []string{"supersecretpassword", "mytoken123"},
		},
		{
			name:  "secret volume",
			steps: []v1.Step{{Name: "step1"}},
			volumes: []corev1.Volume{{
				Name: "secret-vol",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: "other-secret"},
				},
			}},
			expected: []string{"apikey456"},
		},
		{
			name:  "taskSpec secret volume",
			steps: []v1.Step{{Name: "step1"}},
			taskSpecVolumes: []corev1.Volume{{
				Name: "secret-vol",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: "other-secret"},
				},
			}},
			expected: []string{"apikey456"},
		},
		{
			name: "multiple sources",
			steps: []v1.Step{{
				Name: "step1",
				Env: []corev1.EnvVar{{
					Name: "MY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
							Key:                  "password",
						},
					},
				}},
			}},
			volumes: []corev1.Volume{{
				Name: "secret-vol",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: "other-secret"},
				},
			}},
			expected: []string{"supersecretpassword", "apikey456"},
		},
		{
			name: "missing secret",
			steps: []v1.Step{{
				Name: "step1",
				Env: []corev1.EnvVar{{
					Name: "MY_VAR",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "missing"},
							Key:                  "key",
						},
					},
				}},
			}},
			expectErr: true,
		},
		{
			name: "missing key",
			steps: []v1.Step{{
				Name: "step1",
				Env: []corev1.EnvVar{{
					Name: "MY_VAR",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
							Key:                  "missing-key",
						},
					},
				}},
			}},
			expectErr: true,
		},
		{
			name: "optional missing secret",
			steps: []v1.Step{{
				Name: "step1",
				Env: []corev1.EnvVar{{
					Name: "MY_VAR",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "missing"},
							Key:                  "key",
							Optional:             &optional,
						},
					},
				}},
			}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskSpec := &v1.TaskSpec{Volumes: tc.taskSpecVolumes}
			result, err := CollectSecretsForMasking(ctx, kubeclient, "default", taskSpec, tc.steps, tc.volumes)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := decodeSecretLines(t, result)
			if !sameStrings(got, tc.expected) {
				t.Fatalf("got %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestSecretMaskInitContainer(t *testing.T) {
	secretData, err := encodeSecretMaskData("c2VjcmV0MQ==\ncGFzc3dvcmQ=")
	if err != nil {
		t.Fatalf("unexpected error encoding secret mask data: %v", err)
	}
	securityContext := SecurityContextConfig{
		SetSecurityContext:        true,
		SetReadOnlyRootFilesystem: false,
	}

	container := secretMaskInitContainer("entrypoint:latest", secretData, securityContext, false)

	if container.Name != "secret-mask-init" {
		t.Errorf("expected name 'secret-mask-init', got %q", container.Name)
	}
	if container.Image != "entrypoint:latest" {
		t.Errorf("expected image 'entrypoint:latest', got %q", container.Image)
	}
	if len(container.Command) != 3 {
		t.Fatalf("expected 3 command args, got %d: %v", len(container.Command), container.Command)
	}
	if container.Command[0] != "/ko-app/entrypoint" {
		t.Errorf("expected command[0] to be '/ko-app/entrypoint', got %q", container.Command[0])
	}
	if container.Command[1] != "secret-mask-init" {
		t.Errorf("expected command[1] to be 'secret-mask-init', got %q", container.Command[1])
	}
	if container.Command[2] != SecretMaskFilePath() {
		t.Errorf("expected command[2] to be %q, got %q", SecretMaskFilePath(), container.Command[2])
	}
	if len(container.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(container.Env))
	}
	if container.Env[0].Name != SecretMaskDataEnvVar {
		t.Errorf("expected env var name %q, got %q", SecretMaskDataEnvVar, container.Env[0].Name)
	}
	if container.Env[0].Value != secretData {
		t.Errorf("expected env var value to be the secret data")
	}
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != SecretMaskVolumeName {
		t.Errorf("expected volume name %q, got %q", SecretMaskVolumeName, container.VolumeMounts[0].Name)
	}
	if container.VolumeMounts[0].ReadOnly {
		t.Error("init container volume mount should be writable")
	}
	if container.SecurityContext == nil {
		t.Error("expected security context to be set")
	}
}

func TestEncodeSecretMaskData(t *testing.T) {
	original := "c2VjcmV0MQ==\ncGFzc3dvcmQ="
	encoded, err := encodeSecretMaskData(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed base64 decode: %v", err)
	}
	gzr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	decoded, err := io.ReadAll(gzr)
	if err != nil {
		t.Fatalf("failed to read gzip stream: %v", err)
	}
	if err := gzr.Close(); err != nil {
		t.Fatalf("failed to close gzip reader: %v", err)
	}

	if got := string(decoded); got != original {
		t.Fatalf("got %q, want %q", got, original)
	}
}

func decodeSecretLines(t *testing.T, result string) []string {
	t.Helper()
	if result == "" {
		return nil
	}
	lines := strings.Split(result, "\n")
	secrets := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(line)
		if err != nil {
			t.Fatalf("failed to decode line %q: %v", line, err)
		}
		secrets = append(secrets, string(decoded))
	}
	return secrets
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
