/*
Copyright 2024 The Tekton Authors

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
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strconv"
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	utilcache "k8s.io/apimachinery/pkg/util/cache"
)

const (
	DefaultCacheSize     = 1000
	defaultConfigMapName = "resolver-cache-config"
)

var (
	defaultExpiration = 5 * time.Minute
)

// ResolverCache is a wrapper around utilcache.LRUExpireCache that provides
// type-safe methods for caching resolver results.
type ResolverCache struct {
	cache  *utilcache.LRUExpireCache
	logger *zap.SugaredLogger
}

// NewResolverCache creates a new ResolverCache with the given expiration time and max size
func NewResolverCache(maxSize int) *ResolverCache {
	return &ResolverCache{
		cache: utilcache.NewLRUExpireCache(maxSize),
	}
}

// WithLogger returns a new ResolverCache instance with the provided logger.
// This prevents state leak by not storing logger in the global singleton.
func (c *ResolverCache) WithLogger(logger *zap.SugaredLogger) *ResolverCache {
	return &ResolverCache{logger: logger, cache: c.cache}
}

// InitializeFromConfigMap initializes the cache with configuration from a ConfigMap.
// This method should be called via injection.InitializeSharedCache which ensures it's only
// called once using sync.Once to prevent recreating the cache and losing cached data.
func (c *ResolverCache) InitializeFromConfigMap(configMap *corev1.ConfigMap) {
	// Set defaults
	maxSize := DefaultCacheSize
	ttl := defaultExpiration

	if configMap != nil {
		// Parse max size
		if maxSizeStr, ok := configMap.Data["max-size"]; ok {
			if parsed, err := strconv.Atoi(maxSizeStr); err == nil && parsed > 0 {
				maxSize = parsed
			}
		}

		// Parse default TTL
		if ttlStr, ok := configMap.Data["default-ttl"]; ok {
			if parsed, err := time.ParseDuration(ttlStr); err == nil && parsed > 0 {
				ttl = parsed
			}
		}
	}

	c.cache = utilcache.NewLRUExpireCache(maxSize)
	defaultExpiration = ttl
}

func (c *ResolverCache) Get(resolverType string, params []pipelinev1.Param) (resolutionframework.ResolvedResource, bool) {
	key := generateCacheKey(resolverType, params)
	value, found := c.cache.Get(key)
	if !found {
		c.infow("Cache miss", "key", key)
		return nil, found
	}

	resource, ok := value.(resolutionframework.ResolvedResource)
	if !ok {
		c.infow("Failed casting cached resource", "key", key)
		return nil, false
	}

	c.infow("Cache hit", "key", key)
	return newAnnotatedResource(resource, resolverType, CacheOperationRetrieve), true
}

func (c *ResolverCache) infow(msg string, keysAndValues ...any) {
	if c.logger != nil {
		c.logger.Infow(msg, keysAndValues...)
	}
}

func (c *ResolverCache) Add(
	resolverType string,
	params []pipelinev1.Param,
	resource resolutionframework.ResolvedResource,
) resolutionframework.ResolvedResource {
	key := generateCacheKey(resolverType, params)
	c.infow("Adding to cache", "key", key, "expiration", defaultExpiration)

	annotatedResource := newAnnotatedResource(resource, resolverType, CacheOperationStore)

	c.cache.Add(key, annotatedResource, defaultExpiration)

	return annotatedResource
}

func (c *ResolverCache) Remove(resolverType string, params []pipelinev1.Param) {
	key := generateCacheKey(resolverType, params)
	c.infow("Removing from cache", "key", key)

	c.cache.Remove(key)
}

// Clear removes all entries from the cache.
func (c *ResolverCache) Clear() {
	c.infow("Clearing all cache entries")
	// predicate that returns true clears all entries
	c.cache.RemoveAll(func(_ any) bool { return true })
}

// TODO(twoGiants): remove and fix tests
// DEPRECATED_Get retrieves a value from the cache.
func (c *ResolverCache) DEPRECATED_Get(key string) (interface{}, bool) {
	value, found := c.cache.Get(key)
	if c.logger != nil {
		if found {
			c.logger.Infow("Cache hit", "key", key)
		} else {
			c.logger.Infow("Cache miss", "key", key)
		}
	}
	return value, found
}

// TODO(twoGiants): remove and fix tests
// DEPRECATED_Add adds a value to the cache with the default expiration time.
func (c *ResolverCache) DEPRECATED_Add(key string, value interface{}) {
	if c.logger != nil {
		c.logger.Infow("Adding to cache", "key", key, "expiration", defaultExpiration)
	}
	c.cache.Add(key, value, defaultExpiration)
}

// TODO(twoGiants): remove and fix tests
// DEPRECATED_Remove removes a value from the cache.
func (c *ResolverCache) DEPRECATED_Remove(key string) {
	if c.logger != nil {
		c.logger.Infow("Removing from cache", "key", key)
	}
	c.cache.Remove(key)
}

// TODO(twoGiants): looks unused => can be deleted, expiration is set via defaultExpiration
// DEPRECATED_AddWithExpiration adds a value to the cache with a custom expiration time
func (c *ResolverCache) DEPRECATED_AddWithExpiration(key string, value interface{}, expiration time.Duration) {
	if c.logger != nil {
		c.logger.Infow("Adding to cache with custom expiration", "key", key, "expiration", expiration)
	}
	c.cache.Add(key, value, expiration)
}

func generateCacheKey(resolverType string, params []pipelinev1.Param) string {
	// Create a deterministic string representation of the parameters
	paramStr := resolverType + ":"

	// Filter out the 'cache' parameter and sort remaining params by name for determinism
	filteredParams := make([]pipelinev1.Param, 0, len(params))
	for _, p := range params {
		if p.Name != "cache" {
			filteredParams = append(filteredParams, p)
		}
	}

	// Sort params by name to ensure deterministic ordering
	sort.Slice(filteredParams, func(i, j int) bool {
		return filteredParams[i].Name < filteredParams[j].Name
	})

	for _, p := range filteredParams {
		paramStr += p.Name + "="

		switch p.Value.Type {
		case pipelinev1.ParamTypeString:
			paramStr += p.Value.StringVal
		case pipelinev1.ParamTypeArray:
			// Sort array values for determinism
			arrayVals := make([]string, len(p.Value.ArrayVal))
			copy(arrayVals, p.Value.ArrayVal)
			sort.Strings(arrayVals)
			for i, val := range arrayVals {
				if i > 0 {
					paramStr += ","
				}
				paramStr += val
			}
		case pipelinev1.ParamTypeObject:
			// Sort object keys for determinism
			keys := make([]string, 0, len(p.Value.ObjectVal))
			for k := range p.Value.ObjectVal {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for i, key := range keys {
				if i > 0 {
					paramStr += ","
				}
				paramStr += key + ":" + p.Value.ObjectVal[key]
			}
		default:
			// For unknown types, use StringVal as fallback
			paramStr += p.Value.StringVal
		}
		paramStr += ";"
	}

	// Generate a SHA-256 hash of the parameter string
	hash := sha256.Sum256([]byte(paramStr))
	return hex.EncodeToString(hash[:])
}

// TODO(twoGiants): implement this feature
// GetCacheConfigName returns the name of the cache configuration ConfigMap.
// This can be overridden via the CONFIG_RESOLVER_CACHE_NAME environment variable.
func GetCacheConfigName() string {
	if configMapName := os.Getenv("CONFIG_RESOLVER_CACHE_NAME"); configMapName != "" {
		return configMapName
	}

	return defaultConfigMapName
}
