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

	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// key is used as the key for associating information with a context.Context.
type key struct{}

// sharedCache is the shared cache instance used across all contexts
var sharedCache = cache.NewResolverCache(cache.DefaultCacheSize)

// initOnce ensures InitializeSharedCache is only called once
var initOnce sync.Once

func init() {
	injection.Default.RegisterClient(cacheWithLoggerFromCtx)
	injection.Default.RegisterClientFetcher(func(ctx context.Context) any {
		return GetResolverCache(ctx)
	})
}

func cacheWithLoggerFromCtx(ctx context.Context, _ *rest.Config) context.Context {
	logger := logging.FromContext(ctx)

	// Return the SAME shared cache instance with logger to prevent state leak
	resolverCache := sharedCache.WithLogger(logger)

	return context.WithValue(ctx, key{}, resolverCache)
}

// GetResolverCache extracts the ResolverCache from the context.
// If the cache is not available in the context (e.g., in tests),
// it falls back to the shared cache with a logger from the context.
func GetResolverCache(ctx context.Context) *cache.ResolverCache {
	if resolverCache := ctx.Value(key{}); resolverCache != nil {
		return resolverCache.(*cache.ResolverCache)
	}

	// Fallback for test contexts or when injection is not available
	logger := logging.FromContext(ctx)
	return sharedCache.WithLogger(logger)
}

// InitializeSharedCache initializes the shared cache from a ConfigMap.
// This function uses sync.Once to ensure it's only called once, even if
// multiple resolvers try to initialize it. This is safe to call from
// any resolver's Initialize method.
func InitializeSharedCache(configMap *corev1.ConfigMap) {
	initOnce.Do(func() {
		sharedCache.InitializeFromConfigMap(configMap)
	})
}
