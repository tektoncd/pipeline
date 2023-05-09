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
	"context"

	sc "github.com/tektoncd/pipeline/pkg/spire/config"
	"knative.dev/pkg/configmap"
)

type cfgKey struct{}

// Config holds the collection of configurations that we attach to contexts.
// +k8s:deepcopy-gen=false
type Config struct {
	Defaults     *Defaults
	FeatureFlags *FeatureFlags
	Metrics      *Metrics
	SpireConfig  *sc.SpireConfig
}

// FromContext extracts a Config from the provided context.
func FromContext(ctx context.Context) *Config {
	x, ok := ctx.Value(cfgKey{}).(*Config)
	if ok {
		return x
	}
	return nil
}

// FromContextOrDefaults is like FromContext, but when no Config is attached it
// returns a Config populated with the defaults for each of the Config fields.
func FromContextOrDefaults(ctx context.Context) *Config {
	if cfg := FromContext(ctx); cfg != nil {
		return cfg
	}

	return &Config{
		Defaults:     DefaultConfig.DeepCopy(),
		FeatureFlags: DefaultFeatureFlags.DeepCopy(),
		Metrics:      DefaultMetrics.DeepCopy(),
		SpireConfig:  DefaultSpire.DeepCopy(),
	}
}

// ToContext attaches the provided Config to the provided context, returning the
// new context with the Config attached.
func ToContext(ctx context.Context, c *Config) context.Context {
	return context.WithValue(ctx, cfgKey{}, c)
}

// Store is a typed wrapper around configmap.Untyped store to handle our configmaps.
// +k8s:deepcopy-gen=false
type Store struct {
	*configmap.UntypedStore
}

// NewStore creates a new store of Configs and optionally calls functions when ConfigMaps are updated.
func NewStore(logger configmap.Logger, onAfterStore ...func(name string, value interface{})) *Store {
	store := &Store{
		UntypedStore: configmap.NewUntypedStore(
			"defaults/features/artifacts",
			logger,
			configmap.Constructors{
				GetDefaultsConfigName():     NewDefaultsFromConfigMap,
				GetFeatureFlagsConfigName(): NewFeatureFlagsFromConfigMap,
				GetMetricsConfigName():      NewMetricsFromConfigMap,
				GetSpireConfigName():        NewSpireConfigFromConfigMap,
			},
			onAfterStore...,
		),
	}

	return store
}

// ToContext attaches the current Config state to the provided context.
func (s *Store) ToContext(ctx context.Context) context.Context {
	return ToContext(ctx, s.Load())
}

// Load creates a Config from the current config state of the Store.
func (s *Store) Load() *Config {
	defaults := s.UntypedLoad(GetDefaultsConfigName())
	if defaults == nil {
		defaults = DefaultConfig.DeepCopy()
	}
	featureFlags := s.UntypedLoad(GetFeatureFlagsConfigName())
	if featureFlags == nil {
		featureFlags = DefaultFeatureFlags.DeepCopy()
	}
	metrics := s.UntypedLoad(GetMetricsConfigName())
	if metrics == nil {
		metrics = DefaultMetrics.DeepCopy()
	}

	spireconfig := s.UntypedLoad(GetSpireConfigName())
	if spireconfig == nil {
		spireconfig = DefaultSpire.DeepCopy()
	}

	return &Config{
		Defaults:     defaults.(*Defaults).DeepCopy(),
		FeatureFlags: featureFlags.(*FeatureFlags).DeepCopy(),
		Metrics:      metrics.(*Metrics).DeepCopy(),
		SpireConfig:  spireconfig.(*sc.SpireConfig).DeepCopy(),
	}
}
