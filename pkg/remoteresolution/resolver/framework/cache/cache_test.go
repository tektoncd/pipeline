/*
Copyright 2024 The Tekton Authors

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
					Name:      getCacheConfigName(),
					Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
				},
				Data: map[string]string{
					"max-size": "100",
				},
			},
			expectedSize:   100,
			expectedTTL:    defaultExpiration,
			shouldRecreate: true,
		},
		{
			name: "cache config with maxSize and expiration",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getCacheConfigName(),
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
					Name:      getCacheConfigName(),
					Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
				},
				Data: map[string]string{
					"max-size":    "150",
					"default-ttl": "invalid",
				},
			},
			expectedSize:   150,
			expectedTTL:    defaultExpiration,
			shouldRecreate: true,
		},
		{
			name:           "nil config map",
			configMap:      nil,
			expectedSize:   defaultCacheSize,
			expectedTTL:    defaultExpiration,
			shouldRecreate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original defaultExpiration to restore later
			originalTTL := defaultExpiration
			defer func() { defaultExpiration = originalTTL }()

			cache := newResolverCache(defaultCacheSize)
			originalCache := cache.cache

			cache.initializeFromConfigMap(tt.configMap)

			// Verify cache size
			if tt.shouldRecreate && cache.cache == originalCache {
				t.Error("Expected cache to be recreated with new size")
			}

			// Verify TTL (InitializeFromConfigMap modifies the global defaultExpiration)
			if defaultExpiration != tt.expectedTTL {
				t.Errorf("Expected TTL %v, got %v", tt.expectedTTL, defaultExpiration)
			}
		})
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
			expectedKey:  "1c31dda07cb1e09e89bd660a8d114936b44f728b73a3bc52c69a409ee1d44e67",
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
			expectedKey: "63f68e3e567eafd7efb4149b3389b3261784c8ac5847b62e90b7ae8d23f6e889",
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
