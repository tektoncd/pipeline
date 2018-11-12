/*
Copyright 2018 The Knative Authors.

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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/system"
)

func TestEntrypointConfiguration(t *testing.T) {
	testImage := "gcr.io/k8s-prow/entrypoint@sha256:123"
	configTests := []struct {
		name           string
		wantErr        bool
		wantEntrypoint interface{}
		config         *corev1.ConfigMap
	}{{
		name: "entrypoint configuration normal",
		wantEntrypoint: &Entrypoint{
			Image: testImage,
		},
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace,
				Name:      EntrypointConfigName,
			},
			Data: map[string]string{
				ImageKey: testImage,
			},
		}}, {
		name: "entrypoint with no image",
		wantEntrypoint: &Entrypoint{
			Image: DefaultEntrypointImage,
		},
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace,
				Name:      EntrypointConfigName,
			},
			Data: map[string]string{},
		}},
		{
			name: "entrypoint with no data",
			wantEntrypoint: &Entrypoint{
				Image: DefaultEntrypointImage,
			},
			config: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: system.Namespace,
					Name:      EntrypointConfigName,
				},
			}},
	}

	for _, tt := range configTests {
		actualEntrypoint, _ := NewEntrypointConfigFromConfigMap(tt.config)

		if diff := cmp.Diff(actualEntrypoint, tt.wantEntrypoint); diff != "" {
			t.Fatalf("Test: %q; want %v, but got %v", tt.name, tt.wantEntrypoint, actualEntrypoint)
		}
	}
}
