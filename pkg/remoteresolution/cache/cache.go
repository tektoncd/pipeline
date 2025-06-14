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
	"fmt"
	"time"

	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	utilcache "k8s.io/apimachinery/pkg/util/cache"
	"knative.dev/pkg/logging"
)

const (
	// DefaultExpiration is the default expiration time for cache entries
	DefaultExpiration = 5 * time.Minute

	// DefaultMaxSize is the default size for the cache
	DefaultMaxSize = 1000
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

// InitializeLogger initializes the logger for the cache using the provided context
func (c *ResolverCache) InitializeLogger(ctx context.Context) {
	if c.logger == nil {
		c.logger = logging.FromContext(ctx)
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
		c.logger.Infow("Adding to cache", "key", key, "expiration", DefaultExpiration)
	}
	c.cache.Add(key, value, DefaultExpiration)
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

// globalCache is the global instance of ResolverCache
var globalCache = NewResolverCache(DefaultMaxSize)

// GetGlobalCache returns the global cache instance.
func GetGlobalCache() *ResolverCache {
	return globalCache
}

// GenerateCacheKey generates a cache key for the given resolver type and parameters.
func GenerateCacheKey(resolverType string, params []v1.Param) (string, error) {
	// Create a deterministic string representation of the parameters
	paramStr := fmt.Sprintf("%s:", resolverType)
	for _, p := range params {
		paramStr += fmt.Sprintf("%s=%s;", p.Name, p.Value.StringVal)
	}

	// Generate a SHA-256 hash of the parameter string
	hash := sha256.Sum256([]byte(paramStr))
	return hex.EncodeToString(hash[:]), nil
}
