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

package container_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/container"
	corev1 "k8s.io/api/core/v1"
)

func TestApplyStepReplacements(t *testing.T) {
	replacements := map[string]string{
		"replace.me":           "replaced!",
		"workspaces.data.path": "/workspace/data",
	}

	arrayReplacements := map[string][]string{
		"array.replace.me": {"val1", "val2"},
	}

	s := v1.Step{
		Script:     "$(replace.me)",
		Name:       "$(replace.me)",
		Image:      "$(replace.me)",
		Command:    []string{"$(array.replace.me)"},
		Args:       []string{"$(array.replace.me)"},
		WorkingDir: "$(replace.me)",
		OnError:    "$(replace.me)",
		EnvFrom: []corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "$(replace.me)",
				},
			},
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "$(replace.me)",
				},
			},
		}},
		Env: []corev1.EnvVar{{
			Name:  "not_me",
			Value: "$(replace.me)",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(replace.me)",
					},
					Key: "$(replace.me)",
				},
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(replace.me)",
					},
					Key: "$(replace.me)",
				},
			},
		}},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "$(replace.me)",
			MountPath: "$(replace.me)",
			SubPath:   "$(replace.me)",
		}},
		StdoutConfig: &v1.StepOutputConfig{
			Path: "$(workspaces.data.path)/stdout.txt",
		},
		StderrConfig: &v1.StepOutputConfig{
			Path: "$(workspaces.data.path)/stderr.txt",
		},
	}

	expected := v1.Step{
		Script:     "replaced!",
		Name:       "replaced!",
		Image:      "replaced!",
		Command:    []string{"val1", "val2"},
		Args:       []string{"val1", "val2"},
		WorkingDir: "replaced!",
		OnError:    "replaced!",
		EnvFrom: []corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "replaced!",
				},
			},
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "replaced!",
				},
			},
		}},
		Env: []corev1.EnvVar{{
			Name:  "not_me",
			Value: "replaced!",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "replaced!",
					},
					Key: "replaced!",
				},
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "replaced!",
					},
					Key: "replaced!",
				},
			},
		}},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "replaced!",
			MountPath: "replaced!",
			SubPath:   "replaced!",
		}},
		StdoutConfig: &v1.StepOutputConfig{
			Path: "/workspace/data/stdout.txt",
		},
		StderrConfig: &v1.StepOutputConfig{
			Path: "/workspace/data/stderr.txt",
		},
	}
	container.ApplyStepReplacements(&s, replacements, arrayReplacements)
	if d := cmp.Diff(s, expected); d != "" {
		t.Errorf("Container replacements failed: %s", d)
	}
}
