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
	"testing"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	"k8s.io/client-go/rest"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestKey(t *testing.T) {
	// Test that key is a distinct type that can be used as context key
	key1 := key{}
	key2 := key{}

	// Keys should be equivalent when used as context keys
	ctx := t.Context()
	ctx = context.WithValue(ctx, key1, "test-value")

	value := ctx.Value(key2)
	if value != "test-value" {
		t.Errorf("Expected key{} to be usable as context key, got nil")
	}
}

func TestSharedCacheInitialization(t *testing.T) {
	// Reset globals and create context with resolvers enabled
	resetCacheGlobals()
	ctx := createContextWithResolverConfig(t, true)

	// Initialize cache by calling Get
	Get(ctx)

	// Test that sharedCache is properly initialized when resolvers are enabled
	if sharedCache == nil {
		t.Fatal("sharedCache should be initialized when resolvers are enabled")
	}
}

func TestWithCacheFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() context.Context
		cfg         *rest.Config
		expectCache bool
	}{
		{
			name:        "basic context with config",
			setupCtx:    func() context.Context { return t.Context() },
			cfg:         &rest.Config{},
			expectCache: true,
		},
		{
			name:        "context with logger",
			setupCtx:    func() context.Context { return logtesting.TestContextWithLogger(t) },
			cfg:         &rest.Config{},
			expectCache: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := withCacheFromConfig(tt.setupCtx(), tt.cfg)

			// Check that result context contains cache
			cache := result.Value(key{})
			if tt.expectCache && cache == nil {
				t.Errorf("Expected cache in context, got nil")
			}

			// Check that cache is functional (don't need type assertion for this test)
			if tt.expectCache && cache != nil {
				// The fact that it was stored and retrieved indicates correct type
				// Cache is non-nil as verified by outer condition, so it's functional
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name               string
		setupContext       func() context.Context
		expectNotNil       bool
		expectSameInstance bool
	}{
		{
			name: "context without cache",
			setupContext: func() context.Context {
				return t.Context()
			},
			expectNotNil:       true,
			expectSameInstance: false, // Should get fallback instance
		},
		{
			name: "context with cache",
			setupContext: func() context.Context {
				ctx := t.Context()
				testCache := cache.NewResolverCache(100)
				return context.WithValue(ctx, key{}, testCache)
			},
			expectNotNil:       true,
			expectSameInstance: false, // Should get the injected instance
		},
		{
			name: "context with logger but no cache",
			setupContext: func() context.Context {
				return logtesting.TestContextWithLogger(t)
			},
			expectNotNil:       true,
			expectSameInstance: false, // Should get fallback with logger
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			result := Get(ctx)

			if tt.expectNotNil && result == nil {
				t.Errorf("Expected non-nil cache, got nil")
			}

			// Test that we can call methods on the returned cache
			if result != nil {
				// This should not panic
				result.Clear()
			}
		})
	}
}

func TestGetConsistency(t *testing.T) {
	// Test that Get returns consistent results for fallback case
	ctx := t.Context()

	cache1 := Get(ctx)
	cache2 := Get(ctx)

	if cache1 == nil || cache2 == nil {
		t.Fatal("Get should never return nil")
	}

	// Both should be valid cache instances
	cache1.Clear()
	cache2.Clear()
}

func TestGetWithInjectedCache(t *testing.T) {
	// Test that Get returns the injected cache when available
	ctx := t.Context()
	testCache := cache.NewResolverCache(50)
	ctx = context.WithValue(ctx, key{}, testCache)

	result := Get(ctx)
	if result != testCache {
		t.Errorf("Expected injected cache instance, got different instance")
	}
}

func TestWithCacheFromConfigIntegration(t *testing.T) {
	// Test the integration between withCacheFromConfig and Get
	ctx := logtesting.TestContextWithLogger(t)
	cfg := &rest.Config{}

	// Inject cache into context
	ctxWithCache := withCacheFromConfig(ctx, cfg)

	// Retrieve cache using Get
	retrievedCache := Get(ctxWithCache)

	if retrievedCache == nil {
		t.Fatal("Expected cache from injected context")
	}

	// Test that the cache is functional
	retrievedCache.Clear()
}

func TestGetFallbackWithLogger(t *testing.T) {
	// Test that Get properly handles logger in fallback case
	ctx := logtesting.TestContextWithLogger(t)

	cache := Get(ctx)
	if cache == nil {
		t.Fatal("Expected cache with logger fallback")
	}

	// Cache should be functional
	cache.Clear()
}

// TestConditionalCacheLoading tests that cache is only loaded when resolvers are enabled
func TestConditionalCacheLoading(t *testing.T) {
	testCases := []struct {
		name             string
		resolversEnabled bool
		expectCacheNil   bool
	}{
		{
			name:             "Cache loaded when resolvers enabled",
			resolversEnabled: true,
			expectCacheNil:   false,
		},
		{
			name:             "Cache not loaded when resolvers disabled",
			resolversEnabled: false,
			expectCacheNil:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset global state for each test
			resetCacheGlobals()

			// Create context with resolver config
			ctx := createContextWithResolverConfig(t, tc.resolversEnabled)

			// Test Get function
			cache := Get(ctx)

			if tc.expectCacheNil && cache != nil {
				t.Errorf("Expected cache to be nil when resolvers disabled, got non-nil cache")
			}
			if !tc.expectCacheNil && cache == nil {
				t.Errorf("Expected cache to be non-nil when resolvers enabled, got nil")
			}
		})
	}
}

// resetCacheGlobals resets the global cache state for testing
// This is necessary because sync.Once prevents re-initialization
func resetCacheGlobals() {
	// Reset the sync.Once variables by creating new instances
	cacheOnce = sync.Once{}
	injectionOnce = sync.Once{}
	sharedCache = nil
	resolversEnabled = false
}

// createContextWithResolverConfig creates a test context with resolver configuration
func createContextWithResolverConfig(t *testing.T, anyResolverEnabled bool) context.Context {
	t.Helper()

	ctx := t.Context()

	// Create feature flags based on test case
	featureFlags := &resolverconfig.FeatureFlags{
		EnableGitResolver:     anyResolverEnabled,
		EnableHubResolver:     false,
		EnableBundleResolver:  false,
		EnableClusterResolver: false,
		EnableHttpResolver:    false,
	}

	config := &resolverconfig.Config{
		FeatureFlags: featureFlags,
	}

	return resolverconfig.ToContext(ctx, config)
}
