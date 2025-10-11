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

// TODO(twoGiants): implement this feature
// InitializeFromConfigMap initializes the cache with configuration from a ConfigMap
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

// TODO(twoGiants): implement this feature
// GetCacheConfigName returns the name of the cache configuration ConfigMap.
// This can be overridden via the CONFIG_RESOLVER_CACHE_NAME environment variable.
func GetCacheConfigName() string {
	if configMapName := os.Getenv("CONFIG_RESOLVER_CACHE_NAME"); configMapName != "" {
		return configMapName
	}

	return defaultConfigMapName
}

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

// Get retrieves a value from the cache.
func (c *ResolverCache) Get(key string) (interface{}, bool) {
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

// Add adds a value to the cache with the default expiration time.
func (c *ResolverCache) Add(key string, value interface{}) {
	if c.logger != nil {
		c.logger.Infow("Adding to cache", "key", key, "expiration", defaultExpiration)
	}
	c.cache.Add(key, value, defaultExpiration)
}

// Remove removes a value from the cache.
func (c *ResolverCache) Remove(key string) {
	if c.logger != nil {
		c.logger.Infow("Removing from cache", "key", key)
	}
	c.cache.Remove(key)
}

// AddWithExpiration adds a value to the cache with a custom expiration time
func (c *ResolverCache) AddWithExpiration(key string, value interface{}, expiration time.Duration) {
	if c.logger != nil {
		c.logger.Infow("Adding to cache with custom expiration", "key", key, "expiration", expiration)
	}
	c.cache.Add(key, value, expiration)
}

// Clear removes all entries from the cache.
func (c *ResolverCache) Clear() {
	if c.logger != nil {
		c.logger.Infow("Clearing all cache entries")
	}
	// Use RemoveAll with a predicate that always returns true to clear all entries
	c.cache.RemoveAll(func(key any) bool {
		return true
	})
}

// Note: Global cache removed - use dependency injection via cache/injection package

// WithLogger returns a new ResolverCache instance with the provided logger.
// This prevents state leak by not storing logger in the global singleton.
func (c *ResolverCache) WithLogger(logger *zap.SugaredLogger) *ResolverCache {
	return &ResolverCache{logger: logger, cache: c.cache}
}

// GenerateCacheKey generates a cache key for the given resolver type and parameters.
func GenerateCacheKey(resolverType string, params []pipelinev1.Param) (string, error) {
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
	return hex.EncodeToString(hash[:]), nil
}
