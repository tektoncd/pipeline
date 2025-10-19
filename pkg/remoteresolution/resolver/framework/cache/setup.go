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
	"sync"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

var (
	sharedCache   *resolverCache
	cacheInitOnce sync.Once
)

type resolverCacheKey struct{}

func init() {
	injection.Default.RegisterClient(addCacheWithLoggerToCtx)
}

func addCacheWithLoggerToCtx(ctx context.Context, _ *rest.Config) context.Context {
	return context.WithValue(ctx, resolverCacheKey{}, createCacheOnce(ctx))
}

func createCacheOnce(ctx context.Context) *resolverCache {
	cacheInitOnce.Do(func() {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		sharedCache = newResolverCache(defaultCacheSize, defaultExpiration)
	})

	return sharedCache.withLogger(
		logging.FromContext(ctx),
	)
}

// Get extracts the ResolverCache from the context.
// If the cache is not available in the context (e.g., in tests),
// it falls back to the shared cache with a logger from the context.
func Get(ctx context.Context) *resolverCache {
	if untyped := ctx.Value(resolverCacheKey{}); untyped != nil {
		return untyped.(*resolverCache)
	}

	return createCacheOnce(ctx)
}
