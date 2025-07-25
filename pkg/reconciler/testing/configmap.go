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

package testing

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	"sigs.k8s.io/yaml"
)

const (
	apiFieldsFeatureFlag           = "enable-api-fields"
	maxMatrixCombinationsCountFlag = "default-max-matrix-combinations-count"
)

// ConfigMapFromTestFile creates a v1.ConfigMap from a YAML file
// It loads the YAML file from the testdata folder.
func ConfigMapFromTestFile(t *testing.T, name string) *corev1.ConfigMap {
	t.Helper()

	b, err := os.ReadFile(fmt.Sprintf("testdata/%s.yaml", name))
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}

	var cm corev1.ConfigMap

	// Use "sigs.k8s.io/yaml" since it reads json struct
	// tags so things unmarshal properly
	if err := yaml.Unmarshal(b, &cm); err != nil {
		t.Fatalf("yaml.Unmarshal() = %v", err)
	}

	return &cm
}

func NewFeatureFlagsConfigMapInSlice() []*corev1.ConfigMap {
	return []*corev1.ConfigMap{newFeatureFlagsConfigMap()}
}

func newFeatureFlagsConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.GetFeatureFlagsConfigName(),
			Namespace: system.Namespace(),
		},
		Data: make(map[string]string),
	}
}

func NewAlphaFeatureFlagsConfigMapInSlice() []*corev1.ConfigMap {
	return []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
}

func withEnabledAlphaAPIFields(cm *corev1.ConfigMap) *corev1.ConfigMap {
	newCM := cm.DeepCopy()
	newCM.Data[apiFieldsFeatureFlag] = config.AlphaAPIFields
	return newCM
}

func NewFeatureFlagsConfigMapWithMatrixInSlice(count int) []*corev1.ConfigMap {
	return append(
		NewFeatureFlagsConfigMapInSlice(),
		withMaxMatrixCombinationsCount(newDefaultsConfigMap(), count),
	)
}

func withMaxMatrixCombinationsCount(cm *corev1.ConfigMap, count int) *corev1.ConfigMap {
	newCM := cm.DeepCopy()
	newCM.Data[maxMatrixCombinationsCountFlag] = strconv.Itoa(count)
	return newCM
}

func newDefaultsConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.GetDefaultsConfigName(),
			Namespace: system.Namespace(),
		},
		Data: make(map[string]string),
	}
}

func NewAlphaFeatureFlagsConfigMapWithMatrixInSlice(count int) []*corev1.ConfigMap {
	return append(
		NewAlphaFeatureFlagsConfigMapInSlice(),
		withMaxMatrixCombinationsCount(newDefaultsConfigMap(), count),
	)
}

func NewDefaultsCofigMapInSlice() []*corev1.ConfigMap {
	return []*corev1.ConfigMap{newDefaultsConfigMap()}
}
