/*
Copyright 2023 The Tekton Authors

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

package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewTracingFromConfigMap(t *testing.T) {
	for _, tc := range []struct {
		name     string
		want     *config.Tracing
		fileName string
	}{
		{
			name: "empty",
			want: &config.Tracing{
				Enabled:  false,
				Endpoint: "http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces",
			},
			fileName: "config-tracing-empty",
		},
		{
			name: "enabled with endpoint",
			want: &config.Tracing{
				Enabled:  true,
				Endpoint: "http://jaeger-test",
			},
			fileName: "config-tracing-enabled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm := test.ConfigMapFromTestFile(t, tc.fileName)
			if got, err := config.NewTracingFromConfigMap(cm); err == nil {
				if d := cmp.Diff(tc.want, got); d != "" {
					t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
				}
			} else {
				t.Errorf("NewTracingFromConfigMap(actual) = %v", err)
			}
		})
	}
}

func TestTracingEquals(t *testing.T) {
	testCases := []struct {
		name     string
		left     *config.Tracing
		right    *config.Tracing
		expected bool
	}{
		{
			name:     "left and right nil",
			left:     nil,
			right:    nil,
			expected: true,
		},
		{
			name:     "left nil",
			left:     nil,
			right:    &config.Tracing{},
			expected: false,
		},
		{
			name:     "right nil",
			left:     &config.Tracing{},
			right:    nil,
			expected: false,
		},
		{
			name:     "right and right default",
			left:     &config.Tracing{},
			right:    &config.Tracing{},
			expected: true,
		},
		{
			name: "different enabled",
			left: &config.Tracing{
				Enabled: true,
			},
			right: &config.Tracing{
				Enabled: false,
			},
			expected: false,
		},
		{
			name: "different endpoint",
			left: &config.Tracing{
				Endpoint: "a",
			},
			right: &config.Tracing{
				Endpoint: "b",
			},
			expected: false,
		},
		{
			name: "different credentialsSecret",
			left: &config.Tracing{
				CredentialsSecret: "a",
			},
			right: &config.Tracing{
				CredentialsSecret: "b",
			},
			expected: false,
		},
		{
			name: "same all fields",
			left: &config.Tracing{
				Enabled:           true,
				Endpoint:          "a",
				CredentialsSecret: "b",
			},
			right: &config.Tracing{
				Enabled:           true,
				Endpoint:          "a",
				CredentialsSecret: "b",
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.left.Equals(tc.right)
			if actual != tc.expected {
				t.Errorf("Comparison failed expected: %t, actual: %t", tc.expected, actual)
			}
		})
	}
}
