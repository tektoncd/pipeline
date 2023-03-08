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

package resolver

import (
	"fmt"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	// DefaultEnableGitResolver is the default value for "enable-git-resolver".
	DefaultEnableGitResolver = true
	// DefaultEnableHubResolver is the default value for "enable-hub-resolver".
	DefaultEnableHubResolver = true
	// DefaultEnableBundlesResolver is the default value for "enable-bundles-resolver".
	DefaultEnableBundlesResolver = true
	// DefaultEnableClusterResolver is the default value for "enable-cluster-resolver".
	DefaultEnableClusterResolver = true

	// EnableGitResolver is the flag used to enable the git remote resolver
	EnableGitResolver = "enable-git-resolver"
	// EnableHubResolver is the flag used to enable the hub remote resolver
	EnableHubResolver = "enable-hub-resolver"
	// EnableBundlesResolver is the flag used to enable the bundle remote resolver
	EnableBundlesResolver = "enable-bundles-resolver"
	// EnableClusterResolver is the flag used to enable the cluster remote resolver
	EnableClusterResolver = "enable-cluster-resolver"
)

// FeatureFlags holds the features configurations
// +k8s:deepcopy-gen=true
type FeatureFlags struct {
	EnableGitResolver     bool
	EnableHubResolver     bool
	EnableBundleResolver  bool
	EnableClusterResolver bool
}

// GetFeatureFlagsConfigName returns the name of the configmap containing all
// feature flags.
func GetFeatureFlagsConfigName() string {
	if e := os.Getenv("CONFIG_RESOLVERS_FEATURE_FLAGS_NAME"); e != "" {
		return e
	}
	return "resolvers-feature-flags"
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
	if err := setFeature(EnableGitResolver, DefaultEnableGitResolver, &tc.EnableGitResolver); err != nil {
		return nil, err
	}
	if err := setFeature(EnableHubResolver, DefaultEnableHubResolver, &tc.EnableHubResolver); err != nil {
		return nil, err
	}
	if err := setFeature(EnableBundlesResolver, DefaultEnableBundlesResolver, &tc.EnableBundleResolver); err != nil {
		return nil, err
	}
	if err := setFeature(EnableClusterResolver, DefaultEnableClusterResolver, &tc.EnableClusterResolver); err != nil {
		return nil, err
	}
	return &tc, nil
}

// NewFeatureFlagsFromConfigMap returns a Config for the given configmap
func NewFeatureFlagsFromConfigMap(config *corev1.ConfigMap) (*FeatureFlags, error) {
	return NewFeatureFlagsFromMap(config.Data)
}
