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
	"errors"
	"testing"
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	bundleresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"go.uber.org/zap/zaptest"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		params       []pipelinev1.Param
		expectedKey  string
	}{
		{
			name:         "empty params",
			resolverType: "http",
			params:       []pipelinev1.Param{},
			expectedKey:  "1c31dda07cb1e09e89bd660a8d114936b44f728b73a3bc52c69a409ee1d44e67",
		},
		{
			name:         "single string param",
			resolverType: "http",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://example.com"}},
			},
			expectedKey: "63f68e3e567eafd7efb4149b3389b3261784c8ac5847b62e90b7ae8d23f6e889",
		},
		{
			name:         "multiple string params",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
			},
			expectedKey: "fbe74989962e04dbb512a986864acff592dd02e84ab20f7544fa6b473648f28c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualKey := generateCacheKey(tt.resolverType, tt.params)
			if tt.expectedKey != actualKey {
				t.Errorf("want %s, got %s", tt.expectedKey, actualKey)
			}
		})
	}
}

func TestGenerateCacheKeyProperties(t *testing.T) {
	t.Run("all param types produce expected key", func(t *testing.T) {
		expectedKey := "8cc0886ad987feeb9a7dd70cf1b54f988e240d2d6b15a6c4c063c1c8460dcabe"
		params := []pipelinev1.Param{
			{Name: "string-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "string-value"}},
			{Name: "array-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeArray, ArrayVal: []string{"item1", "item2"}}},
			{Name: "object-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeObject, ObjectVal: map[string]string{"key1": "value1", "key2": "value2"}}},
		}

		actualKey := generateCacheKey("test", params)
		if expectedKey != actualKey {
			t.Errorf("want %s, got %s", expectedKey, actualKey)
		}
	})

	t.Run("independent of param order", func(t *testing.T) {
		paramsAB := []pipelinev1.Param{
			{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
			{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
		}
		paramsBA := []pipelinev1.Param{
			{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
			{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
		}

		keyAB := generateCacheKey("git", paramsAB)
		keyBA := generateCacheKey("git", paramsBA)

		if keyAB != keyBA {
			t.Errorf("expected same key regardless of param order, got %s vs %s", keyAB, keyBA)
		}
	})

	t.Run("ignores cache param", func(t *testing.T) {
		paramsWithout := []pipelinev1.Param{
			{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
			{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
		}
		paramsWith := []pipelinev1.Param{
			{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
			{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
			{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
		}

		keyWithout := generateCacheKey("git", paramsWithout)
		keyWith := generateCacheKey("git", paramsWith)

		if keyWithout != keyWith {
			t.Errorf("expected cache param to be ignored, got different keys: %s vs %s", keyWithout, keyWith)
		}
	})

	t.Run("different params produce different keys", func(t *testing.T) {
		params1 := []pipelinev1.Param{
			{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
			{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
		}
		params2 := []pipelinev1.Param{
			{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
			{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "v0.50.0"}},
		}

		key1 := generateCacheKey("git", params1)
		key2 := generateCacheKey("git", params2)

		if key1 == key2 {
			t.Errorf("expected different keys for different params, got same: %s", key1)
		}
	})
}

func TestCacheTTLExpiration(t *testing.T) {
	// GIVEN
	resolverType := "bundle"
	fc := fakeClock{time.Now()}
	ttl := 5 * time.Minute
	params := []pipelinev1.Param{
		{Name: resolverType, Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo@sha256:abcdef"}},
	}
	resolveFn := func() (resolutionframework.ResolvedResource, error) {
		return &mockResolvedResource{data: []byte("test data")}, nil
	}

	// WHEN
	cache := newResolverCacheWithClock(100, ttl, &fc).withLogger(zaptest.NewLogger(t).Sugar())
	defer cache.Clear()
	resource, err := cache.GetCachedOrResolveFromRemote(params, resolverType, resolveFn)

	// THEN
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if resource.Annotations()[cacheOperationKey] != cacheOperationStore {
		t.Fatalf("expected cache miss and cache operation 'store', got %s", resource.Annotations()[cacheOperationKey])
	}

	// WHEN
	fc.Advance(ttl + time.Second)
	resource, err = cache.GetCachedOrResolveFromRemote(params, resolverType, resolveFn)

	// THEN: Verify entry is no longer in cache after TTL expiration
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if resource.Annotations()[cacheOperationKey] != cacheOperationStore {
		t.Fatalf("expected cache miss and cache operation 'store', got %s", resource.Annotations()[cacheOperationKey])
	}
}

func TestCacheMaxSizeLRUEviction(t *testing.T) {
	// Create cache with small size (3 entries) for testing LRU access pattern
	maxSize := 3
	cache := newResolverCache(maxSize, 1*time.Hour)
	defer cache.Clear()

	resolverType := "bundle"

	resolutionErr := errors.New("resolution error")
	resolveFnErr := func() (resolutionframework.ResolvedResource, error) {
		return nil, resolutionErr
	}

	// Create 3 different entries
	entries := []struct {
		name   string
		params []pipelinev1.Param
	}{
		{
			name: "entry1",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo1@sha256:aaa"}},
			},
		},
		{
			name: "entry2",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo2@sha256:bbb"}},
			},
		},
		{
			name: "entry3",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo3@sha256:ccc"}},
			},
		},
	}

	// Add all 3 entries
	for _, entry := range entries {
		resolveFn := func() (resolutionframework.ResolvedResource, error) {
			return &mockResolvedResource{
				data: []byte(entry.name),
			}, nil
		}
		cache.GetCachedOrResolveFromRemote(entry.params, resolverType, resolveFn)
	}

	// Access entry1 to make it recently used (entry2 becomes LRU)
	if _, err := cache.GetCachedOrResolveFromRemote(entries[0].params, resolverType, resolveFnErr); err != nil {
		t.Errorf("Expected cache hit for entry1, but got error %v", err)
	}

	// Add a 4th entry which should evict entry2 (now LRU) instead of entry1
	entry4Params := []pipelinev1.Param{
		{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo4@sha256:ddd"}},
	}
	resolveFnEntry4 := func() (resolutionframework.ResolvedResource, error) {
		return &mockResolvedResource{
			data: []byte("entry4"),
		}, nil
	}
	cache.GetCachedOrResolveFromRemote(entry4Params, resolverType, resolveFnEntry4)

	// Verify entry2 (LRU after entry1 was accessed) was evicted
	if _, err := cache.GetCachedOrResolveFromRemote(entries[1].params, resolverType, resolveFnErr); err == nil {
		t.Error("Expected entry2 to be evicted (LRU), but it was still in cache")
	}

	// Verify entry1 (accessed recently) is still in cache
	if _, err := cache.GetCachedOrResolveFromRemote(entries[0].params, resolverType, resolveFnErr); err != nil {
		t.Errorf("Expected entry1 to still be in cache after being accessed, but got cache miss and error %v", err)
	}

	// Verify entry3 is still in cache
	if _, err := cache.GetCachedOrResolveFromRemote(entries[2].params, resolverType, resolveFnErr); err != nil {
		t.Errorf("Expected entry3 to still be in cache, but got cache miss and error %v", err)
	}

	// Verify entry4 is in cache
	if _, err := cache.GetCachedOrResolveFromRemote(entry4Params, resolverType, resolveFnErr); err != nil {
		t.Errorf("Expected entry4 to be in cache, but got cache miss and error %v", err)
	}
}

// TestGetCachedOrResolveFromRemote verifies the cache-or-resolve logic for all
// combinations of cache state (miss/hit) and resolution or casting outcome.
//
// Test cases:
//   - cache miss with successful resolution stores and retrieves the resource
//   - cache miss with failed resolution returns the resolution error
//   - cache hit with a poisoned entry returns a casting error
func TestGetCachedOrResolveFromRemote(t *testing.T) {
	params := []pipelinev1.Param{{
		Name:  bundleresolution.ParamBundle,
		Value: pipelinev1.ParamValue{StringVal: "registry.io/repo@sha256:abcdef"},
	}}

	t.Run("cache miss and resolution success stores then retrieves", func(t *testing.T) {
		// GIVEN
		resolveFn := func() (resolutionframework.ResolvedResource, error) {
			return &mockResolvedResource{data: []byte("test data")}, nil
		}
		cache := newResolverCache(100, 1*time.Hour)
		defer cache.Clear()

		// WHEN
		cachePopulationResult, cachePopulationErr := cache.GetCachedOrResolveFromRemote(
			params,
			bundleresolution.LabelValueBundleResolverType,
			resolveFn,
		)

		cacheHitResult, cacheHitErr := cache.GetCachedOrResolveFromRemote(
			params,
			bundleresolution.LabelValueBundleResolverType,
			resolveFn,
		)

		// THEN
		if cachePopulationErr != nil {
			t.Fatalf("unexpected error: %v", cachePopulationErr)
		}

		actualCachePopulationOperation := cachePopulationResult.Annotations()[cacheOperationKey]
		if actualCachePopulationOperation != cacheOperationStore {
			t.Fatalf("expected %s, got %s", cacheOperationStore, actualCachePopulationOperation)
		}

		if cacheHitErr != nil {
			t.Fatalf("unexpected error: %v", cacheHitErr)
		}

		actualCacheHitOperation := cacheHitResult.Annotations()[cacheOperationKey]
		if actualCacheHitOperation != cacheOperationRetrieve {
			t.Fatalf("expected %s, got %s", cacheOperationRetrieve, actualCacheHitOperation)
		}
	})

	t.Run("cache miss and resolution error returns error", func(t *testing.T) {
		// GIVEN
		expectedError := errors.New("resolution failed")
		resolveFn := func() (resolutionframework.ResolvedResource, error) {
			return nil, expectedError
		}
		cache := newResolverCache(100, 1*time.Hour)
		defer cache.Clear()

		// WHEN
		result, actualErr := cache.GetCachedOrResolveFromRemote(
			params,
			bundleresolution.LabelValueBundleResolverType,
			resolveFn,
		)

		// THEN
		if result != nil {
			t.Fatalf("unexpected result: %v", result)
		}

		if actualErr == nil {
			t.Fatal("expected error, got nil")
		}

		if !errors.Is(actualErr, expectedError) {
			t.Fatalf("expected error %v, got error %v", expectedError, actualErr)
		}
	})

	t.Run("cache hit with casting error returns error", func(t *testing.T) {
		// GIVEN
		resolveFn := func() (resolutionframework.ResolvedResource, error) {
			return &mockResolvedResource{data: []byte("test data")}, nil
		}
		cacheWrapper := newResolverCache(100, 1*time.Hour)
		key := generateCacheKey(bundleresolution.LabelValueBundleResolverType, params)
		defer cacheWrapper.Clear()

		// WHEN
		cachePopulationResult, cachePopulationErr := cacheWrapper.GetCachedOrResolveFromRemote(
			params,
			bundleresolution.LabelValueBundleResolverType,
			resolveFn,
		)
		cacheWrapper.cache.Add(key, "poisoned-resource", cacheWrapper.TTL())
		result, castingFailedErr := cacheWrapper.GetCachedOrResolveFromRemote(
			params,
			bundleresolution.LabelValueBundleResolverType,
			resolveFn,
		)

		// THEN
		if cachePopulationResult == nil {
			t.Error("expected resolved recourse, got nil")
		}

		if cachePopulationErr != nil {
			t.Fatalf("unexpected error %v", cachePopulationErr)
		}

		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}

		if castingFailedErr == nil {
			t.Fatal("expected casting error, got nil")
		}
	})
}
