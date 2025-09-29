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

package injection

import (
	"context"
	"sync"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// key is used as the key for associating information with a context.Context.
type key struct{}

// sharedCache is the shared cache instance used across all contexts (lazy initialized)
var (
	sharedCache      *cache.ResolverCache
	cacheOnce        sync.Once
	injectionOnce    sync.Once
	resolversEnabled bool
)

// initializeCacheIfNeeded initializes the cache and injection only if resolvers are enabled
func initializeCacheIfNeeded(ctx context.Context) {
	cacheOnce.Do(func() {
		cfg := resolverconfig.FromContextOrDefaults(ctx)
		resolversEnabled = cfg.FeatureFlags.AnyResolverEnabled()

		if resolversEnabled {
			sharedCache = cache.NewResolverCache(cache.DefaultMaxSize)

			// Register injection only if resolvers are enabled
			injectionOnce.Do(func() {
				injection.Default.RegisterClient(withCacheFromConfig)
				injection.Default.RegisterClientFetcher(func(ctx context.Context) interface{} {
					return Get(ctx)
				})
			})
		}
	})
}

func withCacheFromConfig(ctx context.Context, cfg *rest.Config) context.Context {
	// Ensure cache is initialized if needed
	initializeCacheIfNeeded(ctx)

	// Only inject cache if resolvers are enabled
	if !resolversEnabled || sharedCache == nil {
		return ctx // Return context unchanged if caching is disabled
	}

	logger := logging.FromContext(ctx)
	resolverCache := sharedCache.WithLogger(logger)
	return context.WithValue(ctx, key{}, resolverCache)
}

// Get extracts the ResolverCache from the context.
// Returns nil if resolvers are disabled to avoid cache overhead when not needed.
func Get(ctx context.Context) *cache.ResolverCache {
	// Ensure we've checked if cache should be initialized
	initializeCacheIfNeeded(ctx)

	// If resolvers are disabled, return nil - no caching needed
	if !resolversEnabled {
		return nil
	}

	untyped := ctx.Value(key{})
	if untyped == nil {
		// Fallback for test contexts or when injection is not available
		// but only if resolvers are enabled
		if sharedCache != nil {
			logger := logging.FromContext(ctx)
			return sharedCache.WithLogger(logger)
		}
		return nil
	}
	return untyped.(*cache.ResolverCache)
}
