/*
Copyright 2020 The Tekton Authors

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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

func TestNewFeatureFlagsFromConfigMap(t *testing.T) {
	type testCase struct {
		expectedConfig *config.FeatureFlags
		fileName       string
	}

	testCases := []testCase{
		{
			expectedConfig: &config.FeatureFlags{
				DisableAffinityAssistant:         false,
				RunningInEnvWithInjectedSidecars: true,
				RequireGitSSHSecretKnownHosts:    false,

				DisableCredsInit:         config.DefaultDisableCredsInit,
				AwaitSidecarReadiness:    config.DefaultAwaitSidecarReadiness,
				EnableTektonOCIBundles:   config.DefaultEnableTektonOciBundles,
				EnableAPIFields:          config.DefaultEnableAPIFields,
				SendCloudEventsForRuns:   config.DefaultSendCloudEventsForRuns,
				ResourceVerificationMode: config.DefaultResourceVerificationMode,
				EnableProvenanceInStatus: config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:   config.DefaultResultExtractionMethod,
				MaxResultSize:            config.DefaultMaxResultSize,
				CustomTaskVersion:        config.DefaultCustomTaskVersion,
			},
			fileName: config.GetFeatureFlagsConfigName(),
		},
		{
			expectedConfig: &config.FeatureFlags{
				DisableAffinityAssistant:         true,
				RunningInEnvWithInjectedSidecars: false,
				AwaitSidecarReadiness:            false,
				RequireGitSSHSecretKnownHosts:    true,
				EnableTektonOCIBundles:           true,
				EnableAPIFields:                  "alpha",
				SendCloudEventsForRuns:           true,
				EnforceNonfalsifiability:         "spire",
				ResourceVerificationMode:         "enforce",
				EnableProvenanceInStatus:         true,
				ResultExtractionMethod:           "termination-message",
				MaxResultSize:                    4096,
				CustomTaskVersion:                "v1beta1",
			},
			fileName: "feature-flags-all-flags-set",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields: "alpha",
				// These are prescribed as true by enabling "alpha" API fields, even
				// if the submitted text value is "false".
				EnableTektonOCIBundles:           true,
				EnforceNonfalsifiability:         "",
				DisableAffinityAssistant:         config.DefaultDisableAffinityAssistant,
				DisableCredsInit:                 config.DefaultDisableCredsInit,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				RequireGitSSHSecretKnownHosts:    config.DefaultRequireGitSSHSecretKnownHosts,
				SendCloudEventsForRuns:           config.DefaultSendCloudEventsForRuns,
				ResourceVerificationMode:         config.DefaultResourceVerificationMode,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				CustomTaskVersion:                config.DefaultCustomTaskVersion,
			},
			fileName: "feature-flags-enable-api-fields-overrides-bundles-and-custom-tasks",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:        "stable",
				EnableTektonOCIBundles: true,

				DisableAffinityAssistant:         config.DefaultDisableAffinityAssistant,
				DisableCredsInit:                 config.DefaultDisableCredsInit,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				RequireGitSSHSecretKnownHosts:    config.DefaultRequireGitSSHSecretKnownHosts,
				SendCloudEventsForRuns:           config.DefaultSendCloudEventsForRuns,
				ResourceVerificationMode:         config.DefaultResourceVerificationMode,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				CustomTaskVersion:                config.DefaultCustomTaskVersion,
			},
			fileName: "feature-flags-bundles-and-custom-tasks",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields: "beta",

				EnableTektonOCIBundles:           config.DefaultEnableTektonOciBundles,
				DisableAffinityAssistant:         config.DefaultDisableAffinityAssistant,
				DisableCredsInit:                 config.DefaultDisableCredsInit,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				RequireGitSSHSecretKnownHosts:    config.DefaultRequireGitSSHSecretKnownHosts,
				SendCloudEventsForRuns:           config.DefaultSendCloudEventsForRuns,
				ResourceVerificationMode:         config.DefaultResourceVerificationMode,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				CustomTaskVersion:                config.DefaultCustomTaskVersion,
			},
			fileName: "feature-flags-beta-api-fields",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:                  "alpha",
				EnforceNonfalsifiability:         "spire",
				EnableTektonOCIBundles:           true,
				ResourceVerificationMode:         config.DefaultResourceVerificationMode,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				CustomTaskVersion:                config.DefaultCustomTaskVersion,
			},
			fileName: "feature-flags-enforce-nonfalsifiability-spire",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:                  "stable",
				ResourceVerificationMode:         config.DefaultResourceVerificationMode,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				ResultExtractionMethod:           config.ResultExtractionMethodSidecarLogs,
				MaxResultSize:                    8192,
				CustomTaskVersion:                config.DefaultCustomTaskVersion,
			},
			fileName: "feature-flags-results-via-sidecar-logs",
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
	expectedConfig := &config.FeatureFlags{
		DisableAffinityAssistant:         config.DefaultDisableAffinityAssistant,
		DisableCredsInit:                 config.DefaultDisableCredsInit,
		RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
		AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
		RequireGitSSHSecretKnownHosts:    config.DefaultRequireGitSSHSecretKnownHosts,
		EnableTektonOCIBundles:           config.DefaultEnableTektonOciBundles,
		EnableAPIFields:                  config.DefaultEnableAPIFields,
		SendCloudEventsForRuns:           config.DefaultSendCloudEventsForRuns,
		EnforceNonfalsifiability:         config.DefaultEnforceNonfalsifiability,
		ResourceVerificationMode:         config.DefaultResourceVerificationMode,
		EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
		ResultExtractionMethod:           config.DefaultResultExtractionMethod,
		MaxResultSize:                    config.DefaultMaxResultSize,
		CustomTaskVersion:                config.DefaultCustomTaskVersion,
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
		expected:            "feature-flags",
	}, {
		description:         "Feature flags config value set",
		featureFlagEnvValue: "feature-flags-test",
		expected:            "feature-flags-test",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			if tc.featureFlagEnvValue != "" {
				t.Setenv("CONFIG_FEATURE_FLAGS_NAME", tc.featureFlagEnvValue)
			}
			got := config.GetFeatureFlagsConfigName()
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
	}, {
		fileName: "feature-flags-invalid-enable-api-fields",
	}, {
		fileName: "feature-flags-invalid-resource-verification-mode",
	}, {
		fileName: "feature-flags-invalid-results-from",
	}, {
		fileName: "feature-flags-invalid-max-result-size-too-large",
	}, {
		fileName: "feature-flags-invalid-max-result-size-bad-value",
	}, {
		fileName: "feature-flags-invalid-custom-task-version",
	}, {
		fileName: "feature-flags-enforce-nonfalsifiability-bad-flag",
	}, {
		fileName: "feature-flags-spire-with-stable",
	}} {
		t.Run(tc.fileName, func(t *testing.T) {
			cm := test.ConfigMapFromTestFile(t, tc.fileName)
			if _, err := config.NewFeatureFlagsFromConfigMap(cm); err == nil {
				t.Error("expected error but received nil")
			}
		})
	}
}

func TestCheckEnforceResourceVerificationMode(t *testing.T) {
	ctx := context.Background()
	if config.CheckEnforceResourceVerificationMode(ctx) {
		t.Errorf("CheckCheckEnforceResourceVerificationMode got true but expected to be false")
	}
	store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
	featureflags := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "feature-flags",
		},
		Data: map[string]string{
			"resource-verification-mode": config.EnforceResourceVerificationMode,
		},
	}
	store.OnConfigChanged(featureflags)
	ctx = store.ToContext(ctx)
	if !config.CheckEnforceResourceVerificationMode(ctx) {
		t.Errorf("CheckCheckEnforceResourceVerificationMode got false but expected to be true")
	}
}

func TestCheckWarnResourceVerificationMode(t *testing.T) {
	ctx := context.Background()
	if config.CheckWarnResourceVerificationMode(ctx) {
		t.Errorf("CheckWarnResourceVerificationMode got true but expected to be false")
	}
	store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
	featureflags := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "feature-flags",
		},
		Data: map[string]string{
			"resource-verification-mode": config.WarnResourceVerificationMode,
		},
	}
	store.OnConfigChanged(featureflags)
	ctx = store.ToContext(ctx)
	if !config.CheckWarnResourceVerificationMode(ctx) {
		t.Errorf("CheckWarnResourceVerificationMode got false but expected to be true")
	}
}

func verifyConfigFileWithExpectedFeatureFlagsConfig(t *testing.T, fileName string, expectedConfig *config.FeatureFlags) {
	t.Helper()
	cm := test.ConfigMapFromTestFile(t, fileName)
	if flags, err := config.NewFeatureFlagsFromConfigMap(cm); err == nil {
		if d := cmp.Diff(expectedConfig, flags); d != "" {
			t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
		}
	} else {
		t.Errorf("NewFeatureFlagsFromConfigMap(actual) = %v", err)
	}
}
