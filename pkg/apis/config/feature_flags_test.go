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

				DisableCredsInit:          config.DefaultDisableCredsInit,
				AwaitSidecarReadiness:     config.DefaultAwaitSidecarReadiness,
				EnableTektonOCIBundles:    config.DefaultEnableTektonOciBundles,
				EnableAPIFields:           config.DefaultEnableAPIFields,
				SendCloudEventsForRuns:    config.DefaultSendCloudEventsForRuns,
				VerificationNoMatchPolicy: config.DefaultNoMatchPolicyConfig,
				EnableProvenanceInStatus:  config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:    config.DefaultResultExtractionMethod,
				MaxResultSize:             config.DefaultMaxResultSize,
				SetSecurityContext:        config.DefaultSetSecurityContext,
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
				VerificationNoMatchPolicy:        config.FailNoMatchPolicy,
				EnableProvenanceInStatus:         false,
				ResultExtractionMethod:           "termination-message",
				MaxResultSize:                    4096,
				SetSecurityContext:               true,
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
				VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
				EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				SetSecurityContext:               config.DefaultSetSecurityContext,
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
				VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
				EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				SetSecurityContext:               config.DefaultSetSecurityContext,
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
				VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
				EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				SetSecurityContext:               config.DefaultSetSecurityContext,
			},
			fileName: "feature-flags-beta-api-fields",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:                  "alpha",
				EnforceNonfalsifiability:         "spire",
				EnableTektonOCIBundles:           true,
				VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				SetSecurityContext:               config.DefaultSetSecurityContext,
			},
			fileName: "feature-flags-enforce-nonfalsifiability-spire",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:                  config.DefaultEnableAPIFields,
				VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:           config.ResultExtractionMethodSidecarLogs,
				MaxResultSize:                    8192,
				SetSecurityContext:               config.DefaultSetSecurityContext,
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
		VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
		EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
		ResultExtractionMethod:           config.DefaultResultExtractionMethod,
		MaxResultSize:                    config.DefaultMaxResultSize,
		SetSecurityContext:               config.DefaultSetSecurityContext,
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
		want     string
	}{{
		fileName: "feature-flags-invalid-boolean",
		want:     `failed parsing feature flags config "im-really-not-a-valid-boolean": strconv.ParseBool: parsing "im-really-not-a-valid-boolean": invalid syntax`,
	}, {
		fileName: "feature-flags-invalid-enable-api-fields",
		want:     `invalid value for feature flag "enable-api-fields": "im-not-a-valid-feature-gate"`,
	}, {
		fileName: "feature-flags-invalid-trusted-resources-verification-no-match-policy",
		want:     `invalid value for feature flag "trusted-resources-verification-no-match-policy": "wrong value"`,
	}, {
		fileName: "feature-flags-invalid-results-from",
		want:     `invalid value for feature flag "results-from": "im-not-a-valid-results-from"`,
	}, {
		fileName: "feature-flags-invalid-max-result-size-too-large",
		want:     `invalid value for feature flag "results-from": "10000000000000". This is exceeding the CRD limit`,
	}, {
		fileName: "feature-flags-invalid-max-result-size-bad-value",
		want:     `strconv.Atoi: parsing "foo": invalid syntax`,
	}, {
		fileName: "feature-flags-enforce-nonfalsifiability-bad-flag",
		want:     `invalid value for feature flag "enforce-nonfalsifiability": "bad-value"`,
	}, {
		fileName: "feature-flags-spire-with-stable",
		want:     `"enforce-nonfalsifiability" can be set to non-default values ("spire") only in alpha`,
	}} {
		t.Run(tc.fileName, func(t *testing.T) {
			cm := test.ConfigMapFromTestFile(t, tc.fileName)
			_, err := config.NewFeatureFlagsFromConfigMap(cm)
			if d := cmp.Diff(tc.want, err.Error()); d != "" {
				t.Errorf("failed to get expected error; diff:\n%s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetVerificationNoMatchPolicy(t *testing.T) {
	ctx := context.Background()
	tcs := []struct {
		name, noMatchPolicy, expected string
	}{{
		name:          "ignore no match policy",
		noMatchPolicy: config.IgnoreNoMatchPolicy,
		expected:      config.IgnoreNoMatchPolicy,
	}, {
		name:          "warn no match policy",
		noMatchPolicy: config.WarnNoMatchPolicy,
		expected:      config.WarnNoMatchPolicy,
	}, {
		name:          "fail no match policy",
		noMatchPolicy: config.FailNoMatchPolicy,
		expected:      config.FailNoMatchPolicy,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
			featureflags := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "feature-flags",
				},
				Data: map[string]string{
					"trusted-resources-verification-no-match-policy": tc.noMatchPolicy,
				},
			}
			store.OnConfigChanged(featureflags)
			ctx = store.ToContext(ctx)
			got := config.GetVerificationNoMatchPolicy(ctx)
			if d := cmp.Diff(tc.expected, got); d != "" {
				t.Errorf("Unexpected feature flag config: %s", diff.PrintWantGot(d))
			}
		})
	}
}

// TestCheckAlphaOrBetaAPIFields validates CheckAlphaOrBetaAPIFields for alpha, beta, and stable context
func TestCheckAlphaOrBetaAPIFields(t *testing.T) {
	ctx := context.Background()
	type testCase struct {
		name           string
		c              context.Context
		expectedResult bool
	}
	testCases := []testCase{{
		name:           "when enable-api-fields is set to alpha",
		c:              config.EnableAlphaAPIFields(ctx),
		expectedResult: true,
	}, {
		name:           "when enable-api-fields is set to beta",
		c:              config.EnableBetaAPIFields(ctx),
		expectedResult: true,
	}, {
		name:           "when enable-api-fields is set to stable",
		c:              config.EnableStableAPIFields(ctx),
		expectedResult: false,
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := config.CheckAlphaOrBetaAPIFields(tc.c)
			if (tc.expectedResult && !e) || (!tc.expectedResult && e) {
				t.Errorf("Failed to validate CheckAlphaOrBetaAPIFields for \"%v\", expected \"%v\" but got \"%v\"", tc.name, tc.expectedResult, e)
			}
		})
	}
}

func TestIsSpireEnabled(t *testing.T) {
	testCases := []struct {
		name      string
		configmap map[string]string
		want      bool
	}{{
		name: "when enable-api-fields is set to beta and non-falsifiablity is not set.",
		configmap: map[string]string{
			"enable-api-fields":         "beta",
			"enforce-nonfalsifiability": config.EnforceNonfalsifiabilityNone,
		},
		want: false,
	}, {
		name: "when enable-api-fields is set to beta and non-falsifiability is set to 'spire'",
		configmap: map[string]string{
			"enable-api-fields":         "beta",
			"enforce-nonfalsifiability": config.EnforceNonfalsifiabilityWithSpire,
		},
		want: false,
	}, {
		name: "when enable-api-fields is set to alpha and non-falsifiability is not set",
		configmap: map[string]string{
			"enable-api-fields":         "alpha",
			"enforce-nonfalsifiability": config.EnforceNonfalsifiabilityNone,
		},
		want: false,
	}, {
		name: "when enable-api-fields is set to alpha and non-falsifiability is set to 'spire'",
		configmap: map[string]string{
			"enable-api-fields":         "alpha",
			"enforce-nonfalsifiability": config.EnforceNonfalsifiabilityWithSpire,
		},
		want: true,
	}}
	ctx := context.Background()
	store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
	for _, tc := range testCases {
		featureflags := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "feature-flags",
			},
			Data: tc.configmap,
		}
		store.OnConfigChanged(featureflags)
		ctx = store.ToContext(ctx)
		got := config.IsSpireEnabled(ctx)

		if tc.want != got {
			t.Errorf("IsSpireEnabled() = %t, want %t", got, tc.want)
		}
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
