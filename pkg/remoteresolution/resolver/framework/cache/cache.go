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
	"errors"
	"sort"
	"strings"
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	utilcache "k8s.io/apimachinery/pkg/util/cache"
)

type resolveFn = func() (resolutionframework.ResolvedResource, error)

var _ resolutionframework.ConfigWatcher = (*resolverCache)(nil)

// resolverCache is a wrapper around utilcache.LRUExpireCache that provides
// type-safe methods for caching resolver results.
type resolverCache struct {
	cache       *utilcache.LRUExpireCache
	logger      *zap.SugaredLogger
	ttl         time.Duration
	maxSize     int
	clock       utilcache.Clock
	flightGroup *singleflight.Group
}

func newResolverCache(maxSize int, ttl time.Duration) *resolverCache {
	return newResolverCacheWithClock(maxSize, ttl, realClock{})
}

func newResolverCacheWithClock(maxSize int, ttl time.Duration, clock utilcache.Clock) *resolverCache {
	return &resolverCache{
		cache:       utilcache.NewLRUExpireCacheWithClock(maxSize, clock),
		ttl:         ttl,
		maxSize:     maxSize,
		clock:       clock,
		flightGroup: &singleflight.Group{},
	}
}

// GetConfigName returns the name of the cache's configmap.
func (c *resolverCache) GetConfigName(_ context.Context) string {
	return getCacheConfigName()
}

// withLogger returns a new ResolverCache instance with the provided logger.
// This prevents state leak by not storing logger in the global singleton.
func (c *resolverCache) withLogger(logger *zap.SugaredLogger) *resolverCache {
	return &resolverCache{logger: logger, cache: c.cache, ttl: c.ttl, maxSize: c.maxSize, clock: c.clock, flightGroup: c.flightGroup}
}

// TTL returns the time-to-live duration for cache entries.
func (c *resolverCache) TTL() time.Duration {
	return c.ttl
}

// MaxSize returns the maximum number of entries the cache can hold.
func (c *resolverCache) MaxSize() int {
	return c.maxSize
}

func (c *resolverCache) GetCachedOrResolveFromRemote(
	params []pipelinev1.Param,
	resolverType string,
	resolveFromRemote resolveFn,
) (resolutionframework.ResolvedResource, error) {
	key := generateCacheKey(resolverType, params)

	if untyped, found := c.cache.Get(key); found {
		cached, ok := untyped.(resolutionframework.ResolvedResource)
		if !ok {
			c.infow("Failed casting cached resource", "key", key)
			return nil, errors.New("failed casting cached resource")
		}

		c.infow("Cache hit", "key", key)

		return c.annotate(cached, resolverType, cacheOperationRetrieve), nil
	}

	// If cache miss, resolve from remote using singleflight
	untyped, err, _ := c.flightGroup.Do(key, func() (any, error) {
		resolved, err := resolveFromRemote()
		if err != nil {
			return nil, err
		}

		annotated := c.annotate(resolved, resolverType, cacheOperationStore)

		// Store annotated resource with store operation and return annotated resource
		// to indicate it was stored in cache
		c.infow("Adding to cache", "key", key, "expiration", c.ttl)
		c.cache.Add(key, annotated, c.ttl)
		return annotated, nil
	})
	if err != nil {
		return nil, err
	}

	return untyped.(resolutionframework.ResolvedResource), nil
}

func (c *resolverCache) annotate(resolvedResource resolutionframework.ResolvedResource, resolverType, operation string) *annotatedResource {
	timestamp := c.clock.Now().Format(time.RFC3339)
	result := newAnnotatedResource(resolvedResource, resolverType, operation, timestamp)
	return result
}

func (c *resolverCache) infow(msg string, keysAndValues ...any) {
	if c.logger != nil {
		c.logger.Infow(msg, keysAndValues...)
	}
}

// Clear removes all entries from the cache.
func (c *resolverCache) Clear() {
	c.infow("Clearing all cache entries")
	// predicate that returns true clears all entries
	c.cache.RemoveAll(func(_ any) bool { return true })
}

func generateCacheKey(resolverType string, params []pipelinev1.Param) string {
	// Create a deterministic string representation of the parameters
	var sb strings.Builder
	sb.WriteString(resolverType)
	sb.WriteByte(':')

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
		sb.WriteString(p.Name)
		sb.WriteByte('=')

		switch p.Value.Type {
		case pipelinev1.ParamTypeString:
			sb.WriteString(p.Value.StringVal)
		case pipelinev1.ParamTypeArray:
			// Sort array values for determinism
			arrayVals := make([]string, len(p.Value.ArrayVal))
			copy(arrayVals, p.Value.ArrayVal)
			sort.Strings(arrayVals)
			for i, val := range arrayVals {
				if i > 0 {
					sb.WriteByte(',')
				}
				sb.WriteString(val)
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
					sb.WriteByte(',')
				}
				sb.WriteString(key)
				sb.WriteByte(':')
				sb.WriteString(p.Value.ObjectVal[key])
			}
		default:
			// For unknown types, use StringVal as fallback
			sb.WriteString(p.Value.StringVal)
		}
		sb.WriteByte(';')
	}

	// Generate a SHA-256 hash of the parameter string
	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}
