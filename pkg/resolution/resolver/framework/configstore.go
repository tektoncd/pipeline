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

package framework

import (
	"context"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/configmap"
)

// resolverConfigKey is the contenxt key associated with configuration
// for one specific resolver, and is only used if that resolver
// implements the optional framework.ConfigWatcher interface.
var resolverConfigKey = struct{}{}

// DataFromConfigMap returns a copy of the contents of a configmap or an
// empty map if the configmap doesn't have any data.
func DataFromConfigMap(config *corev1.ConfigMap) (map[string]string, error) {
	resolverConfig := map[string]string{}
	if config == nil {
		return resolverConfig, nil
	}
	for key, value := range config.Data {
		resolverConfig[key] = value
	}
	return resolverConfig, nil
}

// ConfigStore wraps a knative untyped store and provides helper methods
// for working with a resolver's configuration data.
type ConfigStore struct {
	*resolverconfig.Store
	resolverConfigName string
	untyped            *configmap.UntypedStore
}

// NewConfigStore creates a new untyped store for the resolver's configuration and a config.Store for general Pipeline configuration.
func NewConfigStore(resolverConfigName string, logger configmap.Logger) *ConfigStore {
	return &ConfigStore{
		Store:              resolverconfig.NewStore(logger),
		resolverConfigName: resolverConfigName,
		untyped: configmap.NewUntypedStore(
			"resolver-config",
			logger,
			configmap.Constructors{
				resolverConfigName: DataFromConfigMap,
			},
		),
	}
}

// WatchConfigs uses the provided configmap.Watcher
// to setup watches for the config names provided in the
// Constructors map
func (store *ConfigStore) WatchConfigs(w configmap.Watcher) {
	store.untyped.WatchConfigs(w)
	store.Store.WatchConfigs(w)
}

// GetResolverConfig returns a copy of the resolver's current
// configuration or an empty map if the stored config is nil or invalid.
func (store *ConfigStore) GetResolverConfig() map[string]string {
	resolverConfig := map[string]string{}
	untypedConf := store.untyped.UntypedLoad(store.resolverConfigName)
	if conf, ok := untypedConf.(map[string]string); ok {
		for key, val := range conf {
			resolverConfig[key] = val
		}
	}
	return resolverConfig
}

// ToContext returns a new context with the resolver's configuration
// data stored in it.
func (store *ConfigStore) ToContext(ctx context.Context) context.Context {
	conf := store.GetResolverConfig()
	return InjectResolverConfigToContext(store.Store.ToContext(ctx), conf)
}

// InjectResolverConfigToContext returns a new context with a
// map stored in it for a resolvers config.
func InjectResolverConfigToContext(ctx context.Context, conf map[string]string) context.Context {
	return context.WithValue(ctx, resolverConfigKey, conf)
}

// GetResolverConfigFromContext returns any resolver-specific
// configuration that has been stored or an empty map if none exists.
func GetResolverConfigFromContext(ctx context.Context) map[string]string {
	conf := map[string]string{}
	storedConfig := ctx.Value(resolverConfigKey)
	if resolverConfig, ok := storedConfig.(map[string]string); ok {
		conf = resolverConfig
	}
	return conf
}
