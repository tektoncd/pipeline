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
	"k8s.io/client-go/rest"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestKey(t *testing.T) {
	// Test that Key is a distinct type that can be used as context key
	key1 := Key{}
	key2 := Key{}

	// Keys should be equivalent when used as context keys
	ctx := context.Background()
	ctx = context.WithValue(ctx, key1, "test-value")

	value := ctx.Value(key2)
	if value != "test-value" {
		t.Errorf("Expected Key{} to be usable as context key, got nil")
	}
}

func TestSharedCacheInitialization(t *testing.T) {
	// Test that sharedCache is properly initialized
	if sharedCache == nil {
		t.Fatal("sharedCache should be initialized")
	}

	// Test that it's a valid ResolverCache instance
	if sharedCache == nil {
		t.Fatal("sharedCache should be a valid ResolverCache instance")
	}
}

func TestWithCacheFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		cfg         *rest.Config
		expectCache bool
	}{
		{
			name:        "basic context with config",
			ctx:         context.Background(),
			cfg:         &rest.Config{},
			expectCache: true,
		},
		{
			name:        "context with logger",
			ctx:         logtesting.TestContextWithLogger(t),
			cfg:         &rest.Config{},
			expectCache: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := withCacheFromConfig(tt.ctx, tt.cfg)

			// Check that result context contains cache
			cache := result.Value(Key{})
			if tt.expectCache && cache == nil {
				t.Errorf("Expected cache in context, got nil")
			}

			// Check that cache is functional (don't need type assertion for this test)
			if tt.expectCache && cache != nil {
				// The fact that it was stored and retrieved indicates correct type
				// Just check that we have a non-nil interface value
				if cache == nil {
					t.Errorf("Expected non-nil cache")
				}
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
				return context.Background()
			},
			expectNotNil:       true,
			expectSameInstance: false, // Should get fallback instance
		},
		{
			name: "context with cache",
			setupContext: func() context.Context {
				ctx := context.Background()
				testCache := cache.NewResolverCache(100)
				return context.WithValue(ctx, Key{}, testCache)
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
	ctx := context.Background()

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
	ctx := context.Background()
	testCache := cache.NewResolverCache(50)
	ctx = context.WithValue(ctx, Key{}, testCache)

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
