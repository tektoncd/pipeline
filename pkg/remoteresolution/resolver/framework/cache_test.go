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

package framework_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	cacheinjection "github.com/tektoncd/pipeline/pkg/remoteresolution/cache/injection"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

// mockImmutabilityChecker implements the ImmutabilityChecker interface for testing
type mockImmutabilityChecker struct {
	immutable bool
}

func (r *mockImmutabilityChecker) IsImmutable(ctx context.Context, params []pipelinev1.Param) bool {
	return r.immutable
}

func TestShouldUseCache(t *testing.T) {
	tests := []struct {
		name         string
		params       []pipelinev1.Param
		configMap    map[string]string
		resolverType string
		immutable    bool
		expected     bool
	}{
		// Test 1: Task parameter has highest priority
		{
			name: "task param always overrides config and system defaults",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: framework.CacheModeAlways}},
			},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeNever},
			resolverType: "test",
			immutable:    false,
			expected:     true,
		},
		{
			name: "task param 'never overrides config and system defaults",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: framework.CacheModeNever}},
			},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeAlways},
			resolverType: "test",
			immutable:    true,
			expected:     false,
		},
		{
			name: "task param auto overrides config and system defaults with immutable true",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: framework.CacheModeAuto}},
			},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeNever},
			resolverType: "test",
			immutable:    true,
			expected:     true,
		},
		{
			name: "task param auto overrides config and system defaults with immutable false",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: framework.CacheModeAuto}},
			},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeAlways},
			resolverType: "test",
			immutable:    false,
			expected:     false,
		},

		// Test 2: ConfigMap has middle priority when no task param
		{
			name: "config always when no task param",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeAlways},
			resolverType: "test",
			immutable:    false,
			expected:     true,
		},
		{
			name: "config never when no task param",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeNever},
			resolverType: "test",
			immutable:    true,
			expected:     false,
		},
		{
			name: "config auto when no task param with immutable true",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeAuto},
			resolverType: "test",
			immutable:    true,
			expected:     true,
		},
		{
			name: "config auto when no task param with immutable false",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeAuto},
			resolverType: "test",
			immutable:    false,
			expected:     false,
		},

		// Test 3: System default has lowest priority when no task param or config (always "auto")
		{
			name: "system default auto when no config with immutable true",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:    map[string]string{},
			resolverType: "test",
			immutable:    true,
			expected:     true,
		},
		{
			name: "system default auto when no config with immutable false",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:    map[string]string{},
			resolverType: "test",
			immutable:    false,
			expected:     false,
		},

		// Test 4: Invalid cache modes default to auto
		{
			name: "invalid task param defaults to auto with immutable true",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: "invalid-mode"}},
			},
			configMap:    map[string]string{},
			resolverType: "test",
			immutable:    true,
			expected:     true,
		},
		{
			name: "invalid task param defaults to auto with immutable false",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: "invalid-mode"}},
			},
			configMap:    map[string]string{},
			resolverType: "test",
			immutable:    false,
			expected:     false,
		},
		{
			name: "invalid config defaults to auto with immutable true",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:    map[string]string{"default-cache-mode": "invalid-mode"},
			resolverType: "test",
			immutable:    true,
			expected:     true,
		},
		{
			name: "invalid config defaults to auto with immutable false",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:    map[string]string{"default-cache-mode": "invalid-mode"},
			resolverType: "test",
			immutable:    false,
			expected:     false,
		},

		// Test 5: Empty params
		{
			name:         "empty params uses config",
			params:       []pipelinev1.Param{},
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeAlways},
			resolverType: "test",
			immutable:    false,
			expected:     true,
		},

		// Test 6: Nil params (edge case)
		{
			name:         "nil params uses config",
			params:       nil,
			configMap:    map[string]string{"default-cache-mode": framework.CacheModeNever},
			resolverType: "test",
			immutable:    true,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create resolver with specified immutability
			resolver := &mockImmutabilityChecker{immutable: tt.immutable}

			// Create context with config
			ctx := t.Context()
			ctx = resolutionframework.InjectResolverConfigToContext(ctx, tt.configMap)

			// Test ShouldUseCache
			result := framework.ShouldUseCache(ctx, resolver, tt.params, tt.resolverType)

			if result != tt.expected {
				t.Errorf("ShouldUseCache() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestShouldUseCachePrecedence(t *testing.T) {
	tests := []struct {
		name           string
		taskCacheParam string            // cache parameter from task/ResolutionRequest
		configMap      map[string]string // resolver ConfigMap
		immutable      bool              // whether the resolver considers params immutable
		expected       bool              // expected result
		description    string            // test case description
	}{
		// Test case 1: Default behavior (no config, no task param) -> should be "auto"
		{
			name:           "no_config_no_task_param_immutable",
			taskCacheParam: "",                  // no cache param in task
			configMap:      map[string]string{}, // no default-cache-mode in ConfigMap
			immutable:      true,                // resolver says it's immutable (like digest)
			expected:       true,                // auto mode + immutable = cache
			description:    "No config anywhere, defaults to auto, immutable should be cached",
		},
		{
			name:           "no_config_no_task_param_mutable",
			taskCacheParam: "",                  // no cache param in task
			configMap:      map[string]string{}, // no default-cache-mode in ConfigMap
			immutable:      false,               // resolver says it's mutable (like tag)
			expected:       false,               // auto mode + mutable = no cache
			description:    "No config anywhere, defaults to auto, mutable should not be cached",
		},

		// Test case 2: ConfigMap default-cache-mode takes precedence over system default
		{
			name:           "configmap_always_no_task_param",
			taskCacheParam: "",                                                // no cache param in task
			configMap:      map[string]string{"default-cache-mode": "always"}, // ConfigMap says always
			immutable:      false,                                             // resolver says it's mutable
			expected:       true,                                              // always mode = cache regardless
			description:    "ConfigMap always overrides auto default, should cache even when mutable",
		},
		{
			name:           "configmap_never_no_task_param",
			taskCacheParam: "",                                               // no cache param in task
			configMap:      map[string]string{"default-cache-mode": "never"}, // ConfigMap says never
			immutable:      true,                                             // resolver says it's immutable
			expected:       false,                                            // never mode = no cache regardless
			description:    "ConfigMap never overrides auto default, should not cache even when immutable",
		},
		{
			name:           "configmap_auto_no_task_param_immutable",
			taskCacheParam: "",                                              // no cache param in task
			configMap:      map[string]string{"default-cache-mode": "auto"}, // ConfigMap says auto (explicit)
			immutable:      true,                                            // resolver says it's immutable
			expected:       true,                                            // auto mode + immutable = cache
			description:    "ConfigMap auto explicit, immutable should be cached",
		},

		// Test case 3: Task cache parameter has highest precedence
		{
			name:           "configmap_always_task_never",
			taskCacheParam: "never",                                           // task says never
			configMap:      map[string]string{"default-cache-mode": "always"}, // ConfigMap says always
			immutable:      true,                                              // resolver says it's immutable
			expected:       false,                                             // task param wins: never = no cache
			description:    "Task param never overrides ConfigMap always, should not cache",
		},
		{
			name:           "configmap_never_task_always",
			taskCacheParam: "always",                                         // task says always
			configMap:      map[string]string{"default-cache-mode": "never"}, // ConfigMap says never
			immutable:      false,                                            // resolver says it's mutable
			expected:       true,                                             // task param wins: always = cache
			description:    "Task param always overrides ConfigMap never, should cache",
		},
		{
			name:           "configmap_auto_task_always",
			taskCacheParam: "always",                                        // task says always
			configMap:      map[string]string{"default-cache-mode": "auto"}, // ConfigMap says auto
			immutable:      false,                                           // resolver says it's mutable
			expected:       true,                                            // task param wins: always = cache
			description:    "Task param always overrides ConfigMap auto, should cache even when mutable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create params with cache param if provided
			var params []pipelinev1.Param
			if tt.taskCacheParam != "" {
				params = append(params, pipelinev1.Param{
					Name:  framework.CacheParam,
					Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: tt.taskCacheParam},
				})
			}

			// Setup context with resolver config
			ctx := t.Context()
			if len(tt.configMap) > 0 {
				ctx = resolutionframework.InjectResolverConfigToContext(ctx, tt.configMap)
			}

			// Create mock resolver with specified immutability
			resolver := &mockImmutabilityChecker{immutable: tt.immutable}

			// Test the cache decision logic using the framework directly
			actual := framework.ShouldUseCache(ctx, resolver, params, "test")

			if actual != tt.expected {
				t.Errorf("framework.ShouldUseCache() = %v, want %v\nDescription: %s", actual, tt.expected, tt.description)
			}
		})
	}
}

func TestValidateCacheMode(t *testing.T) {
	tests := []struct {
		name      string
		cacheMode string
		expected  string
		wantError bool
	}{
		{
			name:      "valid always mode",
			cacheMode: framework.CacheModeAlways,
			expected:  framework.CacheModeAlways,
			wantError: false,
		},
		{
			name:      "valid never mode",
			cacheMode: framework.CacheModeNever,
			expected:  framework.CacheModeNever,
			wantError: false,
		},
		{
			name:      "valid auto mode",
			cacheMode: framework.CacheModeAuto,
			expected:  framework.CacheModeAuto,
			wantError: false,
		},
		{
			name:      "invalid mode returns error",
			cacheMode: "invalid-mode",
			expected:  "",
			wantError: true,
		},
		{
			name:      "empty mode returns error",
			cacheMode: "",
			expected:  "",
			wantError: true,
		},
		{
			name:      "case sensitive - Always returns error",
			cacheMode: "Always",
			expected:  "",
			wantError: true,
		},
		{
			name:      "case sensitive - NEVER returns error",
			cacheMode: "NEVER",
			expected:  "",
			wantError: true,
		},
		{
			name:      "case sensitive - Auto returns error",
			cacheMode: "Auto",
			expected:  "",
			wantError: true,
		},
		{
			name:      "whitespace mode returns error",
			cacheMode: " always ",
			expected:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := framework.ValidateCacheMode(tt.cacheMode)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateCacheMode(%q) expected error but got none", tt.cacheMode)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateCacheMode(%q) unexpected error: %v", tt.cacheMode, err)
				}
			}

			if result != tt.expected {
				t.Errorf("ValidateCacheMode(%q) = %q, expected %q", tt.cacheMode, result, tt.expected)
			}
		})
	}
}

func TestCacheConstants(t *testing.T) {
	// Test that constants are defined correctly
	if framework.CacheModeAlways != "always" {
		t.Errorf("CacheModeAlways = %q, expected 'always'", framework.CacheModeAlways)
	}
	if framework.CacheModeNever != "never" {
		t.Errorf("CacheModeNever = %q, expected 'never'", framework.CacheModeNever)
	}
	if framework.CacheModeAuto != "auto" {
		t.Errorf("CacheModeAuto = %q, expected 'auto'", framework.CacheModeAuto)
	}
	if framework.CacheParam != "cache" {
		t.Errorf("CacheParam = %q, expected 'cache'", framework.CacheParam)
	}
}

// mockBundleImmutabilityChecker mimics bundle resolver's IsImmutable logic for testing
type mockBundleImmutabilityChecker struct{}

func (m *mockBundleImmutabilityChecker) IsImmutable(ctx context.Context, params []pipelinev1.Param) bool {
	// Extract bundle reference from params (mimics bundle resolver logic)
	var bundleRef string
	for _, param := range params {
		if param.Name == "bundle" {
			bundleRef = param.Value.StringVal
			break
		}
	}
	// Check if bundle is specified by digest (immutable) vs tag (mutable)
	return strings.Contains(bundleRef, "@sha256:")
}

// mockResolvedResource implements resolutionframework.ResolvedResource for testing
type mockResolvedResource struct {
	data        []byte
	annotations map[string]string
}

func (m *mockResolvedResource) Data() []byte {
	return m.data
}

func (m *mockResolvedResource) Annotations() map[string]string {
	return m.annotations
}

func (m *mockResolvedResource) RefSource() *pipelinev1.RefSource {
	return nil
}

func TestShouldUseCacheBundleResolver(t *testing.T) {
	// Test cache decision logic specifically for bundle resolver scenarios
	// This was moved from bundle/resolver_test.go per twoGiants feedback to centralize
	// all cache decision tests in the framework package
	tests := []struct {
		name        string
		cacheParam  string
		bundleRef   string
		expectCache bool
		description string
	}{
		{
			name:        "cache_always_with_tag",
			cacheParam:  "always",
			bundleRef:   "registry.io/repo:v1.0.0",
			expectCache: true,
			description: "cache=always should use cache even with tag",
		},
		{
			name:        "cache_never_with_digest",
			cacheParam:  "never",
			bundleRef:   "registry.io/repo@sha256:abcdef1234567890",
			expectCache: false,
			description: "cache=never should not use cache even with digest",
		},
		{
			name:        "cache_auto_with_digest",
			cacheParam:  "auto",
			bundleRef:   "registry.io/repo@sha256:abcdef1234567890",
			expectCache: true,
			description: "cache=auto with digest should use cache",
		},
		{
			name:        "cache_auto_with_tag",
			cacheParam:  "auto",
			bundleRef:   "registry.io/repo:v1.0.0",
			expectCache: false,
			description: "cache=auto with tag should not use cache",
		},
		{
			name:        "no_cache_param_with_digest",
			cacheParam:  "",
			bundleRef:   "registry.io/repo@sha256:abcdef1234567890",
			expectCache: true,
			description: "no cache param defaults to auto, digest should use cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &mockBundleImmutabilityChecker{}

			// Create params
			var params []pipelinev1.Param
			if tt.cacheParam != "" {
				params = append(params, pipelinev1.Param{
					Name:  framework.CacheParam,
					Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: tt.cacheParam},
				})
			}
			params = append(params, []pipelinev1.Param{
				{
					Name:  "bundle",
					Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: tt.bundleRef},
				},
				{
					Name:  "kind",
					Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "task"},
				},
				{
					Name:  "name",
					Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "test-task"},
				},
			}...)

			ctx := t.Context()

			// Test the cache decision logic
			actual := framework.ShouldUseCache(ctx, resolver, params, "bundle")

			if actual != tt.expectCache {
				t.Errorf("framework.ShouldUseCache() = %v, want %v\nDescription: %s", actual, tt.expectCache, tt.description)
			}
		})
	}
}

func TestRunCacheOperations(t *testing.T) {
	tests := []struct {
		name           string
		cachedResource resolutionframework.ResolvedResource
		cacheHit       bool
		resolveError   error
		expectError    bool
		description    string
	}{
		{
			name:           "cache_hit_returns_cached_resource",
			cachedResource: &mockResolvedResource{data: []byte("cached-content"), annotations: map[string]string{"test": "cached"}},
			cacheHit:       true,
			expectError:    false,
			description:    "When resource exists in cache, should return cached resource without calling resolve function",
		},
		{
			name:        "cache_miss_resolves_and_stores",
			cacheHit:    false,
			expectError: false,
			description: "When resource not in cache, should call resolve function and store result in cache",
		},
		{
			name:         "cache_miss_with_resolve_error",
			cacheHit:     false,
			resolveError: errors.New("resolution failed"),
			expectError:  true,
			description:  "When resolve function returns error, should propagate the error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake cache
			testCache := cache.NewResolverCache(100)

			// Create test parameters
			params := []pipelinev1.Param{
				{Name: "url", Value: *pipelinev1.NewStructuredValues("https://example.com")},
			}
			resolverType := "test"

			// Set up cache if this is a cache hit scenario
			if tt.cacheHit {
				testCache.Add(resolverType, params, tt.cachedResource)
			}

			// Create context with cache injection
			ctx := t.Context()
			ctx = context.WithValue(ctx, cacheinjection.Key{}, testCache)

			// Track if resolve function was called
			resolveCalled := false
			resolveFunc := func() (resolutionframework.ResolvedResource, error) {
				resolveCalled = true
				if tt.resolveError != nil {
					return nil, tt.resolveError
				}
				return &mockResolvedResource{
					data:        []byte("resolved-content"),
					annotations: map[string]string{"test": "resolved"},
				}, nil
			}

			// Call RunCacheOperations
			result, err := framework.RunCacheOperations(ctx, params, resolverType, resolveFunc)

			// Validate error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Validate result is not nil
			if result == nil {
				t.Error("Expected result but got nil")
				return
			}

			// Validate cache hit vs miss behavior
			if tt.cacheHit {
				if resolveCalled {
					t.Error("Resolve function should not be called on cache hit")
				}
				if string(result.Data()) != "cached-content" {
					t.Errorf("Expected cached content, got %s", string(result.Data()))
				}
			} else {
				if !resolveCalled {
					t.Error("Resolve function should be called on cache miss")
				}
				if string(result.Data()) != "resolved-content" {
					t.Errorf("Expected resolved content, got %s", string(result.Data()))
				}

				// Verify the resource was stored in cache for future hits
				cached, found := testCache.Get(resolverType, params)
				if !found {
					t.Error("Resource should be stored in cache after resolution")
				} else if string(cached.Data()) != "resolved-content" {
					t.Errorf("Expected resolved content in cache, got %s", string(cached.Data()))
				}
			}
		})
	}
}
