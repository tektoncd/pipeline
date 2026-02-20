/*
Copyright 2025 The Tekton Authors

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

package cache

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"knative.dev/pkg/configmap"
)

const (
	// resolverCacheConfigMapNameEnv env var overwrites the cache ConfigMap name
	// defaults to "resolver-cache-config"
	resolverCacheConfigMapNameEnv = "RESOLVER_CACHE_CONFIG_MAP_NAME"
	// defaultConfigMapName is the default name of the ConfigMap that configures resolver cache settings
	// the ConfigMap contains max-size and ttl configuration for the shared resolver cache
	defaultConfigMapName = "resolver-cache-config"
	maxSizeConfigMapKey  = "max-size"
	ttlConfigMapKey      = "ttl"
	defaultCacheSize     = 1000
	defaultExpiration    = 5 * time.Minute
)

var (
	cacheMu           sync.Mutex
	startWatchingOnce sync.Once
)

type cacheConfigKey struct{}

type CacheConfigStore struct {
	untyped         *configmap.UntypedStore
	cacheConfigName string
}

func NewCacheConfigStore(cacheConfigName string, logger configmap.Logger) *CacheConfigStore {
	return &CacheConfigStore{
		cacheConfigName: cacheConfigName,
		untyped: configmap.NewUntypedStore(
			defaultConfigMapName,
			logger,
			configmap.Constructors{
				getCacheConfigName(): resolutionframework.DataFromConfigMap,
			},
			onCacheConfigChanged,
		),
	}
}

func (store *CacheConfigStore) WatchConfigs(w configmap.Watcher) {
	startWatchingOnce.Do(func() {
		store.untyped.WatchConfigs(w)
	})
}

func (store *CacheConfigStore) GetResolverConfig() map[string]string {
	resolverConfig := map[string]string{}
	untypedConf := store.untyped.UntypedLoad(store.cacheConfigName)
	if conf, ok := untypedConf.(map[string]string); ok {
		for key, val := range conf {
			resolverConfig[key] = val
		}
	}
	return resolverConfig
}

// ToContext returns a new context with the cache's configuration
// data stored in it.
func (store *CacheConfigStore) ToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheConfigKey{}, store.GetResolverConfig())
}

// getCacheConfigName returns the name of the cache configuration ConfigMap.
// This can be overridden via the cacheConfigEnv environment variable.
func getCacheConfigName() string {
	if configMapName := os.Getenv(resolverCacheConfigMapNameEnv); configMapName != "" {
		return configMapName
	}

	return defaultConfigMapName
}

func onCacheConfigChanged(_ string, value any) {
	conf, ok := value.(map[string]string)
	if !ok {
		return
	}

	maxSize := defaultCacheSize
	if maxSizeStr, ok := conf[maxSizeConfigMapKey]; ok {
		if parsed, err := strconv.Atoi(maxSizeStr); err == nil && parsed > 0 {
			maxSize = parsed
		}
	}

	ttl := defaultExpiration
	if ttlStr, ok := conf[ttlConfigMapKey]; ok {
		if parsed, err := time.ParseDuration(ttlStr); err == nil && parsed > 0 {
			ttl = parsed
		}
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	sharedCache = newResolverCache(maxSize, ttl)
}
