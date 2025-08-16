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
	"testing"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	resolvedresource "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

// mockCacheAwareResolver implements the CacheAwareResolver interface for testing
type mockCacheAwareResolver struct {
	immutable bool
}

func (r *mockCacheAwareResolver) IsImmutable(ctx context.Context, req *v1beta1.ResolutionRequestSpec) bool {
	return r.immutable
}

func (r *mockCacheAwareResolver) Initialize(ctx context.Context) error {
	return nil
}

func (r *mockCacheAwareResolver) GetName(ctx context.Context) string {
	return "test-resolver"
}

func (r *mockCacheAwareResolver) GetSelector(ctx context.Context) map[string]string {
	return map[string]string{}
}

func (r *mockCacheAwareResolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	return nil
}

func (r *mockCacheAwareResolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolvedresource.ResolvedResource, error) {
	return nil, errors.New("mock resolver - not implemented")
}

func TestShouldUseCache(t *testing.T) {
	tests := []struct {
		name          string
		params        []pipelinev1.Param
		configMap     map[string]string
		systemDefault string
		immutable     bool
		expected      bool
	}{
		// Test 1: Task parameter has highest priority
		{
			name: "task param always overrides config and system defaults",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: framework.CacheModeAlways}},
			},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeNever},
			systemDefault: framework.CacheModeNever,
			immutable:     false,
			expected:      true,
		},
		{
			name: "task param 'never overrides config and system defaults",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: framework.CacheModeNever}},
			},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeAlways},
			systemDefault: framework.CacheModeAlways,
			immutable:     true,
			expected:      false,
		},
		{
			name: "task param auto overrides config and system defaults with immutable true",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: framework.CacheModeAuto}},
			},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeNever},
			systemDefault: framework.CacheModeNever,
			immutable:     true,
			expected:      true,
		},
		{
			name: "task param auto overrides config and system defaults with immutable false",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: framework.CacheModeAuto}},
			},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeAlways},
			systemDefault: framework.CacheModeAlways,
			immutable:     false,
			expected:      false,
		},

		// Test 2: ConfigMap has middle priority when no task param
		{
			name: "config always when no task param",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeAlways},
			systemDefault: framework.CacheModeNever,
			immutable:     false,
			expected:      true,
		},
		{
			name: "config never when no task param",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeNever},
			systemDefault: framework.CacheModeAlways,
			immutable:     true,
			expected:      false,
		},
		{
			name: "config auto when no task param with immutable true",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeAuto},
			systemDefault: framework.CacheModeNever,
			immutable:     true,
			expected:      true,
		},
		{
			name: "config auto when no task param with immutable false",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeAuto},
			systemDefault: framework.CacheModeAlways,
			immutable:     false,
			expected:      false,
		},

		// Test 3: System default has lowest priority when no task param or config
		{
			name: "system default always when no config",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{},
			systemDefault: framework.CacheModeAlways,
			immutable:     false,
			expected:      true,
		},
		{
			name: "system default never when no config",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{},
			systemDefault: framework.CacheModeNever,
			immutable:     true,
			expected:      false,
		},
		{
			name: "system default auto when no config with immutable true",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{},
			systemDefault: framework.CacheModeAuto,
			immutable:     true,
			expected:      true,
		},
		{
			name: "system default auto when no config with immutable false",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{},
			systemDefault: framework.CacheModeAuto,
			immutable:     false,
			expected:      false,
		},

		// Test 4: Invalid cache modes default to auto
		{
			name: "invalid task param defaults to auto with immutable true",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: "invalid-mode"}},
			},
			configMap:     map[string]string{},
			systemDefault: framework.CacheModeNever,
			immutable:     true,
			expected:      true,
		},
		{
			name: "invalid task param defaults to auto with immutable false",
			params: []pipelinev1.Param{
				{Name: framework.CacheParam, Value: pipelinev1.ParamValue{StringVal: "invalid-mode"}},
			},
			configMap:     map[string]string{},
			systemDefault: framework.CacheModeAlways,
			immutable:     false,
			expected:      false,
		},
		{
			name: "invalid config defaults to auto with immutable true",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{"default-cache-mode": "invalid-mode"},
			systemDefault: framework.CacheModeNever,
			immutable:     true,
			expected:      true,
		},
		{
			name: "invalid config defaults to auto with immutable false",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{"default-cache-mode": "invalid-mode"},
			systemDefault: framework.CacheModeAlways,
			immutable:     false,
			expected:      false,
		},
		{
			name: "invalid system default defaults to auto with immutable true",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{},
			systemDefault: "invalid-mode",
			immutable:     true,
			expected:      true,
		},
		{
			name: "invalid system default defaults to auto with immutable false",
			params: []pipelinev1.Param{
				{Name: "other-param", Value: pipelinev1.ParamValue{StringVal: "value"}},
			},
			configMap:     map[string]string{},
			systemDefault: "invalid-mode",
			immutable:     false,
			expected:      false,
		},

		// Test 5: Empty params
		{
			name:          "empty params uses config",
			params:        []pipelinev1.Param{},
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeAlways},
			systemDefault: framework.CacheModeNever,
			immutable:     false,
			expected:      true,
		},

		// Test 6: Nil params (edge case)
		{
			name:          "nil params uses config",
			params:        nil,
			configMap:     map[string]string{"default-cache-mode": framework.CacheModeNever},
			systemDefault: framework.CacheModeAlways,
			immutable:     true,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create resolver with specified immutability
			resolver := &mockCacheAwareResolver{immutable: tt.immutable}

			// Create context with config
			ctx := context.Background()
			ctx = resolutionframework.InjectResolverConfigToContext(ctx, tt.configMap)

			// Create request spec
			req := &v1beta1.ResolutionRequestSpec{
				Params: tt.params,
			}

			// Test ShouldUseCache
			result := framework.ShouldUseCache(ctx, resolver, req, tt.systemDefault)

			if result != tt.expected {
				t.Errorf("ShouldUseCache() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetSystemDefaultCacheMode(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		expected     string
	}{
		{
			name:         "git resolver",
			resolverType: "git",
			expected:     framework.CacheModeAuto,
		},
		{
			name:         "bundle resolver",
			resolverType: "bundle",
			expected:     framework.CacheModeAuto,
		},
		{
			name:         "cluster resolver",
			resolverType: "cluster",
			expected:     framework.CacheModeAuto,
		},
		{
			name:         "http resolver",
			resolverType: "http",
			expected:     framework.CacheModeAuto,
		},
		{
			name:         "unknown resolver",
			resolverType: "unknown",
			expected:     framework.CacheModeAuto,
		},
		{
			name:         "empty resolver type",
			resolverType: "",
			expected:     framework.CacheModeAuto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := framework.GetSystemDefaultCacheMode(tt.resolverType)
			if result != tt.expected {
				t.Errorf("GetSystemDefaultCacheMode(%q) = %q, expected %q", tt.resolverType, result, tt.expected)
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
