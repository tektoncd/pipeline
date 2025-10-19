package cache

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	"knative.dev/pkg/configmap"
)

const (
	// TODO(twoGiants): add docs for this env var and default config map name
	resolverCacheConfigMapNameEnv = "RESOLVER_CACHE_CONFIG_MAP_NAME"
	defaultConfigMapName          = "resolver-cache-config"
	maxSizeConfigMapKey           = "max-size"
	ttlConfigMapKey               = "ttl"
	defaultCacheSize              = 1000
	defaultExpiration             = 5 * time.Minute
)

var (
	cacheMu           sync.Mutex
	startWatchingOnce sync.Once
)

type cacheConfigKey struct{}

type cacheConfig struct {
	maxSize int
	ttl     time.Duration
}

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
				getCacheConfigName(): parseCacheConfigMap,
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

func (store *CacheConfigStore) GetResolverConfig() cacheConfig {
	untypedConf := store.untyped.UntypedLoad(store.cacheConfigName)
	if cacheConf, ok := untypedConf.(cacheConfig); ok {
		return cacheConf
	}

	return cacheConfig{}
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

func parseCacheConfigMap(cm *corev1.ConfigMap) (*cacheConfig, error) {
	config := &cacheConfig{
		maxSize: defaultCacheSize,
		ttl:     defaultExpiration,
	}

	if cm == nil {
		return config, nil
	}

	if maxSizeStr, ok := cm.Data[maxSizeConfigMapKey]; ok {
		if parsed, err := strconv.Atoi(maxSizeStr); err == nil && parsed > 0 {
			config.maxSize = parsed
		}
	}

	if ttlStr, ok := cm.Data[ttlConfigMapKey]; ok {
		if parsed, err := time.ParseDuration(ttlStr); err == nil && parsed > 0 {
			config.ttl = parsed
		}
	}

	return config, nil
}

func onCacheConfigChanged(_ string, value any) {
	config, ok := value.(*cacheConfig)
	if !ok {
		return
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	sharedCache = newResolverCache(config.maxSize, config.ttl)
}
