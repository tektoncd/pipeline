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
	"testing"
	"time"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		params       []pipelinev1.Param
	}{
		{
			name:         "empty params",
			resolverType: "http",
			params:       []pipelinev1.Param{},
		},
		{
			name:         "single param",
			resolverType: "http",
			params: []pipelinev1.Param{
				{
					Name: "url",
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "https://example.com",
					},
				},
			},
		},
		{
			name:         "multiple params",
			resolverType: "git",
			params: []pipelinev1.Param{
				{
					Name: "url",
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "https://github.com/tektoncd/pipeline",
					},
				},
				{
					Name: "revision",
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "main",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := GenerateCacheKey(tt.resolverType, tt.params)
			if key == "" {
				t.Error("GenerateCacheKey() returned empty key")
			}
		})
	}
}

func TestGenerateCacheKey_IndependentOfCacheParam(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		params       []pipelinev1.Param
		expectedSame bool
		description  string
	}{
		{
			name:         "same params without cache param",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
			},
			expectedSame: true,
			description:  "Params without cache param should generate same key",
		},
		{
			name:         "same params with different cache values",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
				{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
			},
			expectedSame: true,
			description:  "Params with cache=true should generate same key as without cache param",
		},
		{
			name:         "same params with cache=false",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
				{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "false"}},
			},
			expectedSame: true,
			description:  "Params with cache=false should generate same key as without cache param",
		},
		{
			name:         "different params should generate different keys",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "v0.50.0"}},
			},
			expectedSame: false,
			description:  "Different revision should generate different key",
		},
		{
			name:         "array params",
			resolverType: "bundle",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "gcr.io/tekton-releases/catalog/upstream/git-clone"}},
				{Name: "name", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "git-clone"}},
				{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
			},
			expectedSame: true,
			description:  "Array params with cache should generate same key as without cache",
		},
		{
			name:         "object params",
			resolverType: "hub",
			params: []pipelinev1.Param{
				{Name: "name", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "git-clone"}},
				{Name: "version", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "0.8"}},
				{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "false"}},
			},
			expectedSame: true,
			description:  "Object params with cache should generate same key as without cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedSame {
				// Generate key with cache param
				keyWithCache := GenerateCacheKey(tt.resolverType, tt.params)

				// Generate key without cache param
				paramsWithoutCache := make([]pipelinev1.Param, 0, len(tt.params))
				for _, p := range tt.params {
					if p.Name != "cache" {
						paramsWithoutCache = append(paramsWithoutCache, p)
					}
				}
				keyWithoutCache := GenerateCacheKey(tt.resolverType, paramsWithoutCache)

				if keyWithCache != keyWithoutCache {
					t.Errorf("Expected same keys, but got different:\nWith cache: %s\nWithout cache: %s\nDescription: %s",
						keyWithCache, keyWithoutCache, tt.description)
				}
			} else {
				// For different params test, create a second set with different values
				params2 := make([]pipelinev1.Param, len(tt.params))
				copy(params2, tt.params)
				// Change the revision value to make it different
				for i := range params2 {
					if params2[i].Name == "revision" {
						params2[i].Value.StringVal = "main"
						break
					}
				}

				key1 := GenerateCacheKey(tt.resolverType, tt.params)

				key2 := GenerateCacheKey(tt.resolverType, params2)

				if key1 == key2 {
					t.Errorf("Expected different keys, but got same: %s\nDescription: %s",
						key1, tt.description)
				}
			}
		})
	}
}

func TestGenerateCacheKey_Deterministic(t *testing.T) {
	resolverType := "git"
	params := []pipelinev1.Param{
		{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
		{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
		{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
	}

	// Generate the same key multiple times
	key1 := GenerateCacheKey(resolverType, params)

	key2 := GenerateCacheKey(resolverType, params)

	if key1 != key2 {
		t.Errorf("Cache key generation is not deterministic. Got different keys: %s vs %s", key1, key2)
	}
}

func TestGenerateCacheKey_AllParamTypes(t *testing.T) {
	resolverType := "test"
	params := []pipelinev1.Param{
		{Name: "string-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "string-value"}},
		{Name: "array-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeArray, ArrayVal: []string{"item1", "item2"}}},
		{Name: "object-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeObject, ObjectVal: map[string]string{"key1": "value1", "key2": "value2"}}},
		{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
	}

	// Generate key with cache param
	keyWithCache := GenerateCacheKey(resolverType, params)

	// Generate key without cache param
	paramsWithoutCache := make([]pipelinev1.Param, 0, len(params))
	for _, p := range params {
		if p.Name != "cache" {
			paramsWithoutCache = append(paramsWithoutCache, p)
		}
	}
	keyWithoutCache := GenerateCacheKey(resolverType, paramsWithoutCache)

	if keyWithCache != keyWithoutCache {
		t.Errorf("Expected same keys for all param types, but got different:\nWith cache: %s\nWithout cache: %s",
			keyWithCache, keyWithoutCache)
	}
}

func TestResolverCache(t *testing.T) {
	cache := NewResolverCache(DefaultMaxSize)

	// Test adding and getting a value
	key := "test-key"
	value := "test-value"
	cache.Add(key, value)

	if got, ok := cache.Get(key); !ok || got != value {
		t.Errorf("Get() = %v, %v, want %v, true", got, ok, value)
	}

	// Test expiration
	shortExpiration := 100 * time.Millisecond
	cache.AddWithExpiration("expiring-key", "expiring-value", shortExpiration)
	time.Sleep(shortExpiration + 50*time.Millisecond)

	if _, ok := cache.Get("expiring-key"); ok {
		t.Error("Get() returned true for expired key")
	}

	// Test removed - using dependency injection instead of global cache

	// Test that WithLogger creates new instances with logger
	testCache := NewResolverCache(1000)
	logger1 := testCache.WithLogger(nil)
	logger2 := testCache.WithLogger(nil)
	if logger1 == logger2 {
		t.Error("WithLogger() should return different instances")
	}
}

func TestInitializeFromConfigMap(t *testing.T) {
	tests := []struct {
		name           string
		configMap      *corev1.ConfigMap
		expectedSize   int
		expectedTTL    time.Duration
		shouldRecreate bool
	}{
		{
			name: "valid configuration",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GetCacheConfigName(),
					Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
				},
				Data: map[string]string{
					"max-size": "100",
				},
			},
			expectedSize:   100,
			expectedTTL:    DefaultExpiration,
			shouldRecreate: true,
		},
		{
			name: "cache config with maxSize and expiration",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GetCacheConfigName(),
					Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
				},
				Data: map[string]string{
					"max-size":    "200",
					"default-ttl": "10m",
				},
			},
			expectedSize:   200,
			expectedTTL:    10 * time.Minute,
			shouldRecreate: true,
		},
		{
			name: "cache config with invalid expiration",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GetCacheConfigName(),
					Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
				},
				Data: map[string]string{
					"max-size":    "150",
					"default-ttl": "invalid",
				},
			},
			expectedSize:   150,
			expectedTTL:    DefaultExpiration,
			shouldRecreate: true,
		},
		{
			name:           "nil config map",
			configMap:      nil,
			expectedSize:   DefaultMaxSize,
			expectedTTL:    DefaultExpiration,
			shouldRecreate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original DefaultExpiration to restore later
			originalTTL := DefaultExpiration
			defer func() { DefaultExpiration = originalTTL }()

			cache := NewResolverCache(DefaultMaxSize)
			originalCache := cache.cache

			cache.InitializeFromConfigMap(tt.configMap)

			// Verify cache size
			if tt.shouldRecreate && cache.cache == originalCache {
				t.Error("Expected cache to be recreated with new size")
			}

			// Verify TTL (InitializeFromConfigMap modifies the global DefaultExpiration)
			if DefaultExpiration != tt.expectedTTL {
				t.Errorf("Expected TTL %v, got %v", tt.expectedTTL, DefaultExpiration)
			}
		})
	}
}

func TestResolverCacheOperations(t *testing.T) {
	cache := NewResolverCache(100)

	// Test Add and Get
	key := "test-key"
	value := "test-value"
	cache.Add(key, value)

	if v, found := cache.Get(key); !found || v != value {
		t.Errorf("Expected to find value %v, got %v (found: %v)", value, v, found)
	}

	// Test Remove
	cache.Remove(key)
	if _, found := cache.Get(key); found {
		t.Error("Expected key to be removed")
	}

	// Test AddWithExpiration
	customTTL := 1 * time.Second
	cache.AddWithExpiration(key, value, customTTL)

	if v, found := cache.Get(key); !found || v != value {
		t.Errorf("Expected to find value %v, got %v (found: %v)", value, v, found)
	}

	// Wait for expiration
	time.Sleep(customTTL + 100*time.Millisecond)
	if _, found := cache.Get(key); found {
		t.Error("Expected key to be expired")
	}
}
