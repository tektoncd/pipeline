/*
Copyright 2021 The Tekton Authors

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
package deprecated_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/internal/deprecated"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

const (
	featureFlagDisableHomeEnvKey    = "disable-home-env-overwrite"
	featureFlagDisableWorkingDirKey = "disable-working-directory-overwrite"
)

func TestNewOverrideWorkingDirTransformer(t *testing.T) {

	for _, tc := range []struct {
		description string
		configMap   *corev1.ConfigMap
		podspec     corev1.PodSpec
		expected    corev1.PodSpec
	}{{
		description: "Default behaviour: A missing disable-working-directory-overwrite should mean true, so no overwrite",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: "tekton-pipelines"},
			Data:       map[string]string{},
		},
		podspec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
			}, {
				Name:  "sidecar-bar",
				Image: "foo",
			}, {
				Name:       "step-bar-wg",
				Image:      "foo",
				WorkingDir: "/foobar",
			}},
		},
		expected: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
			}, {
				Name:  "sidecar-bar",
				Image: "foo",
			}, {
				Name:       "step-bar-wg",
				Image:      "foo",
				WorkingDir: "/foobar",
			}},
		},
	}, {
		description: "Setting disable-working-directory-overwrite to false should result in we don't disable the behavior, so there should be an overwrite",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: "tekton-pipelines"},
			Data: map[string]string{
				featureFlagDisableWorkingDirKey: "false",
			},
		},
		podspec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
			}, {
				Name:  "sidecar-bar",
				Image: "foo",
			}, {
				Name:       "step-bar-wg",
				Image:      "foo",
				WorkingDir: "/foobar",
			}},
		},
		expected: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:       "step-bar",
				Image:      "foo",
				WorkingDir: "/workspace",
			}, {
				Name:  "sidecar-bar",
				Image: "foo",
			}, {
				Name:       "step-bar-wg",
				Image:      "foo",
				WorkingDir: "/foobar",
			}},
		},
	}, {
		description: "Setting disable-working-directory-overwrite to true should disable the overwrite, so no overwrite",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: "tekton-pipelines"},
			Data: map[string]string{
				featureFlagDisableWorkingDirKey: "true",
			},
		},
		podspec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
			}, {
				Name:  "sidecar-bar",
				Image: "foo",
			}, {
				Name:       "step-bar-wg",
				Image:      "foo",
				WorkingDir: "/foobar",
			}},
		},
		expected: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
			}, {
				Name:  "sidecar-bar",
				Image: "foo",
			}, {
				Name:       "step-bar-wg",
				Image:      "foo",
				WorkingDir: "/foobar",
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			store := config.NewStore(logtesting.TestLogger(t))
			store.OnConfigChanged(tc.configMap)
			ctx := store.ToContext(context.Background())
			f := deprecated.NewOverrideWorkingDirTransformer(ctx)
			got, err := f(&corev1.Pod{Spec: tc.podspec})
			if err != nil {
				t.Fatalf("Transformer failed: %v", err)
			}
			if d := cmp.Diff(tc.expected, got.Spec); d != "" {
				t.Errorf("Diff pod: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestShouldOverrideHomeEnv(t *testing.T) {
	for _, tc := range []struct {
		description string
		configMap   *corev1.ConfigMap
		podspec     corev1.PodSpec
		expected    corev1.PodSpec
	}{{
		description: "Default behaviour: A missing disable-home-env-overwrite flag should result in no overwrite",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: "tekton-pipelines"},
			Data:       map[string]string{},
		},
		podspec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: "/home",
				}},
			}, {
				Name:  "step-baz",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "FOO",
					Value: "bar",
				}},
			}},
		},
		expected: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: "/home",
				}},
			}, {
				Name:  "step-baz",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "FOO",
					Value: "bar",
				}},
			}},
		},
	}, {
		description: "Setting disable-home-env-overwrite to false should result in an overwrite",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: "tekton-pipelines"},
			Data: map[string]string{
				featureFlagDisableHomeEnvKey: "false",
			},
		},
		podspec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: "/home",
				}},
			}, {
				Name:  "step-baz",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "FOO",
					Value: "bar",
				}},
			}},
		},
		expected: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: "/home",
				}},
			}, {
				Name:  "step-baz",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "FOO",
					Value: "bar",
				}, {
					Name:  "HOME",
					Value: "/tekton/home",
				}},
			}},
		},
	}, {
		description: "Setting disable-home-env-overwrite to true should result in no overwrite",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: "tekton-pipelines"},
			Data: map[string]string{
				featureFlagDisableHomeEnvKey: "true",
			},
		},
		podspec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: "/home",
				}},
			}, {
				Name:  "step-baz",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "FOO",
					Value: "bar",
				}},
			}},
		},
		expected: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "step-bar",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: "/home",
				}},
			}, {
				Name:  "step-baz",
				Image: "foo",
				Env: []corev1.EnvVar{{
					Name:  "FOO",
					Value: "bar",
				}},
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			store := config.NewStore(logtesting.TestLogger(t))
			store.OnConfigChanged(tc.configMap)
			ctx := store.ToContext(context.Background())
			f := deprecated.NewOverrideHomeTransformer(ctx)
			got, err := f(&corev1.Pod{Spec: tc.podspec})
			if err != nil {
				t.Fatalf("Transformer failed: %v", err)
			}
			if d := cmp.Diff(tc.expected, got.Spec); d != "" {
				t.Errorf("Diff pod: %s", diff.PrintWantGot(d))
			}
		})
	}
}
