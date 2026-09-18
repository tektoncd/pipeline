/*
Copyright 2026 The Tekton Authors

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

package namespace

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"sigs.k8s.io/yaml"
)

const (
	namespaceConfigLabel               = "tekton.dev/pipeline-config"
	partOfLabel                        = "app.kubernetes.io/part-of"
	partOfValue                        = "tekton-pipelines"
	namespaceFeatureFlagsConfigMapName = "tekton-feature-flags"
	namespaceDefaultsConfigMapName     = "tekton-config-defaults"
	configValueTrue                    = "true"
	exampleConfigKey                   = "_example"

	defaultTimeoutMinutesKey                = "default-timeout-minutes"
	defaultServiceAccountKey                = "default-service-account"
	defaultManagedByLabelValueKey           = "default-managed-by-label-value"
	defaultPodTemplateKey                   = "default-pod-template"
	defaultAAPodTemplateKey                 = "default-affinity-assistant-pod-template"
	defaultCloudEventsSinkKey               = "default-cloud-events-sink"
	defaultTaskRunWorkspaceBindingKey       = "default-task-run-workspace-binding"
	defaultMaxMatrixCombinationsCountKey    = "default-max-matrix-combinations-count"
	defaultForbiddenEnvKey                  = "default-forbidden-env"
	defaultResolverTypeKey                  = "default-resolver-type"
	defaultContainerResourceRequirementsKey = "default-container-resource-requirements"
	defaultImagePullBackOffTimeoutKey       = "default-imagepullbackoff-timeout"
	defaultCreateContainerErrorTimeoutKey   = "default-create-container-error-timeout"
	defaultMaximumResolutionTimeoutKey      = "default-maximum-resolution-timeout"
	defaultSidecarLogPollingIntervalKey     = "default-sidecar-log-polling-interval"
	defaultStepRefConcurrencyLimitKey       = "default-step-ref-concurrency-limit"

	disableCredsInitKey                         = "disable-creds-init"
	runningInEnvWithInjectedSidecarsKey         = "running-in-environment-with-injected-sidecars"
	requireGitSSHSecretKnownHostsKey            = "require-git-ssh-secret-known-hosts" //nolint:gosec
	enableAPIFieldsKey                          = "enable-api-fields"
	sendCloudEventsForRunsKey                   = "send-cloudevents-for-runs"
	awaitSidecarReadinessKey                    = "await-sidecar-readiness"
	enforceNonfalsifiabilityKey                 = "enforce-nonfalsifiability"
	verificationNoMatchPolicyKey                = "trusted-resources-verification-no-match-policy"
	enableProvenanceInStatusKey                 = "enable-provenance-in-status"
	resultsFromKey                              = "results-from"
	maxResultSizeKey                            = "max-result-size"
	setSecurityContextKey                       = "set-security-context"
	setSecurityContextReadOnlyRootFilesystemKey = "set-security-context-read-only-root-filesystem"
	coscheduleKey                               = "coschedule"
	keepPodOnCancelKey                          = "keep-pod-on-cancel"
	enableCELInWhenExpressionKey                = "enable-cel-in-whenexpression"
	enableStepActionsKey                        = "enable-step-actions"
	enableArtifactsKey                          = "enable-artifacts"
	enableParamEnumKey                          = "enable-param-enum"
	disableInlineSpecKey                        = "disable-inline-spec"
	enableConciseResolverSyntaxKey              = "enable-concise-resolver-syntax"
	enableKubernetesSidecarKey                  = "enable-kubernetes-sidecar"
	enableWaitExponentialBackoffKey             = "enable-wait-exponential-backoff"
	enableTerminationMessageCompressionKey      = "enable-termination-message-compression"
	perNamespaceConfigurationKey                = "per-namespace-configuration"
	nonOverridableFieldsKey                     = "non-overridable-fields"
	deprecatedEnableTektonOCIBundlesKey         = "enable-tekton-oci-bundles"
)

var defaultsAllowList = map[string]bool{
	defaultServiceAccountKey:                true,
	defaultTimeoutMinutesKey:                true,
	defaultManagedByLabelValueKey:           true,
	defaultPodTemplateKey:                   true,
	defaultTaskRunWorkspaceBindingKey:       true,
	defaultMaxMatrixCombinationsCountKey:    true,
	defaultImagePullBackOffTimeoutKey:       true,
	defaultContainerResourceRequirementsKey: true,
}

var featureFlagsAllowList = map[string]bool{
	runningInEnvWithInjectedSidecarsKey: true,
	awaitSidecarReadinessKey:            true,
	maxResultSizeKey:                    true,
	coscheduleKey:                       true,
	keepPodOnCancelKey:                  true,
	enableCELInWhenExpressionKey:        true,
	enableArtifactsKey:                  true,
	enableParamEnumKey:                  true,
	enableKubernetesSidecarKey:          true,
	enableWaitExponentialBackoffKey:     true,
}

var blockList = map[string]bool{
	enforceNonfalsifiabilityKey:                 true,
	setSecurityContextKey:                       true,
	setSecurityContextReadOnlyRootFilesystemKey: true,
	verificationNoMatchPolicyKey:                true,
	defaultForbiddenEnvKey:                      true,
	disableCredsInitKey:                         true,
	disableInlineSpecKey:                        true,
	enableAPIFieldsKey:                          true,
	resultsFromKey:                              true,
	defaultSidecarLogPollingIntervalKey:         true,
	defaultStepRefConcurrencyLimitKey:           true,
	perNamespaceConfigurationKey:                true,
	nonOverridableFieldsKey:                     true,
}

var excludedNamespaces = map[string]bool{
	"kube-system": true,
	"kube-public": true,
}

type namespaceConfig struct {
	defaults map[string]string
	flags    map[string]string
}

// PerNamespaceConfig merges labeled namespace ConfigMaps with the global config.
type PerNamespaceConfig struct {
	configMapLister corev1listers.ConfigMapLister
}

// NewPerNamespaceConfig starts and syncs a filtered ConfigMap informer.
func NewPerNamespaceConfig(ctx context.Context, kubeClient kubernetes.Interface) *PerNamespaceConfig {
	selector := labels.Set{
		namespaceConfigLabel: configValueTrue,
		partOfLabel:          partOfValue,
	}.AsSelector().String()
	factory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 0,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector
		}),
	)
	configMaps := factory.Core().V1().ConfigMaps()
	_ = configMaps.Informer()
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	return &PerNamespaceConfig{configMapLister: configMaps.Lister()}
}

// MergeGlobalConfigWithLocal returns a context containing the namespace overrides.
// It returns the original context when the feature is disabled, the namespace is
// excluded, or no matching ConfigMaps exist. Invalid values are returned as errors.
func (p *PerNamespaceConfig) MergeGlobalConfigWithLocal(ctx context.Context, namespace string) (context.Context, error) {
	if p == nil || namespace == system.Namespace() || excludedNamespaces[namespace] {
		return ctx, nil
	}
	cfg := config.FromContextOrDefaults(ctx)
	if cfg.FeatureFlags == nil || !cfg.FeatureFlags.PerNamespaceConfiguration {
		return ctx, nil
	}
	local := p.get(namespace)
	if local == nil {
		return ctx, nil
	}

	logger := logging.FromContext(ctx)
	operatorBlockList := parseOperatorBlockList(cfg.FeatureFlags.NonOverridableFields)
	merged := cfg.DeepCopy()

	if len(local.defaults) > 0 {
		globalConfig, err := configToDefaultsMap(cfg.Defaults)
		if err != nil {
			return ctx, fmt.Errorf("serialize global defaults: %w", err)
		}
		mergedConfig := mergeConfigMaps(globalConfig, local.defaults, defaultsAllowList, operatorBlockList, logger)
		mergedDefaults, err := config.NewDefaultsFromMap(mergedConfig)
		if err != nil {
			return ctx, fmt.Errorf("parse namespace defaults for %q: %w", namespace, err)
		}
		merged.Defaults = mergedDefaults
		logOverrides(logger, namespace, "config-defaults", local.defaults, defaultsAllowList, operatorBlockList)
	}

	if len(local.flags) > 0 {
		globalConfig := configToFeatureFlagsMap(cfg.FeatureFlags)
		mergedConfig := mergeConfigMaps(globalConfig, local.flags, featureFlagsAllowList, operatorBlockList, logger)
		mergedFlags, err := config.NewFeatureFlagsFromMap(mergedConfig)
		if err != nil {
			return ctx, fmt.Errorf("parse namespace feature flags for %q: %w", namespace, err)
		}
		mergedFlags.PerNamespaceConfiguration = cfg.FeatureFlags.PerNamespaceConfiguration
		mergedFlags.NonOverridableFields = cfg.FeatureFlags.NonOverridableFields
		merged.FeatureFlags = mergedFlags
		logOverrides(logger, namespace, "feature-flags", local.flags, featureFlagsAllowList, operatorBlockList)
	}

	return config.ToContext(ctx, merged), nil
}

func (p *PerNamespaceConfig) get(namespace string) *namespaceConfig {
	local := &namespaceConfig{}
	found := false
	if defaults := p.getConfigMap(namespace, namespaceDefaultsConfigMapName); defaults != nil {
		local.defaults = defaults.Data
		found = true
	}
	if flags := p.getConfigMap(namespace, namespaceFeatureFlagsConfigMapName); flags != nil {
		local.flags = flags.Data
		found = true
	}
	if !found {
		return nil
	}
	return local
}

func (p *PerNamespaceConfig) getConfigMap(namespace, name string) *corev1.ConfigMap {
	if p.configMapLister == nil {
		return nil
	}
	cm, err := p.configMapLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		return nil
	}
	if cm.Labels[namespaceConfigLabel] != configValueTrue || cm.Labels[partOfLabel] != partOfValue {
		return nil
	}
	return cm
}

func mergeConfigMaps(globalConfig, localConfig map[string]string, allowList, operatorBlockList map[string]bool, logger *zap.SugaredLogger) map[string]string {
	merged := make(map[string]string, len(globalConfig))
	for key, value := range globalConfig {
		merged[key] = value
	}
	for key, value := range localConfig {
		switch {
		case key == exampleConfigKey:
			continue
		case blockList[key]:
			logger.Warnf("Namespace config attempted to override non-overridable field %q, ignoring", key)
		case operatorBlockList[key]:
			logger.Warnf("Namespace config attempted to override operator-locked field %q, ignoring", key)
		case allowList[key]:
			merged[key] = value
		default:
			logger.Warnf("Namespace config contains unknown or non-overridable field %q, ignoring", key)
		}
	}
	return merged
}

func parseOperatorBlockList(value string) map[string]bool {
	if value == "" {
		return nil
	}
	result := make(map[string]bool)
	for _, field := range strings.Split(value, ",") {
		if field := strings.TrimSpace(field); field != "" {
			result[field] = true
		}
	}
	return result
}

func logOverrides(logger *zap.SugaredLogger, namespace, configMap string, localConfig map[string]string, allowList, operatorBlockList map[string]bool) {
	var overridden []string
	for key := range localConfig {
		if key != exampleConfigKey && !blockList[key] && !operatorBlockList[key] && allowList[key] {
			overridden = append(overridden, key)
		}
	}
	if len(overridden) == 0 {
		return
	}
	sort.Strings(overridden)
	logger.Infof("Applying namespace config for %q: overriding %s fields: %s", namespace, configMap, strings.Join(overridden, ", "))
}

func configToDefaultsMap(defaults *config.Defaults) (map[string]string, error) {
	if defaults == nil {
		return map[string]string{}, nil
	}
	result := map[string]string{
		defaultTimeoutMinutesKey:              strconv.Itoa(defaults.DefaultTimeoutMinutes),
		defaultServiceAccountKey:              defaults.DefaultServiceAccount,
		defaultManagedByLabelValueKey:         defaults.DefaultManagedByLabelValue,
		defaultCloudEventsSinkKey:             defaults.DefaultCloudEventsSink,
		defaultMaxMatrixCombinationsCountKey:  strconv.Itoa(defaults.DefaultMaxMatrixCombinationsCount),
		defaultResolverTypeKey:                defaults.DefaultResolverType,
		defaultImagePullBackOffTimeoutKey:     defaults.DefaultImagePullBackOffTimeout.String(),
		defaultCreateContainerErrorTimeoutKey: defaults.DefaultCreateContainerErrorTimeout.String(),
		defaultMaximumResolutionTimeoutKey:    defaults.DefaultMaximumResolutionTimeout.String(),
		defaultSidecarLogPollingIntervalKey:   defaults.DefaultSidecarLogPollingInterval.String(),
		defaultStepRefConcurrencyLimitKey:     strconv.Itoa(defaults.DefaultStepRefConcurrencyLimit),
	}
	if defaults.DefaultTaskRunWorkspaceBinding != "" {
		result[defaultTaskRunWorkspaceBindingKey] = defaults.DefaultTaskRunWorkspaceBinding
	}
	if len(defaults.DefaultForbiddenEnv) > 0 {
		result[defaultForbiddenEnvKey] = strings.Join(defaults.DefaultForbiddenEnv, ",")
	}
	if defaults.DefaultPodTemplate != nil {
		if err := setYAMLValue(result, defaultPodTemplateKey, defaults.DefaultPodTemplate); err != nil {
			return nil, err
		}
	}
	if defaults.DefaultAAPodTemplate != nil {
		if err := setYAMLValue(result, defaultAAPodTemplateKey, defaults.DefaultAAPodTemplate); err != nil {
			return nil, err
		}
	}
	if len(defaults.DefaultContainerResourceRequirements) > 0 {
		if err := setYAMLValue(result, defaultContainerResourceRequirementsKey, defaults.DefaultContainerResourceRequirements); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func setYAMLValue[T *pod.Template | *pod.AffinityAssistantTemplate | map[string]corev1.ResourceRequirements](values map[string]string, key string, value T) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", key, err)
	}
	values[key] = string(data)
	return nil
}

func configToFeatureFlagsMap(flags *config.FeatureFlags) map[string]string {
	if flags == nil {
		return map[string]string{}
	}
	result := map[string]string{
		disableCredsInitKey:                         strconv.FormatBool(flags.DisableCredsInit),
		runningInEnvWithInjectedSidecarsKey:         strconv.FormatBool(flags.RunningInEnvWithInjectedSidecars),
		requireGitSSHSecretKnownHostsKey:            strconv.FormatBool(flags.RequireGitSSHSecretKnownHosts),
		enableAPIFieldsKey:                          flags.EnableAPIFields,
		sendCloudEventsForRunsKey:                   strconv.FormatBool(flags.SendCloudEventsForRuns),
		awaitSidecarReadinessKey:                    strconv.FormatBool(flags.AwaitSidecarReadiness),
		enforceNonfalsifiabilityKey:                 flags.EnforceNonfalsifiability,
		verificationNoMatchPolicyKey:                flags.VerificationNoMatchPolicy,
		enableProvenanceInStatusKey:                 strconv.FormatBool(flags.EnableProvenanceInStatus),
		resultsFromKey:                              flags.ResultExtractionMethod,
		maxResultSizeKey:                            strconv.Itoa(flags.MaxResultSize),
		setSecurityContextKey:                       strconv.FormatBool(flags.SetSecurityContext),
		setSecurityContextReadOnlyRootFilesystemKey: strconv.FormatBool(flags.SetSecurityContextReadOnlyRootFilesystem),
		coscheduleKey:                               flags.Coschedule,
		keepPodOnCancelKey:                          strconv.FormatBool(flags.EnableKeepPodOnCancel),
		enableCELInWhenExpressionKey:                strconv.FormatBool(flags.EnableCELInWhenExpression),
		enableStepActionsKey:                        strconv.FormatBool(flags.EnableStepActions),
		enableArtifactsKey:                          strconv.FormatBool(flags.EnableArtifacts),
		enableParamEnumKey:                          strconv.FormatBool(flags.EnableParamEnum),
		disableInlineSpecKey:                        flags.DisableInlineSpec,
		enableConciseResolverSyntaxKey:              strconv.FormatBool(flags.EnableConciseResolverSyntax),
		enableKubernetesSidecarKey:                  strconv.FormatBool(flags.EnableKubernetesSidecar),
		enableWaitExponentialBackoffKey:             strconv.FormatBool(flags.EnableWaitExponentialBackoff),
		enableTerminationMessageCompressionKey:      strconv.FormatBool(flags.EnableTerminationMessageCompression),
		perNamespaceConfigurationKey:                strconv.FormatBool(flags.PerNamespaceConfiguration),
		nonOverridableFieldsKey:                     flags.NonOverridableFields,
	}
	if flags.DeprecatedEnableTektonOCIBundles != nil {
		result[deprecatedEnableTektonOCIBundlesKey] = strconv.FormatBool(*flags.DeprecatedEnableTektonOCIBundles)
	}
	return result
}
