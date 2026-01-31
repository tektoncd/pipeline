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
	"strconv"
	"testing"
	"time"

	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestOnCacheConfigMapChanged(t *testing.T) {
	tests := []struct {
		name            string
		configMap       *corev1.ConfigMap
		expectedMaxSize int
		expectedTTL     time.Duration
	}{
		{
			name:            "nil ConfigMap returns defaults",
			configMap:       nil,
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     defaultExpiration,
		},
		{
			name: "empty ConfigMap returns defaults",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{},
			},
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     defaultExpiration,
		},
		{
			name: "ConfigMap with custom max-size",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"max-size": "500",
				},
			},
			expectedMaxSize: 500,
			expectedTTL:     defaultExpiration,
		},
		{
			name: "ConfigMap with custom ttl",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"ttl": "10m",
				},
			},
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     10 * time.Minute,
		},
		{
			name: "ConfigMap with both max-size and ttl",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"max-size": "2000",
					"ttl":      "1h",
				},
			},
			expectedMaxSize: 2000,
			expectedTTL:     1 * time.Hour,
		},
		{
			name: "ConfigMap with invalid max-size uses default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"max-size": "invalid",
				},
			},
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     defaultExpiration,
		},
		{
			name: "ConfigMap with negative max-size uses default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"max-size": "-100",
				},
			},
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     defaultExpiration,
		},
		{
			name: "ConfigMap with zero max-size uses default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"max-size": "0",
				},
			},
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     defaultExpiration,
		},
		{
			name: "ConfigMap with invalid ttl uses default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"ttl": "invalid",
				},
			},
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     defaultExpiration,
		},
		{
			name: "ConfigMap with negative ttl uses default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"ttl": "-5m",
				},
			},
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     defaultExpiration,
		},
		{
			name: "ConfigMap with zero ttl uses default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"ttl": "0",
				},
			},
			expectedMaxSize: defaultCacheSize,
			expectedTTL:     defaultExpiration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := resolutionframework.DataFromConfigMap(tt.configMap)
			if err != nil {
				t.Fatalf("DataFromConfigMap() returned error: %v", err)
			}

			onCacheConfigChanged("test-config", data)
			cache := Get(logtesting.TestContextWithLogger(t))

			if cache.MaxSize() != tt.expectedMaxSize {
				t.Errorf("MaxSize = %d, want %d", cache.MaxSize(), tt.expectedMaxSize)
			}

			if cache.TTL() != tt.expectedTTL {
				t.Errorf("TTL = %v, want %v", cache.TTL(), tt.expectedTTL)
			}
		})
	}
}

func TestOnCacheConfigChangedWithInvalidType(t *testing.T) {
	// First, set up a known good config
	goodConfig := map[string]string{
		"max-size": strconv.Itoa(defaultCacheSize),
		"ttl":      defaultExpiration.String(),
	}
	onCacheConfigChanged("test-config", goodConfig)

	ctx := logtesting.TestContextWithLogger(t)
	cacheBefore := Get(ctx)
	if cacheBefore == nil {
		t.Fatal("Expected cache before invalid config change")
	}
	ttlBefore := cacheBefore.TTL()
	maxSizeBefore := cacheBefore.MaxSize()

	// Test that onCacheConfigChanged handles invalid types gracefully
	// This should not panic and should preserve the existing cache
	onCacheConfigChanged("test-config", "invalid-type")

	// Verify we can still get the cache and it wasn't modified
	cacheAfter := Get(ctx)
	if cacheAfter == nil {
		t.Fatal("Expected cache after invalid config change but got nil")
	}

	// Verify cache config wasn't changed by invalid input
	if cacheAfter.TTL() != ttlBefore {
		t.Errorf("Expected TTL to remain %v after invalid config, got %v", ttlBefore, cacheAfter.TTL())
	}

	if cacheAfter.MaxSize() != maxSizeBefore {
		t.Errorf("Expected MaxSize to remain %d after invalid config, got %d", maxSizeBefore, cacheAfter.MaxSize())
	}
}
