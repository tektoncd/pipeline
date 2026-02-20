//go:build !disable_tls

package config

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/metrics"
)

// GetMetricsConfigName returns the name of the configmap containing all
// customizations for the storage bucket.
func GetMetricsConfigName() string {
	return metrics.ConfigMapName()
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
