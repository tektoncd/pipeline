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

package resolver_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewFeatureFlagsFromConfigMap(t *testing.T) {
	type testCase struct {
		expectedConfig *resolver.FeatureFlags
		fileName       string
	}

	testCases := []testCase{
		{
			expectedConfig: &resolver.FeatureFlags{
				EnableGitResolver:     true,
				EnableHubResolver:     true,
				EnableBundleResolver:  true,
				EnableClusterResolver: true,
			},
			fileName: "feature-flags-empty",
		},
		{
			expectedConfig: &resolver.FeatureFlags{
				EnableGitResolver:     false,
				EnableHubResolver:     false,
				EnableBundleResolver:  false,
				EnableClusterResolver: false,
			},
			fileName: "feature-flags-all-flags-set",
		},
	}

	for _, tc := range testCases {
		fileName := tc.fileName
		expectedConfig := tc.expectedConfig
		t.Run(fileName, func(t *testing.T) {
			verifyConfigFileWithExpectedFeatureFlagsConfig(t, fileName, expectedConfig)
		})
	}
}

func TestNewFeatureFlagsFromEmptyConfigMap(t *testing.T) {
	FeatureFlagsConfigEmptyName := "feature-flags-empty"
	expectedConfig := &resolver.FeatureFlags{
		EnableGitResolver:     resolver.DefaultEnableGitResolver,
		EnableHubResolver:     resolver.DefaultEnableHubResolver,
		EnableBundleResolver:  resolver.DefaultEnableBundlesResolver,
		EnableClusterResolver: resolver.DefaultEnableClusterResolver,
	}
	verifyConfigFileWithExpectedFeatureFlagsConfig(t, FeatureFlagsConfigEmptyName, expectedConfig)
}

func TestGetFeatureFlagsConfigName(t *testing.T) {
	for _, tc := range []struct {
		description         string
		featureFlagEnvValue string
		expected            string
	}{{
		description:         "Feature flags config value not set",
		featureFlagEnvValue: "",
		expected:            "resolvers-feature-flags",
	}, {
		description:         "Feature flags config value set",
		featureFlagEnvValue: "feature-flags-test",
		expected:            "feature-flags-test",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			if tc.featureFlagEnvValue != "" {
				t.Setenv("CONFIG_RESOLVERS_FEATURE_FLAGS_NAME", tc.featureFlagEnvValue)
			}
			got := resolver.GetFeatureFlagsConfigName()
			want := tc.expected
			if got != want {
				t.Errorf("GetFeatureFlagsConfigName() = %s, want %s", got, want)
			}
		})
	}
}

func TestNewFeatureFlagsConfigMapErrors(t *testing.T) {
	for _, tc := range []struct {
		fileName string
	}{{
		fileName: "feature-flags-invalid-boolean",
	}} {
		t.Run(tc.fileName, func(t *testing.T) {
			cm := test.ConfigMapFromTestFile(t, tc.fileName)
			if _, err := resolver.NewFeatureFlagsFromConfigMap(cm); err == nil {
				t.Error("expected error but received nil")
			}
		})
	}
}

func verifyConfigFileWithExpectedFeatureFlagsConfig(t *testing.T, fileName string, expectedConfig *resolver.FeatureFlags) {
	t.Helper()
	cm := test.ConfigMapFromTestFile(t, fileName)
	if flags, err := resolver.NewFeatureFlagsFromConfigMap(cm); err == nil {
		if d := cmp.Diff(expectedConfig, flags); d != "" {
			t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
		}
	} else {
		t.Errorf("NewFeatureFlagsFromConfigMap(actual) = %v", err)
	}
}
