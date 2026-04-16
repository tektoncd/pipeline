/*
Copyright 2026 The Tekton Authors

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
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
)

func TestCollectSecretNames(t *testing.T) {
	tests := []struct {
		name     string
		taskSpec v1.TaskSpec
		want     []string
	}{
		{
			name:     "no secrets referenced",
			taskSpec: v1.TaskSpec{Steps: []v1.Step{{Image: "alpine"}}},
			want:     nil,
		},
		{
			name: "secret in step env via secretKeyRef",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{
					Image: "alpine",
					Env: []corev1.EnvVar{{
						Name: "PASSWORD",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "db-creds"},
								Key:                  "password",
							},
						},
					}},
				}},
			},
			want: []string{"db-creds"},
		},
		{
			name: "secret in step envFrom",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{
					Image: "alpine",
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "app-secrets"},
						},
					}},
				}},
			},
			want: []string{"app-secrets"},
		},
		{
			name: "secret in stepTemplate env",
			taskSpec: v1.TaskSpec{
				StepTemplate: &v1.StepTemplate{
					Env: []corev1.EnvVar{{
						Name: "TOKEN",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "auth-token"},
								Key:                  "token",
							},
						},
					}},
				},
				Steps: []v1.Step{{Image: "alpine"}},
			},
			want: []string{"auth-token"},
		},
		{
			name: "secret in stepTemplate envFrom",
			taskSpec: v1.TaskSpec{
				StepTemplate: &v1.StepTemplate{
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "shared-secrets"},
						},
					}},
				},
				Steps: []v1.Step{{Image: "alpine"}},
			},
			want: []string{"shared-secrets"},
		},
		{
			name: "secret in sidecar env",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{Image: "alpine"}},
				Sidecars: []v1.Sidecar{{
					Env: []corev1.EnvVar{{
						Name: "PROXY_KEY",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "proxy-creds"},
								Key:                  "key",
							},
						},
					}},
				}},
			},
			want: []string{"proxy-creds"},
		},
		{
			name: "secret in sidecar envFrom",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{Image: "alpine"}},
				Sidecars: []v1.Sidecar{{
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "sidecar-secrets"},
						},
					}},
				}},
			},
			want: []string{"sidecar-secrets"},
		},
		{
			name: "secret in task-level volume",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{Image: "alpine"}},
				Volumes: []corev1.Volume{{
					Name: "tls-cert",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "tls-secret",
						},
					},
				}},
			},
			want: []string{"tls-secret"},
		},
		{
			name: "deduplicate same secret from multiple locations",
			taskSpec: v1.TaskSpec{
				StepTemplate: &v1.StepTemplate{
					Env: []corev1.EnvVar{{
						Name: "A",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "shared"},
								Key:                  "a",
							},
						},
					}},
				},
				Steps: []v1.Step{{
					Image: "alpine",
					Env: []corev1.EnvVar{{
						Name: "B",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "shared"},
								Key:                  "b",
							},
						},
					}},
				}},
				Volumes: []corev1.Volume{{
					Name: "vol",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: "shared"},
					},
				}},
			},
			want: []string{"shared"},
		},
		{
			name: "multiple distinct secrets across all locations",
			taskSpec: v1.TaskSpec{
				StepTemplate: &v1.StepTemplate{
					Env: []corev1.EnvVar{{
						Name: "T",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "template-secret"},
								Key:                  "val",
							},
						},
					}},
				},
				Steps: []v1.Step{{
					Image: "alpine",
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "step-secret"},
						},
					}},
				}},
				Sidecars: []v1.Sidecar{{
					Env: []corev1.EnvVar{{
						Name: "S",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "sidecar-secret"},
								Key:                  "val",
							},
						},
					}},
				}},
				Volumes: []corev1.Volume{{
					Name: "vol",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: "volume-secret"},
					},
				}},
			},
			want: []string{"template-secret", "step-secret", "sidecar-secret", "volume-secret"},
		},
		{
			name: "env var without secretKeyRef is ignored",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{
					Image: "alpine",
					Env: []corev1.EnvVar{
						{Name: "PLAIN", Value: "hello"},
						{Name: "FROM_CM", ValueFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
								Key:                  "key",
							},
						}},
					},
				}},
			},
			want: nil,
		},
		{
			name: "non-secret volume is ignored",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{Image: "alpine"}},
				Volumes: []corev1.Volume{{
					Name: "cm-vol",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
						},
					},
				}},
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := collectSecretNames(tc.taskSpec)
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("collectSecretNames() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestSecretMaskingVolumes(t *testing.T) {
	tests := []struct {
		name          string
		taskSpec      v1.TaskSpec
		wantVolCount  int
		wantMountDirs []string
	}{
		{
			name:          "no secrets produces no volumes",
			taskSpec:      v1.TaskSpec{Steps: []v1.Step{{Image: "alpine"}}},
			wantVolCount:  0,
			wantMountDirs: nil,
		},
		{
			name: "one secret produces one volume and mount",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{
					Image: "alpine",
					Env: []corev1.EnvVar{{
						Name: "P",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
								Key:                  "pass",
							},
						},
					}},
				}},
			},
			wantVolCount:  1,
			wantMountDirs: []string{filepath.Join(pipeline.SecretMaskDir, "my-secret")},
		},
		{
			name: "two secrets produce two volumes and mounts",
			taskSpec: v1.TaskSpec{
				Steps: []v1.Step{{
					Image: "alpine",
					Env: []corev1.EnvVar{
						{
							Name: "A",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "secret-a"},
									Key:                  "val",
								},
							},
						},
						{
							Name: "B",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "secret-b"},
									Key:                  "val",
								},
							},
						},
					},
				}},
			},
			wantVolCount: 2,
			wantMountDirs: []string{
				filepath.Join(pipeline.SecretMaskDir, "secret-a"),
				filepath.Join(pipeline.SecretMaskDir, "secret-b"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vols, mounts := secretMaskingVolumes(tc.taskSpec)
			if len(vols) != tc.wantVolCount {
				t.Errorf("expected %d volumes, got %d", tc.wantVolCount, len(vols))
			}
			if len(mounts) != tc.wantVolCount {
				t.Errorf("expected %d mounts, got %d", tc.wantVolCount, len(mounts))
			}

			for i, wantDir := range tc.wantMountDirs {
				if mounts[i].MountPath != wantDir {
					t.Errorf("mount[%d] path = %q, want %q", i, mounts[i].MountPath, wantDir)
				}
				if !mounts[i].ReadOnly {
					t.Errorf("mount[%d] should be read-only", i)
				}
			}

			for i, vol := range vols {
				if vol.VolumeSource.Secret == nil {
					t.Fatalf("volume[%d] has no secret source", i)
				}
				if vol.VolumeSource.Secret.Optional == nil || !*vol.VolumeSource.Secret.Optional {
					t.Errorf("volume[%d] should be optional", i)
				}
			}
		})
	}
}
