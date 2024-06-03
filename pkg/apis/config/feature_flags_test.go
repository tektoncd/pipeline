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
				DisableCredsInit:                 config.DefaultDisableCredsInit,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				EnableAPIFields:                  config.DefaultEnableAPIFields,
				SendCloudEventsForRuns:           config.DefaultSendCloudEventsForRuns,
				VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
				EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				SetSecurityContext:               config.DefaultSetSecurityContext,
				Coschedule:                       config.DefaultCoschedule,
				EnforceNonfalsifiability:         config.DefaultEnforceNonfalsifiability,
				EnableKeepPodOnCancel:            config.DefaultEnableKeepPodOnCancel.Enabled,
				EnableCELInWhenExpression:        config.DefaultEnableCELInWhenExpression.Enabled,
				EnableStepActions:                config.DefaultEnableStepActions.Enabled,
				EnableParamEnum:                  config.DefaultEnableParamEnum.Enabled,
				DisableInlineSpec:                config.DefaultDisableInlineSpec,
				EnableConciseResolverSyntax:      config.DefaultEnableConciseResolverSyntax.Enabled,
			},
			fileName: config.GetFeatureFlagsConfigName(),
		},
		{
			expectedConfig: &config.FeatureFlags{
				DisableAffinityAssistant:         true,
				RunningInEnvWithInjectedSidecars: false,
				AwaitSidecarReadiness:            false,
				RequireGitSSHSecretKnownHosts:    true,
				EnableAPIFields:                  "alpha",
				SendCloudEventsForRuns:           true,
				EnforceNonfalsifiability:         "spire",
				VerificationNoMatchPolicy:        config.FailNoMatchPolicy,
				EnableProvenanceInStatus:         false,
				ResultExtractionMethod:           "termination-message",
				EnableKeepPodOnCancel:            true,
				MaxResultSize:                    4096,
				SetSecurityContext:               true,
				Coschedule:                       config.CoscheduleDisabled,
				EnableCELInWhenExpression:        true,
				EnableStepActions:                true,
				EnableArtifacts:                  true,
				EnableParamEnum:                  true,
				DisableInlineSpec:                "pipeline,pipelinerun,taskrun",
				EnableConciseResolverSyntax:      true,
				EnableKubernetesSidecar:          true,
			},
			fileName: "feature-flags-all-flags-set",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields: "alpha",
				// These are prescribed as true by enabling "alpha" API fields, even
				// if the submitted text value is "false".
				EnforceNonfalsifiability:         config.DefaultEnforceNonfalsifiability,
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
				Coschedule:                       config.DefaultCoschedule,
				EnableKeepPodOnCancel:            config.DefaultEnableKeepPodOnCancel.Enabled,
				EnableCELInWhenExpression:        config.DefaultEnableCELInWhenExpression.Enabled,
				EnableStepActions:                config.DefaultEnableStepActions.Enabled,
				EnableParamEnum:                  config.DefaultEnableParamEnum.Enabled,
				EnableArtifacts:                  config.DefaultEnableArtifacts.Enabled,
				DisableInlineSpec:                config.DefaultDisableInlineSpec,
			},
			fileName: "feature-flags-enable-api-fields-overrides-bundles-and-custom-tasks",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:                  "stable",
				EnforceNonfalsifiability:         config.DefaultEnforceNonfalsifiability,
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
				Coschedule:                       config.DefaultCoschedule,
				EnableParamEnum:                  config.DefaultEnableParamEnum.Enabled,
				DisableInlineSpec:                config.DefaultDisableInlineSpec,
			},
			fileName: "feature-flags-bundles-and-custom-tasks",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:                  "beta",
				EnforceNonfalsifiability:         config.DefaultEnforceNonfalsifiability,
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
				Coschedule:                       config.DefaultCoschedule,
				EnableParamEnum:                  config.DefaultEnableParamEnum.Enabled,
				DisableInlineSpec:                config.DefaultDisableInlineSpec,
			},
			fileName: "feature-flags-beta-api-fields",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:                  config.DefaultEnableAPIFields,
				EnforceNonfalsifiability:         config.EnforceNonfalsifiabilityWithSpire,
				VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:           config.DefaultResultExtractionMethod,
				MaxResultSize:                    config.DefaultMaxResultSize,
				SetSecurityContext:               config.DefaultSetSecurityContext,
				Coschedule:                       config.DefaultCoschedule,
				EnableKeepPodOnCancel:            config.DefaultEnableKeepPodOnCancel.Enabled,
				EnableCELInWhenExpression:        config.DefaultEnableCELInWhenExpression.Enabled,
				EnableStepActions:                config.DefaultEnableStepActions.Enabled,
				EnableParamEnum:                  config.DefaultEnableParamEnum.Enabled,
				DisableInlineSpec:                config.DefaultDisableInlineSpec,
			},
			fileName: "feature-flags-enforce-nonfalsifiability-spire",
		},
		{
			expectedConfig: &config.FeatureFlags{
				EnableAPIFields:                  config.DefaultEnableAPIFields,
				EnforceNonfalsifiability:         config.DefaultEnforceNonfalsifiability,
				VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
				RunningInEnvWithInjectedSidecars: config.DefaultRunningInEnvWithInjectedSidecars,
				AwaitSidecarReadiness:            config.DefaultAwaitSidecarReadiness,
				EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
				ResultExtractionMethod:           config.ResultExtractionMethodSidecarLogs,
				MaxResultSize:                    8192,
				SetSecurityContext:               config.DefaultSetSecurityContext,
				Coschedule:                       config.DefaultCoschedule,
				EnableKeepPodOnCancel:            config.DefaultEnableKeepPodOnCancel.Enabled,
				EnableCELInWhenExpression:        config.DefaultEnableCELInWhenExpression.Enabled,
				EnableStepActions:                config.DefaultEnableStepActions.Enabled,
				EnableParamEnum:                  config.DefaultEnableParamEnum.Enabled,
				DisableInlineSpec:                config.DefaultDisableInlineSpec,
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
		EnableAPIFields:                  config.DefaultEnableAPIFields,
		SendCloudEventsForRuns:           config.DefaultSendCloudEventsForRuns,
		EnforceNonfalsifiability:         config.DefaultEnforceNonfalsifiability,
		VerificationNoMatchPolicy:        config.DefaultNoMatchPolicyConfig,
		EnableProvenanceInStatus:         config.DefaultEnableProvenanceInStatus,
		ResultExtractionMethod:           config.DefaultResultExtractionMethod,
		MaxResultSize:                    config.DefaultMaxResultSize,
		SetSecurityContext:               config.DefaultSetSecurityContext,
		Coschedule:                       config.DefaultCoschedule,
		EnableKeepPodOnCancel:            config.DefaultEnableKeepPodOnCancel.Enabled,
		EnableCELInWhenExpression:        config.DefaultEnableCELInWhenExpression.Enabled,
		EnableStepActions:                config.DefaultEnableStepActions.Enabled,
		EnableParamEnum:                  config.DefaultEnableParamEnum.Enabled,
		DisableInlineSpec:                config.DefaultDisableInlineSpec,
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
		fileName: "feature-flags-invalid-coschedule-affinity-assistant-comb",
		want:     `coschedule value pipelineruns is incompatible with disable-affinity-assistant setting to false`,
	}, {
		fileName: "feature-flags-invalid-coschedule",
		want:     `invalid value for feature flag "coschedule": "invalid"`,
	}, {
		fileName: "feature-flags-invalid-keep-pod-on-cancel",
		want:     `failed parsing feature flags config "invalid": strconv.ParseBool: parsing "invalid": invalid syntax for feature keep-pod-on-cancel`,
	}, {
		fileName: "feature-flags-invalid-running-in-environment-with-injected-sidecars",
		want:     `failed parsing feature flags config "invalid-boolean": strconv.ParseBool: parsing "invalid-boolean": invalid syntax`,
	}, {
		fileName: "feature-flags-invalid-disable-affinity-assistant",
		want:     `failed parsing feature flags config "truee": strconv.ParseBool: parsing "truee": invalid syntax`,
	}, {
		fileName: "feature-flags-invalid-enable-cel-in-whenexpression",
		want:     `failed parsing feature flags config "invalid": strconv.ParseBool: parsing "invalid": invalid syntax for feature enable-cel-in-whenexpression`,
	}, {
		fileName: "feature-flags-invalid-enable-step-actions",
		want:     `failed parsing feature flags config "invalid": strconv.ParseBool: parsing "invalid": invalid syntax for feature enable-step-actions`,
	}, {
		fileName: "feature-flags-invalid-enable-param-enum",
		want:     `failed parsing feature flags config "invalid": strconv.ParseBool: parsing "invalid": invalid syntax for feature enable-param-enum`,
	}, {
		fileName: "feature-flags-invalid-enable-artifacts",
		want:     `failed parsing feature flags config "invalid": strconv.ParseBool: parsing "invalid": invalid syntax for feature enable-artifacts`,
	}, {
		fileName: "feature-flags-invalid-enable-concise-resolver-syntax",
		want:     `failed parsing feature flags config "invalid": strconv.ParseBool: parsing "invalid": invalid syntax for feature enable-concise-resolver-syntax`,
	}, {
		fileName: "feature-flags-invalid-enable-kubernetes-sidecar",
		want:     `failed parsing feature flags config "invalid": strconv.ParseBool: parsing "invalid": invalid syntax`,
	}} {
		t.Run(tc.fileName, func(t *testing.T) {
			cm := test.ConfigMapFromTestFile(t, tc.fileName)
			_, err := config.NewFeatureFlagsFromConfigMap(cm)
			if err == nil {
				t.Error("failed to get:", tc.want)
			} else if d := cmp.Diff(tc.want, err.Error()); d != "" {
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
		want: true,
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
		ctx := store.ToContext(ctx)
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
