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
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"go.uber.org/zap"
	utilcache "k8s.io/apimachinery/pkg/util/cache"
)

var _ resolutionframework.ConfigWatcher = (*resolverCache)(nil)

// resolverCache is a wrapper around utilcache.LRUExpireCache that provides
// type-safe methods for caching resolver results.
type resolverCache struct {
	cache  *utilcache.LRUExpireCache
	logger *zap.SugaredLogger
	ttl    time.Duration
	clock  Clock
}

func newResolverCache(maxSize int, ttl time.Duration) *resolverCache {
	return &resolverCache{
		cache: utilcache.NewLRUExpireCache(maxSize),
		ttl:   ttl,
		clock: realClock{},
	}
}

// GetConfigName returns the name of the cache's configmap.
func (c *resolverCache) GetConfigName(_ context.Context) string {
	return getCacheConfigName()
}

// withLogger returns a new ResolverCache instance with the provided logger.
// This prevents state leak by not storing logger in the global singleton.
func (c *resolverCache) withLogger(logger *zap.SugaredLogger) *resolverCache {
	return &resolverCache{logger: logger, cache: c.cache, ttl: c.ttl, clock: c.clock}
}

// Get retrieves a cached resource by resolver type and parameters, returning
// the resource and whether it was found.
func (c *resolverCache) Get(resolverType string, params []pipelinev1.Param) (resolutionframework.ResolvedResource, bool) {
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
	return newAnnotatedResource(resource, resolverType, cacheOperationRetrieve, c.clock), true
}

func (c *resolverCache) infow(msg string, keysAndValues ...any) {
	if c.logger != nil {
		c.logger.Infow(msg, keysAndValues...)
	}
}

// Add stores a resource in the cache with the configured TTL and returns an
// annotated version of the resource.
func (c *resolverCache) Add(
	resolverType string,
	params []pipelinev1.Param,
	resource resolutionframework.ResolvedResource,
) resolutionframework.ResolvedResource {
	key := generateCacheKey(resolverType, params)
	c.infow("Adding to cache", "key", key, "expiration", c.ttl)

	annotatedResource := newAnnotatedResource(resource, resolverType, cacheOperationStore, c.clock)

	c.cache.Add(key, annotatedResource, c.ttl)

	return annotatedResource
}

// Remove deletes a cached resource identified by resolver type and parameters.
func (c *resolverCache) Remove(resolverType string, params []pipelinev1.Param) {
	key := generateCacheKey(resolverType, params)
	c.infow("Removing from cache", "key", key)

	c.cache.Remove(key)
}

// Clear removes all entries from the cache.
func (c *resolverCache) Clear() {
	c.infow("Clearing all cache entries")
	// predicate that returns true clears all entries
	c.cache.RemoveAll(func(_ any) bool { return true })
}

func generateCacheKey(resolverType string, params []pipelinev1.Param) string {
	// Create a deterministic string representation of the parameters
	paramStr := resolverType + ":"

	// Filter out the 'cache' parameter and sort remaining params by name for determinism
	filteredParams := make([]pipelinev1.Param, 0, len(params))
	for _, p := range params {
		if p.Name != CacheParam {
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
