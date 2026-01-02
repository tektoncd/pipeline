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
	"os"
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
			config, err := NewConfigFromConfigMap(tt.configMap)
			if err != nil {
				t.Fatalf("NewConfigFromConfigMap() returned error: %v", err)
			}

			if config.MaxSize != tt.expectedMaxSize {
				t.Errorf("MaxSize = %d, want %d", config.MaxSize, tt.expectedMaxSize)
			}

			if config.TTL != tt.expectedTTL {
				t.Errorf("TTL = %v, want %v", config.TTL, tt.expectedTTL)
			}
		})
	}
}

func TestOnCacheConfigChanged(t *testing.T) {
	ctx := logtesting.TestContextWithLogger(t)

	// Ensure cache is initialized first
	_ = Get(ctx)

	// Test that onCacheConfigChanged updates the shared cache with new config values
	config := &Config{
		MaxSize: 500,
		TTL:     10 * time.Minute,
	}

	// Call onCacheConfigChanged to update the shared cache
	onCacheConfigChanged("test-config", config)

	// Verify the shared cache was updated with the correct config values
	cache := Get(ctx)
	if cache == nil {
		t.Fatal("Expected cache after config change but got nil")
	}

	// Verify TTL was applied
	if cache.TTL() != config.TTL {
		t.Errorf("Expected TTL to be %v, got %v", config.TTL, cache.TTL())
	}

	// Verify MaxSize was applied
	if cache.MaxSize() != config.MaxSize {
		t.Errorf("Expected MaxSize to be %d, got %d", config.MaxSize, cache.MaxSize())
	}
}

func TestOnCacheConfigChangedWithInvalidType(t *testing.T) {
	// First, set up a known good config
	goodConfig := &Config{
		MaxSize: defaultCacheSize,
		TTL:     defaultExpiration,
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

func TestGetCacheConfigName(t *testing.T) {
	// Save and restore env var
	originalValue := os.Getenv(resolverCacheConfigMapNameEnv)
	defer func() {
		if originalValue == "" {
			os.Unsetenv(resolverCacheConfigMapNameEnv)
		} else {
			os.Setenv(resolverCacheConfigMapNameEnv, originalValue)
		}
	}()

	// Ensure env var is unset for default test
	os.Unsetenv(resolverCacheConfigMapNameEnv)

	// Test default config name
	configName := getCacheConfigName()
	if configName != defaultConfigMapName {
		t.Errorf("Expected default config name '%s', got '%s'", defaultConfigMapName, configName)
	}
}

func TestGetCacheConfigNameWithEnvVar(t *testing.T) {
	// Save and restore env var
	originalValue := os.Getenv(resolverCacheConfigMapNameEnv)
	defer func() {
		if originalValue == "" {
			os.Unsetenv(resolverCacheConfigMapNameEnv)
		} else {
			os.Setenv(resolverCacheConfigMapNameEnv, originalValue)
		}
	}()

	// Set custom config name via env var
	customName := "custom-cache-config"
	os.Setenv(resolverCacheConfigMapNameEnv, customName)

	configName := getCacheConfigName()
	if configName != customName {
		t.Errorf("Expected custom config name '%s', got '%s'", customName, configName)
	}
}

func TestCacheConfigStoreToContext(t *testing.T) {
	logger := logtesting.TestLogger(t)
	store := NewCacheConfigStore(getCacheConfigName(), logger)

	ctx := t.Context()
	ctxWithConfig := store.ToContext(ctx)

	// Verify context contains the config
	configValue := ctxWithConfig.Value(cacheConfigKey{})
	if configValue == nil {
		t.Fatal("Expected config in context but got nil")
	}

	config, ok := configValue.(*Config)
	if !ok {
		t.Fatal("Expected *Config type in context")
	}

	// Verify default values
	if config.MaxSize != defaultCacheSize {
		t.Errorf("Expected default MaxSize %d, got %d", defaultCacheSize, config.MaxSize)
	}
	if config.TTL != defaultExpiration {
		t.Errorf("Expected default TTL %v, got %v", defaultExpiration, config.TTL)
	}
}

func TestCacheConfigStoreGetResolverConfig(t *testing.T) {
	logger := logtesting.TestLogger(t)
	store := NewCacheConfigStore(getCacheConfigName(), logger)

	config := store.GetResolverConfig()
	if config == nil {
		t.Fatal("Expected config but got nil")
	}

	// Should return default values when no config is loaded
	if config.MaxSize != defaultCacheSize {
		t.Errorf("Expected default MaxSize %d, got %d", defaultCacheSize, config.MaxSize)
	}
	if config.TTL != defaultExpiration {
		t.Errorf("Expected default TTL %v, got %v", defaultExpiration, config.TTL)
	}
}

func TestNewCacheConfigStore(t *testing.T) {
	logger := logtesting.TestLogger(t)
	configName := "test-config"

	store := NewCacheConfigStore(configName, logger)
	if store == nil {
		t.Fatal("Expected store but got nil")
	}
	if store.cacheConfigName != configName {
		t.Errorf("Expected config name '%s', got '%s'", configName, store.cacheConfigName)
	}
	if store.untyped == nil {
		t.Error("Expected untyped store to be initialized")
	}
}
