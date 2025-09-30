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
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func newMockResolvedResource(content string) resolutionframework.ResolvedResource {
	return &mockResolvedResource{
		data:        []byte(content),
		annotations: map[string]string{"test": "annotation"},
		refSource:   &pipelinev1.RefSource{URI: "test://example.com"},
	}
}

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
			// SHA-256 of "http:"
			expectedKey: "1c31dda07cb1e09e89bd660a8d114936b44f728b73a3bc52c69a409ee1d44e67",
		},
		{
			name:         "single string param",
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
			// SHA-256 of "http:url=https://example.com;"
			expectedKey: "63f68e3e567eafd7efb4149b3389b3261784c8ac5847b62e90b7ae8d23f6e889",
		},
		{
			name:         "multiple string params sorted by name",
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
			// SHA-256 of "git:revision=main;url=https://github.com/tektoncd/pipeline;" (params sorted alphabetically)
			expectedKey: "fbe74989962e04dbb512a986864acff592dd02e84ab20f7544fa6b473648f28c",
		},
		{
			name:         "array param with sorted values",
			resolverType: "bundle",
			params: []pipelinev1.Param{
				{
					Name: "layers",
					Value: pipelinev1.ParamValue{
						Type:     pipelinev1.ParamTypeArray,
						ArrayVal: []string{"config", "base", "app"}, // Will be sorted: app,base,config
					},
				},
			},
			// SHA-256 of "bundle:layers=app,base,config;" (array values sorted)
			expectedKey: "f49a5f32af71ceaf14a749cf9d81c633abf962744dfd5214c3504d8c6485853d",
		},
		{
			name:         "object param with sorted keys",
			resolverType: "cluster",
			params: []pipelinev1.Param{
				{
					Name: "metadata",
					Value: pipelinev1.ParamValue{
						Type: pipelinev1.ParamTypeObject,
						ObjectVal: map[string]string{
							"namespace": "tekton-pipelines",
							"name":      "my-task",
							"kind":      "Task",
						},
					},
				},
			},
			// SHA-256 of "cluster:metadata=kind:Task,name:my-task,namespace:tekton-pipelines;" (object keys sorted)
			expectedKey: "526a5d7e242d438999cb09ac17f3a789bec124fb249573e411ce57f77fcf9858",
		},
		{
			name:         "params with cache param excluded",
			resolverType: "bundle",
			params: []pipelinev1.Param{
				{
					Name: "bundle",
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "gcr.io/tekton/catalog:v1.0.0",
					},
				},
				{
					Name: "cache", // This should be excluded from the key
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "always",
					},
				},
			},
			// SHA-256 of "bundle:bundle=gcr.io/tekton/catalog:v1.0.0;" (cache param excluded)
			expectedKey: "bb4509c0a043f4677f84005a320791c384a15b35026665bd95ff3bca0f563862",
		},
		{
			name:         "complex mixed param types",
			resolverType: "test",
			params: []pipelinev1.Param{
				{
					Name: "string-param",
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "simple-value",
					},
				},
				{
					Name: "array-param",
					Value: pipelinev1.ParamValue{
						Type:     pipelinev1.ParamTypeArray,
						ArrayVal: []string{"zebra", "alpha", "beta"}, // Will be sorted: alpha,beta,zebra
					},
				},
				{
					Name: "object-param",
					Value: pipelinev1.ParamValue{
						Type: pipelinev1.ParamTypeObject,
						ObjectVal: map[string]string{
							"z-key": "z-value",
							"a-key": "a-value",
							"m-key": "m-value",
						}, // Will be sorted: a-key:a-value,m-key:m-value,z-key:z-value
					},
				},
			},
			// SHA-256 of "test:array-param=alpha,beta,zebra;object-param=a-key:a-value,m-key:m-value,z-key:z-value;string-param=simple-value;"
			expectedKey: "776a04a1cd162d3df653260f01b9be45c158649bdbb019fddb36a419810a5364",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualKey := generateCacheKey(tt.resolverType, tt.params)

			// First verify key is not empty
			if actualKey == "" {
				t.Error("generateCacheKey() returned empty key")
				return
			}

			// Then verify it's a valid SHA-256 hex string (64 characters)
			if len(actualKey) != 64 {
				t.Errorf("Expected 64-character SHA-256 hex string, got %d characters: %s", len(actualKey), actualKey)
				return
			}

			// Most importantly: verify exact expected key for regression testing
			// Note: Update expected keys if the algorithm changes intentionally
			if actualKey != tt.expectedKey {
				t.Errorf("Cache key mismatch:\nExpected: %s\nActual:   %s\n\nThis indicates the cache key generation algorithm has changed.\nIf this is intentional, update the expected key.\nOtherwise, this is a regression that could invalidate existing cache entries.",
					tt.expectedKey, actualKey)
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
				keyWithCache := generateCacheKey(tt.resolverType, tt.params)

				// Generate key without cache param
				paramsWithoutCache := make([]pipelinev1.Param, 0, len(tt.params))
				for _, p := range tt.params {
					if p.Name != "cache" {
						paramsWithoutCache = append(paramsWithoutCache, p)
					}
				}
				keyWithoutCache := generateCacheKey(tt.resolverType, paramsWithoutCache)

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

				key1 := generateCacheKey(tt.resolverType, tt.params)

				key2 := generateCacheKey(tt.resolverType, params2)

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
	key1 := generateCacheKey(resolverType, params)

	key2 := generateCacheKey(resolverType, params)

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
	keyWithCache := generateCacheKey(resolverType, params)

	// Generate key without cache param
	paramsWithoutCache := make([]pipelinev1.Param, 0, len(params))
	for _, p := range params {
		if p.Name != "cache" {
			paramsWithoutCache = append(paramsWithoutCache, p)
		}
	}
	keyWithoutCache := generateCacheKey(resolverType, paramsWithoutCache)

	if keyWithCache != keyWithoutCache {
		t.Errorf("Expected same keys for all param types, but got different:\nWith cache: %s\nWithout cache: %s",
			keyWithCache, keyWithoutCache)
	}
}

func TestResolverCache(t *testing.T) {
	cache := NewResolverCache(DefaultMaxSize)

	// Test adding and getting a value
	resolverType := "http"
	params := []pipelinev1.Param{
		{Name: "url", Value: *pipelinev1.NewStructuredValues("https://example.com")},
	}
	resource := newMockResolvedResource("test-value")
	annotatedResource := cache.Add(resolverType, params, resource)

	if got, ok := cache.Get(resolverType, params); !ok {
		t.Errorf("Get() = %v, %v, want resource, true", got, ok)
	} else {
		// Verify it's an annotated resource
		if string(got.Data()) != "test-value" {
			t.Errorf("Expected data 'test-value', got %s", string(got.Data()))
		}
		if got.Annotations()[cacheOperationKey] != cacheOperationRetrieve {
			t.Errorf("Expected retrieve annotation, got %s", got.Annotations()[cacheOperationKey])
		}
	}

	// Verify that Add returns an annotated resource
	if string(annotatedResource.Data()) != "test-value" {
		t.Errorf("Expected annotated resource data 'test-value', got %s", string(annotatedResource.Data()))
	}
	if annotatedResource.Annotations()[cacheOperationKey] != cacheOperationStore {
		t.Errorf("Expected store annotation, got %s", annotatedResource.Annotations()[cacheOperationKey])
	}

	// Test expiration with short duration for faster tests
	shortExpiration := 10 * time.Millisecond
	expiringParams := []pipelinev1.Param{
		{Name: "url", Value: *pipelinev1.NewStructuredValues("https://expiring.com")},
	}
	expiringResource := newMockResolvedResource("expiring-value")
	cache.AddWithExpiration("http", expiringParams, expiringResource, shortExpiration)

	// Wait for expiration with polling to make test more reliable
	expired := false
	for range 20 { // Max 200ms wait
		time.Sleep(10 * time.Millisecond)
		if _, ok := cache.Get("http", expiringParams); !ok {
			expired = true
			break
		}
	}

	if !expired {
		t.Error("Key should have expired but was still found in cache")
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
	resolverType := "bundle"
	params := []pipelinev1.Param{
		{Name: "bundle", Value: *pipelinev1.NewStructuredValues("gcr.io/test/bundle:v1.0.0")},
	}
	resource := newMockResolvedResource("test-value")
	annotatedResource := cache.Add(resolverType, params, resource)

	if v, found := cache.Get(resolverType, params); !found {
		t.Error("Expected to find value in cache")
	} else if string(v.Data()) != "test-value" {
		t.Errorf("Expected data 'test-value', got %s", string(v.Data()))
	}

	// Verify Add returns annotated resource
	if string(annotatedResource.Data()) != "test-value" {
		t.Errorf("Expected annotated resource data 'test-value', got %s", string(annotatedResource.Data()))
	}

	// Test Remove - generate key for removal since Remove still uses key-based API
	key := generateCacheKey(resolverType, params)
	cache.Remove(key)
	if _, found := cache.Get(resolverType, params); found {
		t.Error("Expected key to be removed")
	}

	// Test AddWithExpiration
	customTTL := 1 * time.Second
	expirationParams := []pipelinev1.Param{
		{Name: "bundle", Value: *pipelinev1.NewStructuredValues("gcr.io/test/expiring:v1.0.0")},
	}
	expiringResource := newMockResolvedResource("expiring-value")
	cache.AddWithExpiration("bundle", expirationParams, expiringResource, customTTL)

	if v, found := cache.Get("bundle", expirationParams); !found {
		t.Error("Expected to find value in cache")
	} else if string(v.Data()) != "expiring-value" {
		t.Errorf("Expected data 'expiring-value', got %s", string(v.Data()))
	}

	// Wait for expiration with polling for more reliable test
	expired := false
	maxWait := customTTL + 100*time.Millisecond
	iterations := int(maxWait / (10 * time.Millisecond))
	for range iterations {
		time.Sleep(10 * time.Millisecond)
		if _, found := cache.Get("bundle", expirationParams); !found {
			expired = true
			break
		}
	}

	if !expired {
		t.Error("Expected key to be expired but was still found in cache")
	}
}
