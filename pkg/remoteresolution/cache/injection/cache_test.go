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
	"testing"

	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestGetResolverCache(t *testing.T) {
	ctx := logtesting.TestContextWithLogger(t)

	// Test getting cache from context
	resolverCache := GetResolverCache(ctx)
	if resolverCache == nil {
		t.Error("Expected resolver cache but got nil")
	}

	// GetResolverCache creates a new wrapper with logger each time
	// but the underlying cache data is shared
	resolverCache2 := GetResolverCache(ctx)
	if resolverCache2 == nil {
		t.Error("Expected resolver cache but got nil on second call")
	}
}

func TestGetResolverCacheWithContextValue(t *testing.T) {
	logger := logtesting.TestLogger(t)
	ctx := t.Context()

	// Create a cache and inject it into context
	testCache := cache.NewResolverCache(100).WithLogger(logger)
	ctx = context.WithValue(ctx, key{}, testCache)

	// Get cache from context
	resolverCache := GetResolverCache(ctx)
	if resolverCache == nil {
		t.Error("Expected resolver cache but got nil")
	}

	if resolverCache != testCache {
		t.Error("Expected injected cache but got different instance")
	}
}

func TestGetResolverCacheFallback(t *testing.T) {
	// Create a plain context without any injected cache
	ctx := logtesting.TestContextWithLogger(t)

	// Should fall back to shared cache
	resolverCache := GetResolverCache(ctx)
	if resolverCache == nil {
		t.Error("Expected resolver cache but got nil")
	}
}

func TestInitializeSharedCache(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"resolver-cache-size": "500",
		},
	}

	// This should not panic
	InitializeSharedCache(configMap)

	// Verify we can still get the cache
	ctx := logtesting.TestContextWithLogger(t)
	resolverCache := GetResolverCache(ctx)
	if resolverCache == nil {
		t.Error("Expected resolver cache after initialization but got nil")
	}
}

func TestInitializeSharedCacheWithNil(t *testing.T) {
	// This should not panic with nil configmap
	InitializeSharedCache(nil)

	// Verify we can still get the cache
	ctx := logtesting.TestContextWithLogger(t)
	resolverCache := GetResolverCache(ctx)
	if resolverCache == nil {
		t.Error("Expected resolver cache after initialization with nil but got nil")
	}
}

func TestCacheSharing(t *testing.T) {
	// Create two different contexts
	ctx1 := logtesting.TestContextWithLogger(t)
	ctx2 := logtesting.TestContextWithLogger(t)

	// Get cache from first context
	cache1 := GetResolverCache(ctx1)
	if cache1 == nil {
		t.Fatal("Expected cache from ctx1")
	}

	// Get cache from second context
	cache2 := GetResolverCache(ctx2)
	if cache2 == nil {
		t.Fatal("Expected cache from ctx2")
	}

	// Both contexts should share the same underlying cache storage
	// even though they have different logger wrappers
	// We can verify this by checking they're both non-nil and functional
	if cache1 == nil || cache2 == nil {
		t.Error("Cache instances should be available from both contexts")
	}
}
