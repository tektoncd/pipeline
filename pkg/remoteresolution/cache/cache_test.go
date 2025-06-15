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

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		params       []pipelinev1.Param
		wantErr      bool
	}{
		{
			name:         "empty params",
			resolverType: "http",
			params:       []pipelinev1.Param{},
			wantErr:      false,
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
			wantErr: false,
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GenerateCacheKey(tt.resolverType, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateCacheKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && key == "" {
				t.Error("GenerateCacheKey() returned empty key")
			}
		})
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

	// Test global cache
	globalCache1 := GetGlobalCache()
	globalCache2 := GetGlobalCache()
	if globalCache1 != globalCache2 {
		t.Error("GetGlobalCache() returned different instances")
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
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
				Data: map[string]string{
					"max-size":    "500",
					"default-ttl": "10m",
				},
			},
			expectedSize:   500,
			expectedTTL:    10 * time.Minute,
			shouldRecreate: true,
		},
		{
			name: "invalid max size",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
				Data: map[string]string{
					"max-size":    "invalid",
					"default-ttl": "5m",
				},
			},
			expectedSize:   DefaultMaxSize,
			expectedTTL:    5 * time.Minute,
			shouldRecreate: false,
		},
		{
			name: "invalid TTL",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
				Data: map[string]string{
					"max-size":    "1000",
					"default-ttl": "invalid",
				},
			},
			expectedSize:   1000,
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
			cache := NewResolverCache(DefaultMaxSize)
			originalCache := cache.cache

			cache.InitializeFromConfigMap(tt.configMap)

			// Verify cache size
			if tt.shouldRecreate && cache.cache == originalCache {
				t.Error("Expected cache to be recreated with new size")
			}

			// Verify TTL
			if DefaultExpiration != tt.expectedTTL {
				t.Errorf("Expected TTL %v, got %v", tt.expectedTTL, DefaultExpiration)
			}
		})
	}
}

func TestCacheOperations(t *testing.T) {
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
