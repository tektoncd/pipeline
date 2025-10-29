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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestParseCacheConfigMap(t *testing.T) {
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
			config, err := parseCacheConfigMap(tt.configMap)
			if err != nil {
				t.Fatalf("parseCacheConfigMap() returned error: %v", err)
			}

			if config.maxSize != tt.expectedMaxSize {
				t.Errorf("maxSize = %d, want %d", config.maxSize, tt.expectedMaxSize)
			}

			if config.ttl != tt.expectedTTL {
				t.Errorf("ttl = %v, want %v", config.ttl, tt.expectedTTL)
			}
		})
	}
}

func TestOnCacheConfigChanged(t *testing.T) {
	// Test that onCacheConfigChanged updates the shared cache
	config := &cacheConfig{
		maxSize: 500,
		ttl:     10 * time.Minute,
	}

	// Call onCacheConfigChanged to update the shared cache
	onCacheConfigChanged("test-config", config)

	// Verify the shared cache was updated by checking we can still get it
	// We can't directly verify the size/ttl without accessing the internal cache,
	// but we can verify it doesn't panic and returns a valid cache
	ctx := logtesting.TestContextWithLogger(t)
	cache := Get(ctx)
	if cache == nil {
		t.Error("Expected cache after config change but got nil")
	}
}

func TestOnCacheConfigChangedWithInvalidType(t *testing.T) {
	// Test that onCacheConfigChanged handles invalid types gracefully
	// This should not panic
	onCacheConfigChanged("test-config", "invalid-type")

	// Verify we can still get the cache
	ctx := logtesting.TestContextWithLogger(t)
	cache := Get(ctx)
	if cache == nil {
		t.Error("Expected cache after invalid config change but got nil")
	}
}
