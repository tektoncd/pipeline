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

package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	// StableAPIFields is the value used for API-driven features of stable stability level.
	StableAPIFields = "stable"
	// AlphaAPIFields is the value used for API-driven features of alpha stability level.
	AlphaAPIFields = "alpha"
	// BetaAPIFields is the value used for API-driven features of beta stability level.
	BetaAPIFields = "beta"
	// Features of "alpha" stability level are disabled by default
	DefaultAlphaFeatureEnabled = false
	// Features of "beta" stability level are disabled by default
	DefaultBetaFeatureEnabled = false
	// Features of "stable" stability level are enabled by default
	DefaultStableFeatureEnabled = true
	// FailNoMatchPolicy is the value used for "trusted-resources-verification-no-match-policy" to fail TaskRun or PipelineRun
	// when no matching policies are found
	FailNoMatchPolicy = "fail"
	// WarnNoMatchPolicy is the value used for "trusted-resources-verification-no-match-policy" to log warning and skip verification
	// when no matching policies are found
	WarnNoMatchPolicy = "warn"
	// IgnoreNoMatchPolicy is the value used for "trusted-resources-verification-no-match-policy" to skip verification
	// when no matching policies are found
	IgnoreNoMatchPolicy = "ignore"
	// CoscheduleWorkspaces is the value used for "coschedule" to coschedule PipelineRun Pods sharing the same PVC workspaces to the same node
	CoscheduleWorkspaces = "workspaces"
	// CoschedulePipelineRuns is the value used for "coschedule" to coschedule all PipelineRun Pods to the same node
	CoschedulePipelineRuns = "pipelineruns"
	// CoscheduleIsolatePipelineRun is the value used for "coschedule" to coschedule all PipelineRun Pods to the same node, and only allows one PipelineRun to run on a node at a time
	CoscheduleIsolatePipelineRun = "isolate-pipelinerun"
	// CoscheduleDisabled is the value used for "coschedule" to disabled PipelineRun Pods coschedule
	CoscheduleDisabled = "disabled"
	// ResultExtractionMethodTerminationMessage is the value used for "results-from" as a way to extract results from tasks using kubernetes termination message.
	ResultExtractionMethodTerminationMessage = "termination-message"
	// ResultExtractionMethodSidecarLogs is the value used for "results-from" as a way to extract results from tasks using sidecar logs.
	ResultExtractionMethodSidecarLogs = "sidecar-logs"
	// DefaultDisableAffinityAssistant is the default value for "disable-affinity-assistant".
	DefaultDisableAffinityAssistant = false
	// DefaultDisableCredsInit is the default value for "disable-creds-init".
	DefaultDisableCredsInit = false
	// DefaultRunningInEnvWithInjectedSidecars is the default value for "running-in-environment-with-injected-sidecars".
	DefaultRunningInEnvWithInjectedSidecars = true
	// DefaultAwaitSidecarReadiness is the default value for "await-sidecar-readiness".
	DefaultAwaitSidecarReadiness = true
	// DefaultRequireGitSSHSecretKnownHosts is the default value for "require-git-ssh-secret-known-hosts".
	DefaultRequireGitSSHSecretKnownHosts = false
	// DefaultEnableTektonOciBundles is the default value for "enable-tekton-oci-bundles".
	DefaultEnableTektonOciBundles = false
	// DefaultEnableAPIFields is the default value for "enable-api-fields".
	DefaultEnableAPIFields = BetaAPIFields
	// DefaultSendCloudEventsForRuns is the default value for "send-cloudevents-for-runs".
	DefaultSendCloudEventsForRuns = false
	// EnforceNonfalsifiabilityWithSpire is the value used for  "enable-nonfalsifiability" when SPIRE is used to enable non-falsifiability.
	EnforceNonfalsifiabilityWithSpire = "spire"
	// EnforceNonfalsifiabilityNone is the value used for  "enable-nonfalsifiability" when non-falsifiability is not enabled.
	EnforceNonfalsifiabilityNone = "none"
	// DefaultEnforceNonfalsifiability is the default value for "enforce-nonfalsifiability".
	DefaultEnforceNonfalsifiability = EnforceNonfalsifiabilityNone
	// DefaultNoMatchPolicyConfig is the default value for "trusted-resources-verification-no-match-policy".
	DefaultNoMatchPolicyConfig = IgnoreNoMatchPolicy
	// DefaultEnableProvenanceInStatus is the default value for "enable-provenance-status".
	DefaultEnableProvenanceInStatus = true
	// DefaultResultExtractionMethod is the default value for ResultExtractionMethod
	DefaultResultExtractionMethod = ResultExtractionMethodTerminationMessage
	// DefaultMaxResultSize is the default value in bytes for the size of a result
	DefaultMaxResultSize = 4096
	// DefaultSetSecurityContext is the default value for "set-security-context"
	DefaultSetSecurityContext = false
	// DefaultCoschedule is the default value for coschedule
	DefaultCoschedule = CoscheduleWorkspaces
	// KeepPodOnCancel is the flag used to enable cancelling a pod using the entrypoint, and keep pod on cancel
	KeepPodOnCancel = "keep-pod-on-cancel"
	// DefaultEnableKeepPodOnCancel is the default value for "keep-pod-on-cancel"
	DefaultEnableKeepPodOnCancel = false
	// EnableCELInWhenExpression is the flag to enabled CEL in WhenExpression
	EnableCELInWhenExpression = "enable-cel-in-whenexpression"
	// DefaultEnableCELInWhenExpression is the default value for EnableCELInWhenExpression
	DefaultEnableCELInWhenExpression = false
	// EnableStepActions is the flag to enable the use of StepActions in Steps
	EnableStepActions = "enable-step-actions"
	// DefaultEnableStepActions is the default value for EnableStepActions
	DefaultEnableStepActions = false

	disableAffinityAssistantKey         = "disable-affinity-assistant"
	disableCredsInitKey                 = "disable-creds-init"
	runningInEnvWithInjectedSidecarsKey = "running-in-environment-with-injected-sidecars"
	awaitSidecarReadinessKey            = "await-sidecar-readiness"
	requireGitSSHSecretKnownHostsKey    = "require-git-ssh-secret-known-hosts" //nolint:gosec
	enableTektonOCIBundles              = "enable-tekton-oci-bundles"
	enableAPIFields                     = "enable-api-fields"
	sendCloudEventsForRuns              = "send-cloudevents-for-runs"
	enforceNonfalsifiability            = "enforce-nonfalsifiability"
	verificationNoMatchPolicy           = "trusted-resources-verification-no-match-policy"
	enableProvenanceInStatus            = "enable-provenance-in-status"
	resultExtractionMethod              = "results-from"
	maxResultSize                       = "max-result-size"
	setSecurityContextKey               = "set-security-context"
	coscheduleKey                       = "coschedule"
)

// DefaultFeatureFlags holds all the default configurations for the feature flags configmap.
var DefaultFeatureFlags, _ = NewFeatureFlagsFromMap(map[string]string{})

// FeatureFlags holds the features configurations
// +k8s:deepcopy-gen=true
//
//nolint:musttag
type FeatureFlags struct {
	DisableAffinityAssistant         bool
	DisableCredsInit                 bool
	RunningInEnvWithInjectedSidecars bool
	RequireGitSSHSecretKnownHosts    bool
	EnableTektonOCIBundles           bool
	ScopeWhenExpressionsToTask       bool
	EnableAPIFields                  string
	SendCloudEventsForRuns           bool
	AwaitSidecarReadiness            bool
	EnforceNonfalsifiability         string
	EnableKeepPodOnCancel            bool
	// VerificationNoMatchPolicy is the feature flag for "trusted-resources-verification-no-match-policy"
	// VerificationNoMatchPolicy can be set to "ignore", "warn" and "fail" values.
	// ignore: skip trusted resources verification when no matching verification policies found
	// warn: skip trusted resources verification when no matching verification policies found and log a warning
	// fail: fail the taskrun or pipelines run if no matching verification policies found
	VerificationNoMatchPolicy string
	EnableProvenanceInStatus  bool
	ResultExtractionMethod    string
	MaxResultSize             int
	SetSecurityContext        bool
	Coschedule                string
	EnableCELInWhenExpression bool
	EnableStepActions         bool
}

// GetFeatureFlagsConfigName returns the name of the configmap containing all
// feature flags.
func GetFeatureFlagsConfigName() string {
	if e := os.Getenv("CONFIG_FEATURE_FLAGS_NAME"); e != "" {
		return e
	}
	return "feature-flags"
}

// NewFeatureFlagsFromMap returns a Config given a map corresponding to a ConfigMap
func NewFeatureFlagsFromMap(cfgMap map[string]string) (*FeatureFlags, error) {
	setFeature := func(key string, defaultValue bool, feature *bool) error {
		if cfg, ok := cfgMap[key]; ok {
			value, err := strconv.ParseBool(cfg)
			if err != nil {
				return fmt.Errorf("failed parsing feature flags config %q: %w", cfg, err)
			}
			*feature = value
			return nil
		}
		*feature = defaultValue
		return nil
	}

	tc := FeatureFlags{}
	if err := setFeature(disableAffinityAssistantKey, DefaultDisableAffinityAssistant, &tc.DisableAffinityAssistant); err != nil {
		return nil, err
	}
	if err := setFeature(disableCredsInitKey, DefaultDisableCredsInit, &tc.DisableCredsInit); err != nil {
		return nil, err
	}
	if err := setFeature(runningInEnvWithInjectedSidecarsKey, DefaultRunningInEnvWithInjectedSidecars, &tc.RunningInEnvWithInjectedSidecars); err != nil {
		return nil, err
	}
	if err := setFeature(awaitSidecarReadinessKey, DefaultAwaitSidecarReadiness, &tc.AwaitSidecarReadiness); err != nil {
		return nil, err
	}
	if err := setFeature(requireGitSSHSecretKnownHostsKey, DefaultRequireGitSSHSecretKnownHosts, &tc.RequireGitSSHSecretKnownHosts); err != nil {
		return nil, err
	}
	if err := setEnabledAPIFields(cfgMap, DefaultEnableAPIFields, &tc.EnableAPIFields); err != nil {
		return nil, err
	}
	if err := setFeature(sendCloudEventsForRuns, DefaultSendCloudEventsForRuns, &tc.SendCloudEventsForRuns); err != nil {
		return nil, err
	}
	if err := setVerificationNoMatchPolicy(cfgMap, DefaultNoMatchPolicyConfig, &tc.VerificationNoMatchPolicy); err != nil {
		return nil, err
	}
	if err := setFeature(enableProvenanceInStatus, DefaultEnableProvenanceInStatus, &tc.EnableProvenanceInStatus); err != nil {
		return nil, err
	}
	if err := setResultExtractionMethod(cfgMap, DefaultResultExtractionMethod, &tc.ResultExtractionMethod); err != nil {
		return nil, err
	}
	if err := setMaxResultSize(cfgMap, DefaultMaxResultSize, &tc.MaxResultSize); err != nil {
		return nil, err
	}
	if err := setFeature(KeepPodOnCancel, DefaultEnableKeepPodOnCancel, &tc.EnableKeepPodOnCancel); err != nil {
		return nil, err
	}
	if err := setEnforceNonFalsifiability(cfgMap, &tc.EnforceNonfalsifiability); err != nil {
		return nil, err
	}
	if err := setFeature(setSecurityContextKey, DefaultSetSecurityContext, &tc.SetSecurityContext); err != nil {
		return nil, err
	}
	if err := setCoschedule(cfgMap, DefaultCoschedule, tc.DisableAffinityAssistant, &tc.Coschedule); err != nil {
		return nil, err
	}
	if err := setFeature(EnableCELInWhenExpression, DefaultEnableCELInWhenExpression, &tc.EnableCELInWhenExpression); err != nil {
		return nil, err
	}
	if err := setFeature(EnableStepActions, DefaultEnableStepActions, &tc.EnableStepActions); err != nil {
		return nil, err
	}
	// Given that they are alpha features, Tekton Bundles and Custom Tasks should be switched on if
	// enable-api-fields is "alpha". If enable-api-fields is not "alpha" then fall back to the value of
	// each feature's individual flag.
	//
	// Note: the user cannot enable "alpha" while disabling bundles or custom tasks - that would
	// defeat the purpose of having a single shared gate for all alpha features.
	if tc.EnableAPIFields == AlphaAPIFields {
		tc.EnableTektonOCIBundles = true
	} else {
		if err := setFeature(enableTektonOCIBundles, DefaultEnableTektonOciBundles, &tc.EnableTektonOCIBundles); err != nil {
			return nil, err
		}
	}
	return &tc, nil
}

// setEnabledAPIFields sets the "enable-api-fields" flag based on the content of a given map.
// If the feature gate is invalid or missing then an error is returned.
func setEnabledAPIFields(cfgMap map[string]string, defaultValue string, feature *string) error {
	value := defaultValue
	if cfg, ok := cfgMap[enableAPIFields]; ok {
		value = strings.ToLower(cfg)
	}
	switch value {
	case AlphaAPIFields, BetaAPIFields, StableAPIFields:
		*feature = value
	default:
		return fmt.Errorf("invalid value for feature flag %q: %q", enableAPIFields, value)
	}
	return nil
}

// setCoschedule sets the "coschedule" flag based on the content of a given map.
// If the feature gate is invalid or incompatible with `disable-affinity-assistant`, then an error is returned.
func setCoschedule(cfgMap map[string]string, defaultValue string, disabledAffinityAssistant bool, feature *string) error {
	value := defaultValue
	if cfg, ok := cfgMap[coscheduleKey]; ok {
		value = strings.ToLower(cfg)
	}

	switch value {
	case CoscheduleDisabled, CoscheduleWorkspaces, CoschedulePipelineRuns, CoscheduleIsolatePipelineRun:
		// validate that "coschedule" is compatible with "disable-affinity-assistant"
		// "coschedule" must be set to "workspaces" when "disable-affinity-assistant" is false
		if !disabledAffinityAssistant && value != CoscheduleWorkspaces {
			return fmt.Errorf("coschedule value %v is incompatible with %v setting to false", value, disableAffinityAssistantKey)
		}
		*feature = value
	default:
		return fmt.Errorf("invalid value for feature flag %q: %q", coscheduleKey, value)
	}

	return nil
}

// setEnforceNonFalsifiability sets the "enforce-nonfalsifiability" flag based on the content of a given map.
// If the feature gate is invalid, then an error is returned.
func setEnforceNonFalsifiability(cfgMap map[string]string, feature *string) error {
	var value = DefaultEnforceNonfalsifiability
	if cfg, ok := cfgMap[enforceNonfalsifiability]; ok {
		value = strings.ToLower(cfg)
	}

	// validate that "enforce-nonfalsifiability" is set to a valid value
	switch value {
	case EnforceNonfalsifiabilityNone, EnforceNonfalsifiabilityWithSpire:
		*feature = value
		return nil
	default:
		return fmt.Errorf("invalid value for feature flag %q: %q", enforceNonfalsifiability, value)
	}
}

// setResultExtractionMethod sets the "results-from" flag based on the content of a given map.
// If the feature gate is invalid or missing then an error is returned.
func setResultExtractionMethod(cfgMap map[string]string, defaultValue string, feature *string) error {
	value := defaultValue
	if cfg, ok := cfgMap[resultExtractionMethod]; ok {
		value = strings.ToLower(cfg)
	}
	switch value {
	case ResultExtractionMethodTerminationMessage, ResultExtractionMethodSidecarLogs:
		*feature = value
	default:
		return fmt.Errorf("invalid value for feature flag %q: %q", resultExtractionMethod, value)
	}
	return nil
}

// setMaxResultSize sets the "max-result-size" flag based on the content of a given map.
// If the feature gate is invalid or missing then an error is returned.
func setMaxResultSize(cfgMap map[string]string, defaultValue int, feature *int) error {
	value := defaultValue
	if cfg, ok := cfgMap[maxResultSize]; ok {
		v, err := strconv.Atoi(cfg)
		if err != nil {
			return err
		}
		value = v
	}
	// if max limit is > 1.5 MB (CRD limit).
	if value >= 1572864 {
		return fmt.Errorf("invalid value for feature flag %q: %q. This is exceeding the CRD limit", resultExtractionMethod, fmt.Sprint(value))
	}
	*feature = value
	return nil
}

// setVerificationNoMatchPolicy sets the "trusted-resources-verification-no-match-policy" flag based on the content of a given map.
// If the value is invalid or missing then an error is returned.
func setVerificationNoMatchPolicy(cfgMap map[string]string, defaultValue string, feature *string) error {
	value := defaultValue
	if cfg, ok := cfgMap[verificationNoMatchPolicy]; ok {
		value = strings.ToLower(cfg)
	}
	switch value {
	case FailNoMatchPolicy, WarnNoMatchPolicy, IgnoreNoMatchPolicy:
		*feature = value
	default:
		return fmt.Errorf("invalid value for feature flag %q: %q", verificationNoMatchPolicy, value)
	}
	return nil
}

// NewFeatureFlagsFromConfigMap returns a Config for the given configmap
func NewFeatureFlagsFromConfigMap(config *corev1.ConfigMap) (*FeatureFlags, error) {
	return NewFeatureFlagsFromMap(config.Data)
}

// GetVerificationNoMatchPolicy returns the "trusted-resources-verification-no-match-policy" value
func GetVerificationNoMatchPolicy(ctx context.Context) string {
	return FromContextOrDefaults(ctx).FeatureFlags.VerificationNoMatchPolicy
}

// IsSpireEnabled checks if non-falsifiable provenance is enforced through SPIRE
func IsSpireEnabled(ctx context.Context) bool {
	return FromContextOrDefaults(ctx).FeatureFlags.EnforceNonfalsifiability == EnforceNonfalsifiabilityWithSpire
}

// TODO(#7285): Patch the default values of new features that were added after
// `enable-api-fields` was no longer used.
type PerFeatureFlag struct {
	// Name of the feature flag
	Name string
	// Stability level of the feature, one of StableAPIFields, BetaAPIFields or AlphaAPIFields
	Stability string
	// Enabled is whether the feature is turned on
	Enabled bool
	// Deprecated indicates whether the feature is deprecated
	// +optional
	//nolint:gocritic
	Deprecated bool
}
