/*
 Copyright 2022 The Tekton Authors

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

package framework_test

import (
	"testing"

	framework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	corev1 "k8s.io/api/core/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

// TestDataFromConfigMap checks that configmaps are correctly converted
// into a map[string]string
func TestDataFromConfigMap(t *testing.T) {
	for _, tc := range []struct {
		configMap *corev1.ConfigMap
		expected  map[string]string
	}{{
		configMap: nil,
		expected:  map[string]string{},
	}, {
		configMap: &corev1.ConfigMap{
			Data: nil,
		},
		expected: map[string]string{},
	}, {
		configMap: &corev1.ConfigMap{
			Data: map[string]string{},
		},
		expected: map[string]string{},
	}, {
		configMap: &corev1.ConfigMap{
			Data: map[string]string{
				"foo": "bar",
			},
		},
		expected: map[string]string{
			"foo": "bar",
		},
	}} {
		out, err := framework.DataFromConfigMap(tc.configMap)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !mapsAreEqual(tc.expected, out) {
			t.Fatalf("expected %#v received %#v", tc.expected, out)
		}
	}
}

func TestGetResolverConfig(t *testing.T) {
	config := framework.NewConfigStore("test", logtesting.TestLogger(t))
	if len(config.GetResolverConfig()) != 0 {
		t.Fatalf("expected empty config")
	}
}

func mapsAreEqual(m1, m2 map[string]string) bool {
	if m1 == nil || m2 == nil {
		return m1 == nil && m2 == nil
	}
	if len(m1) != len(m2) {
		return false
	}
	for k, v := range m1 {
		if m2[k] != v {
			return false
		}
	}
	return true
}
