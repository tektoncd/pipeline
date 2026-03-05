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
	"strconv"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// nsConfigCacheKey is the context key for storing the NamespaceConfigCache.
type nsConfigCacheKey struct{}

// ToContext adds a NamespaceConfigCache to the context.
func ToContext(ctx context.Context, cache *NamespaceConfigCache) context.Context {
	return context.WithValue(ctx, nsConfigCacheKey{}, cache)
}

// FromContext retrieves the NamespaceConfigCache from the context.
// Returns nil if no cache is stored.
func FromContext(ctx context.Context) *NamespaceConfigCache {
	val := ctx.Value(nsConfigCacheKey{})
	if val == nil {
		return nil
	}
	return val.(*NamespaceConfigCache)
}

const (
	// NamespaceConfigLabel is the label that identifies namespace-scoped Tekton config overrides.
	NamespaceConfigLabel = "tekton.dev/pipeline-config"

	// PartOfLabel is the standard Kubernetes recommended label for identifying part-of relationships.
	PartOfLabel = "app.kubernetes.io/part-of"

	// PartOfValue is the value for the part-of label for Tekton Pipelines.
	PartOfValue = "tekton-pipelines"

	// NamespaceFeatureFlagsConfigMapName is the name of the namespace-level feature-flags ConfigMap.
	NamespaceFeatureFlagsConfigMapName = "tekton-feature-flags"

	// NamespaceDefaultsConfigMapName is the name of the namespace-level config-defaults ConfigMap.
	NamespaceDefaultsConfigMapName = "tekton-config-defaults"

	// DefaultResyncPeriod is the default resync period for the namespace config informer.
	DefaultResyncPeriod = 10 * time.Minute
)

// OverridableDefaultsFields lists config-defaults fields that can be overridden per namespace.
var OverridableDefaultsFields = map[string]bool{
	"default-service-account":                 true,
	"default-timeout-minutes":                 true,
	"default-managed-by-label-value":          true,
	"default-pod-template":                    true,
	"default-affinity-assistant-pod-template": true,
	"default-task-run-workspace-binding":      true,
	"default-max-matrix-combinations-count":   true,
	"default-resolver-type":                   true,
	"default-imagepullbackoff-timeout":        true,
	"default-container-resource-requirements": true,
	"default-maximum-resolution-timeout":      true,
	"default-cloud-events-sink":               true,
}

// OverridableFeatureFlagsFields lists feature-flags fields that can be overridden per namespace.
var OverridableFeatureFlagsFields = map[string]bool{
	"running-in-environment-with-injected-sidecars": true,
	"await-sidecar-readiness":                       true,
	"require-git-ssh-secret-known-hosts":            true,
	"send-cloudevents-for-runs":                     true,
	"enable-provenance-in-status":                   true,
	"max-result-size":                               true,
	"coschedule":                                    true,
	"keep-pod-on-cancel":                            true,
	"enable-cel-in-whenexpression":                  true,
	"enable-step-actions":                           true,
	"enable-artifacts":                              true,
	"enable-param-enum":                             true,
	"disable-inline-spec":                           true,
	"enable-concise-resolver-syntax":                true,
	"enable-kubernetes-sidecar":                     true,
	"enable-wait-exponential-backoff":               true,
}

// NonOverridableFields lists fields that can NEVER be overridden per namespace, regardless of operator settings.
var NonOverridableFields = map[string]bool{
	// Security-critical fields
	"enforce-nonfalsifiability":                      true,
	"set-security-context":                           true,
	"set-security-context-read-only-root-filesystem": true,
	"trusted-resources-verification-no-match-policy": true,
	"default-forbidden-env":                          true,
	"disable-creds-init":                             true,
	// Stability gate field
	"enable-api-fields": true,
	// Infrastructure fields
	"results-from":                         true,
	"default-sidecar-log-polling-interval": true,
	"default-step-ref-concurrency-limit":   true,
	// Per-namespace config fields themselves
	"per-namespace-configuration": true,
	"namespace-config-cache-size": true,
	"non-overridable-fields":      true,
}

// SystemNamespaces that are excluded from per-namespace configuration.
var SystemNamespaces = map[string]bool{
	"tekton-pipelines": true,
	"kube-system":      true,
	"kube-public":      true,
}

// NamespaceConfig holds the raw and parsed overrides for a single namespace.
type NamespaceConfig struct {
	// RawDefaults holds the raw ConfigMap data from the namespace tekton-config-defaults.
	RawDefaults map[string]string
	// RawFlags holds the raw ConfigMap data from the namespace tekton-feature-flags.
	RawFlags map[string]string
}

// NamespaceConfigCache provides per-namespace configuration lookups backed by
// a cluster-scoped ConfigMap informer filtered by label. The informer's store
// serves as the cache, eliminating the need for an LRU and direct API calls.
type NamespaceConfigCache struct {
	configMapLister corev1listers.ConfigMapLister
}

// NewNamespaceConfigInformer creates a cluster-scoped ConfigMap informer that
// watches only ConfigMaps with the label tekton.dev/pipeline-config=true.
// The caller must start the returned factory (factory.Start) and wait for
// cache sync before using the lister. The informer is registered with the
// factory before returning so that factory.Start will include it.
func NewNamespaceConfigInformer(kubeClient kubernetes.Interface, resyncPeriod time.Duration) (informers.SharedInformerFactory, coreinformers.ConfigMapInformer) {
	labelSelector := labels.Set{NamespaceConfigLabel: "true"}.AsSelector().String()
	factory := informers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}),
	)
	cmInformer := factory.Core().V1().ConfigMaps()
	// Register the informer with the factory so it starts when factory.Start is called.
	_ = cmInformer.Informer()
	return factory, cmInformer
}

// NewNamespaceConfigCache creates a NamespaceConfigCache backed by the given ConfigMapLister.
// The lister should come from a filtered informer that only tracks ConfigMaps
// with the label tekton.dev/pipeline-config=true.
func NewNamespaceConfigCache(lister corev1listers.ConfigMapLister) *NamespaceConfigCache {
	return &NamespaceConfigCache{
		configMapLister: lister,
	}
}

// Get returns the NamespaceConfig for the given namespace by looking up
// ConfigMaps from the informer's cache (the lister). No API calls are made.
// Returns nil (no error) if no namespace config exists.
func (c *NamespaceConfigCache) Get(_ context.Context, namespace string) *NamespaceConfig {
	if SystemNamespaces[namespace] {
		return nil
	}

	nsConfig := &NamespaceConfig{}
	found := false

	// Look up tekton-config-defaults from the lister
	if defaultsCM := c.getConfigMap(namespace, NamespaceDefaultsConfigMapName); defaultsCM != nil {
		nsConfig.RawDefaults = defaultsCM.Data
		found = true
	}

	// Look up tekton-feature-flags from the lister
	if flagsCM := c.getConfigMap(namespace, NamespaceFeatureFlagsConfigMapName); flagsCM != nil {
		nsConfig.RawFlags = flagsCM.Data
		found = true
	}

	if !found {
		return nil
	}

	return nsConfig
}

// getConfigMap retrieves a ConfigMap by namespace and name from the lister and validates labels.
// Returns nil if the ConfigMap doesn't exist or lacks required labels.
func (c *NamespaceConfigCache) getConfigMap(namespace, name string) *corev1.ConfigMap {
	cm, err := c.configMapLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		// The lister only contains labeled ConfigMaps. A "not found" means
		// either the ConfigMap doesn't exist or it lacks the required label.
		return nil
	}

	// Double-check required labels (the informer filter should already ensure
	// the pipeline-config label, but verify part-of too).
	if cm.Labels[PartOfLabel] != PartOfValue {
		return nil
	}

	return cm
}

// LogEventHandlers returns a ResourceEventHandlerFuncs that logs namespace config changes.
func LogEventHandlers(logger *zap.SugaredLogger) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm, ok := obj.(*corev1.ConfigMap)
			if !ok {
				return
			}
			logger.Infof("TEP-0085: Namespace config added: %s/%s", cm.Namespace, cm.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cm, ok := newObj.(*corev1.ConfigMap)
			if !ok {
				return
			}
			logger.Infof("TEP-0085: Namespace config updated: %s/%s", cm.Namespace, cm.Name)
		},
		DeleteFunc: func(obj interface{}) {
			cm, ok := obj.(*corev1.ConfigMap)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				cm, ok = tombstone.Obj.(*corev1.ConfigMap)
				if !ok {
					return
				}
			}
			logger.Infof("TEP-0085: Namespace config deleted: %s/%s", cm.Namespace, cm.Name)
		},
	}
}

// MergeConfigMaps merges global and namespace ConfigMap data, filtering out non-overridable fields.
// operatorLockedFields is the set of additional fields locked by the operator via non-overridable-fields.
func MergeConfigMaps(globalData, namespaceData map[string]string, overridableFields map[string]bool, operatorLockedFields map[string]bool, logger *zap.SugaredLogger) map[string]string {
	merged := make(map[string]string, len(globalData))
	for k, v := range globalData {
		merged[k] = v
	}

	for k, v := range namespaceData {
		if k == "_example" {
			continue
		}
		if NonOverridableFields[k] {
			if logger != nil {
				logger.Warnf("Namespace config attempted to override non-overridable field %q, ignoring", k)
			}
			continue
		}
		if operatorLockedFields != nil && operatorLockedFields[k] {
			if logger != nil {
				logger.Warnf("Namespace config attempted to override operator-locked field %q, ignoring", k)
			}
			continue
		}
		if !overridableFields[k] {
			if logger != nil {
				logger.Warnf("Namespace config contains unknown or non-overridable field %q, ignoring", k)
			}
			continue
		}
		merged[k] = v
	}

	return merged
}

// ParseOperatorLockedFields parses the non-overridable-fields value from the cluster feature-flags.
func ParseOperatorLockedFields(nonOverridableFieldsStr string) map[string]bool {
	if nonOverridableFieldsStr == "" {
		return nil
	}
	result := make(map[string]bool)
	for _, field := range strings.Split(nonOverridableFieldsStr, ",") {
		f := strings.TrimSpace(field)
		if f != "" {
			result[f] = true
		}
	}
	return result
}

// WithNamespaceConfig merges namespace-specific configuration into the context.
// If per-namespace-configuration is disabled or no namespace config exists, the original context is returned.
func WithNamespaceConfig(ctx context.Context, cache *NamespaceConfigCache, namespace string, logger *zap.SugaredLogger) context.Context {
	if cache == nil {
		return ctx
	}

	cfg := config.FromContextOrDefaults(ctx)
	if cfg == nil || cfg.FeatureFlags == nil {
		return ctx
	}

	if !cfg.FeatureFlags.PerNamespaceConfiguration {
		return ctx
	}

	nsConfig := cache.Get(ctx, namespace)
	if nsConfig == nil {
		return ctx
	}

	// Parse operator-locked fields from cluster config
	operatorLockedFields := ParseOperatorLockedFields(cfg.FeatureFlags.NonOverridableFields)

	// Create merged config
	mergedCfg := cfg.DeepCopy()

	// Merge config-defaults
	if nsConfig.RawDefaults != nil && len(nsConfig.RawDefaults) > 0 {
		globalDefaultsData := configToDefaultsMap(cfg.Defaults)
		mergedDefaultsData := MergeConfigMaps(globalDefaultsData, nsConfig.RawDefaults, OverridableDefaultsFields, operatorLockedFields, logger)
		mergedDefaults, err := config.NewDefaultsFromMap(mergedDefaultsData)
		if err != nil {
			if logger != nil {
				logger.Warnf("Failed to parse merged defaults for namespace %q: %v", namespace, err)
			}
		} else {
			mergedCfg.Defaults = mergedDefaults
			if logger != nil {
				overridden := getOverriddenFields(nsConfig.RawDefaults, OverridableDefaultsFields, operatorLockedFields)
				if len(overridden) > 0 {
					logger.Infof("Applying namespace config for %q: overriding config-defaults fields: %s", namespace, strings.Join(overridden, ", "))
				}
			}
		}
	}

	// Merge feature-flags
	if nsConfig.RawFlags != nil && len(nsConfig.RawFlags) > 0 {
		globalFlagsData := configToFeatureFlagsMap(cfg.FeatureFlags)
		mergedFlagsData := MergeConfigMaps(globalFlagsData, nsConfig.RawFlags, OverridableFeatureFlagsFields, operatorLockedFields, logger)
		mergedFlags, err := config.NewFeatureFlagsFromMap(mergedFlagsData)
		if err != nil {
			if logger != nil {
				logger.Warnf("Failed to parse merged feature flags for namespace %q: %v", namespace, err)
			}
		} else {
			// Preserve per-namespace-configuration fields from the cluster config
			mergedFlags.PerNamespaceConfiguration = cfg.FeatureFlags.PerNamespaceConfiguration
			mergedFlags.NamespaceConfigCacheSize = cfg.FeatureFlags.NamespaceConfigCacheSize
			mergedFlags.NonOverridableFields = cfg.FeatureFlags.NonOverridableFields
			mergedCfg.FeatureFlags = mergedFlags
			if logger != nil {
				overridden := getOverriddenFields(nsConfig.RawFlags, OverridableFeatureFlagsFields, operatorLockedFields)
				if len(overridden) > 0 {
					logger.Infof("Applying namespace config for %q: overriding feature-flags fields: %s", namespace, strings.Join(overridden, ", "))
				}
			}
		}
	}

	return config.ToContext(ctx, mergedCfg)
}

// getOverriddenFields returns the list of fields that were actually overridden (for logging).
func getOverriddenFields(nsData map[string]string, overridableFields map[string]bool, operatorLockedFields map[string]bool) []string {
	var overridden []string
	for k := range nsData {
		if k == "_example" {
			continue
		}
		if NonOverridableFields[k] || (operatorLockedFields != nil && operatorLockedFields[k]) {
			continue
		}
		if overridableFields[k] {
			overridden = append(overridden, k)
		}
	}
	return overridden
}

// configToDefaultsMap converts a Defaults struct back to a map[string]string for merging.
func configToDefaultsMap(d *config.Defaults) map[string]string {
	if d == nil {
		return map[string]string{}
	}
	m := map[string]string{
		"default-timeout-minutes":               strconv.Itoa(d.DefaultTimeoutMinutes),
		"default-service-account":               d.DefaultServiceAccount,
		"default-managed-by-label-value":        d.DefaultManagedByLabelValue,
		"default-cloud-events-sink":             d.DefaultCloudEventsSink,
		"default-max-matrix-combinations-count": strconv.Itoa(d.DefaultMaxMatrixCombinationsCount),
		"default-resolver-type":                 d.DefaultResolverType,
		"default-imagepullbackoff-timeout":      d.DefaultImagePullBackOffTimeout.String(),
		"default-maximum-resolution-timeout":    d.DefaultMaximumResolutionTimeout.String(),
		"default-sidecar-log-polling-interval":  d.DefaultSidecarLogPollingInterval.String(),
		"default-step-ref-concurrency-limit":    strconv.Itoa(d.DefaultStepRefConcurrencyLimit),
	}
	if d.DefaultTaskRunWorkspaceBinding != "" {
		m["default-task-run-workspace-binding"] = d.DefaultTaskRunWorkspaceBinding
	}
	if len(d.DefaultForbiddenEnv) > 0 {
		m["default-forbidden-env"] = strings.Join(d.DefaultForbiddenEnv, ",")
	}
	return m
}

// configToFeatureFlagsMap converts a FeatureFlags struct back to a map[string]string for merging.
func configToFeatureFlagsMap(f *config.FeatureFlags) map[string]string {
	if f == nil {
		return map[string]string{}
	}
	return map[string]string{
		"disable-creds-init":                             strconv.FormatBool(f.DisableCredsInit),
		"running-in-environment-with-injected-sidecars":  strconv.FormatBool(f.RunningInEnvWithInjectedSidecars),
		"require-git-ssh-secret-known-hosts":             strconv.FormatBool(f.RequireGitSSHSecretKnownHosts),
		"enable-api-fields":                              f.EnableAPIFields,
		"send-cloudevents-for-runs":                      strconv.FormatBool(f.SendCloudEventsForRuns),
		"await-sidecar-readiness":                        strconv.FormatBool(f.AwaitSidecarReadiness),
		"enforce-nonfalsifiability":                      f.EnforceNonfalsifiability,
		"trusted-resources-verification-no-match-policy": f.VerificationNoMatchPolicy,
		"enable-provenance-in-status":                    strconv.FormatBool(f.EnableProvenanceInStatus),
		"results-from":                                   f.ResultExtractionMethod,
		"max-result-size":                                strconv.FormatInt(int64(f.MaxResultSize), 10),
		"set-security-context":                           strconv.FormatBool(f.SetSecurityContext),
		"set-security-context-read-only-root-filesystem": strconv.FormatBool(f.SetSecurityContextReadOnlyRootFilesystem),
		"coschedule":                                     f.Coschedule,
		"keep-pod-on-cancel":                             strconv.FormatBool(f.EnableKeepPodOnCancel),
		"enable-cel-in-whenexpression":                   strconv.FormatBool(f.EnableCELInWhenExpression),
		"enable-step-actions":                            strconv.FormatBool(f.EnableStepActions),
		"enable-artifacts":                               strconv.FormatBool(f.EnableArtifacts),
		"enable-param-enum":                              strconv.FormatBool(f.EnableParamEnum),
		"disable-inline-spec":                            f.DisableInlineSpec,
		"enable-concise-resolver-syntax":                 strconv.FormatBool(f.EnableConciseResolverSyntax),
		"enable-kubernetes-sidecar":                      strconv.FormatBool(f.EnableKubernetesSidecar),
		"enable-wait-exponential-backoff":                strconv.FormatBool(f.EnableWaitExponentialBackoff),
	}
}
