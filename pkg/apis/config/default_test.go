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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
)

func TestNewDefaultsFromConfigMap(t *testing.T) {
	type testCase struct {
		expectedConfig *Defaults
		expectedError  bool
		fileName       string
	}

	testCases := []testCase{
		{
			expectedConfig: &Defaults{
				DefaultTimeoutMinutes:      50,
				DefaultServiceAccount:      "tekton",
				DefaultManagedByLabelValue: "something-else",
			},
			fileName: DefaultsConfigName,
		},
		{
			expectedConfig: &Defaults{
				DefaultTimeoutMinutes:      50,
				DefaultServiceAccount:      "tekton",
				DefaultManagedByLabelValue: DefaultManagedByLabelValue,
				DefaultPodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
				},
			},
			fileName: "config-defaults-with-pod-template",
		},
		// the github.com/ghodss/yaml package in the vendor directory does not support UnmarshalStrict
		// update it, switch to UnmarshalStrict in defaults.go, then uncomment these tests
		// {
		// 	expectedError: true,
		// 	fileName:      "config-defaults-timeout-err",
		// },
		// {
		// 	expectedError: true,
		// 	fileName:      "config-defaults-pod-template-err",
		// },
	}

	for _, tc := range testCases {
		if tc.expectedError {
			verifyConfigFileWithExpectedError(t, tc.fileName)
		} else {
			verifyConfigFileWithExpectedConfig(t, tc.fileName, tc.expectedConfig)
		}
	}
}

func TestNewDefaultsFromEmptyConfigMap(t *testing.T) {
	DefaultsConfigEmptyName := "config-defaults-empty"
	expectedConfig := &Defaults{
		DefaultTimeoutMinutes:      60,
		DefaultManagedByLabelValue: "tekton-pipelines",
	}
	verifyConfigFileWithExpectedConfig(t, DefaultsConfigEmptyName, expectedConfig)
}

func TestEquals(t *testing.T) {
	testCases := []struct {
		name     string
		left     *Defaults
		right    *Defaults
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
			right:    &Defaults{},
			expected: false,
		},
		{
			name:     "right nil",
			left:     &Defaults{},
			right:    nil,
			expected: false,
		},
		{
			name:     "right and right default",
			left:     &Defaults{},
			right:    &Defaults{},
			expected: true,
		},
		{
			name: "different default timeout",
			left: &Defaults{
				DefaultTimeoutMinutes: 10,
			},
			right: &Defaults{
				DefaultTimeoutMinutes: 20,
			},
			expected: false,
		},
		{
			name: "same default timeout",
			left: &Defaults{
				DefaultTimeoutMinutes: 20,
			},
			right: &Defaults{
				DefaultTimeoutMinutes: 20,
			},
			expected: true,
		},
		{
			name: "different default pod template",
			left: &Defaults{
				DefaultPodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
				},
			},
			right: &Defaults{
				DefaultPodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label2": "value",
					},
				},
			},
			expected: false,
		},
		{
			name: "same default pod template",
			left: &Defaults{
				DefaultPodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
				},
			},
			right: &Defaults{
				DefaultPodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
				},
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

func verifyConfigFileWithExpectedConfig(t *testing.T, fileName string, expectedConfig *Defaults) {
	cm := test.ConfigMapFromTestFile(t, fileName)
	if Defaults, err := NewDefaultsFromConfigMap(cm); err == nil {
		if d := cmp.Diff(Defaults, expectedConfig); d != "" {
			t.Errorf("Diff:\n%s", d)
		}
	} else {
		t.Errorf("NewDefaultsFromConfigMap(actual) = %v", err)
	}
}

func verifyConfigFileWithExpectedError(t *testing.T, fileName string) {
	cm := test.ConfigMapFromTestFile(t, fileName)
	if _, err := NewDefaultsFromConfigMap(cm); err == nil {
		t.Errorf("NewDefaultsFromConfigMap(actual) was expected to return an error")
	}
}
