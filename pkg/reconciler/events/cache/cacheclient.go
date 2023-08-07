/*
Copyright 2022 The Tekton Authors

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
	"time"

	bc "github.com/allegro/bigcache/v3"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// With 4 events per Run, we can store events for 1024 concurrent Runs
const bufferSize = 4096

func init() {
	injection.Default.RegisterClient(withCacheClient)
}

// cacheKey is a way to associate the Cache from inside the context.Context
type cacheKey struct{}

func withCacheClientFromSize(ctx context.Context, size int) context.Context {
	logger := logging.FromContext(ctx)

	config := bc.DefaultConfig(2 * time.Hour)
	config.HardMaxCacheSize = size

	cacheClient, err := bc.New(ctx, config)
	if err != nil {
		logger.Errorf("unable to create cacheClient :%s", err.Error())
	}

	return ToContext(ctx, cacheClient)
}

func withCacheClient(ctx context.Context, cfg *rest.Config) context.Context {
	return withCacheClientFromSize(ctx, bufferSize)
}

// Get extracts the cloudEventClient client from the context.
func Get(ctx context.Context) *bc.BigCache {
	untyped := ctx.Value(cacheKey{})
	if untyped == nil {
		logging.FromContext(ctx).Errorf("Unable to fetch client from context.")
		return nil
	}
	return untyped.(*bc.BigCache)
}

// ToContext adds the cloud events client to the context
func ToContext(ctx context.Context, c *bc.BigCache) context.Context {
	return context.WithValue(ctx, cacheKey{}, c)
}
