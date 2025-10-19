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
	"strings"
	"testing"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	bundleresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

type resolverFake struct{}

func (r *resolverFake) IsImmutable(params []pipelinev1.Param) bool {
	// Check if bundle parameter contains a digest (@sha256:)
	for _, param := range params {
		if param.Name == bundleresolution.ParamBundle {
			bundleRef := param.Value.StringVal
			return strings.Contains(bundleRef, "@sha256:")
		}
	}
	return false
}

func TestShouldUseCachePrecedence(t *testing.T) {
	tests := []struct {
		name           string
		taskCacheParam string            // cache parameter from task/ResolutionRequest
		configMap      map[string]string // resolver ConfigMap
		bundleRef      string            // bundle reference (affects auto mode)
		expected       bool              // expected result
		description    string            // test case description
	}{
		// Test case 1: Default behavior (no config, no task param) -> should be "auto"
		{
			name:           "no_config_no_task_param_with_digest",
			taskCacheParam: "",                               // no cache param in task
			configMap:      map[string]string{},              // no default-cache-mode in ConfigMap
			bundleRef:      "registry.io/repo@sha256:abcdef", // has digest
			expected:       true,                             // auto mode + digest = cache
			description:    "No config anywhere, defaults to auto, digest should be cached",
		},
		{
			name:           "no_config_no_task_param_with_tag",
			taskCacheParam: "",                        // no cache param in task
			configMap:      map[string]string{},       // no default-cache-mode in ConfigMap
			bundleRef:      "registry.io/repo:latest", // no digest, just tag
			expected:       false,                     // auto mode + tag = no cache
			description:    "No config anywhere, defaults to auto, tag should not be cached",
		},

		// Test case 2: ConfigMap has setting, task has nothing -> should use ConfigMap value
		{
			name:           "configmap_always_no_task_param",
			taskCacheParam: "",                                                // no cache param in task
			configMap:      map[string]string{"default-cache-mode": "always"}, // ConfigMap says always
			bundleRef:      "registry.io/repo:latest",                         // irrelevant for always mode
			expected:       true,                                              // always = cache
			description:    "ConfigMap says always, no task param, should cache",
		},
		{
			name:           "configmap_never_no_task_param",
			taskCacheParam: "",                                               // no cache param in task
			configMap:      map[string]string{"default-cache-mode": "never"}, // ConfigMap says never
			bundleRef:      "registry.io/repo@sha256:abcdef",                 // irrelevant for never mode
			expected:       false,                                            // never = no cache
			description:    "ConfigMap says never, no task param, should not cache",
		},
		{
			name:           "configmap_auto_no_task_param_with_digest",
			taskCacheParam: "",                                              // no cache param in task
			configMap:      map[string]string{"default-cache-mode": "auto"}, // ConfigMap says auto
			bundleRef:      "registry.io/repo@sha256:abcdef",                // has digest
			expected:       true,                                            // auto + digest = cache
			description:    "ConfigMap says auto, no task param, digest should be cached",
		},

		// Test case 3: ConfigMap has setting AND task has setting -> task should win
		{
			name:           "configmap_always_task_never",
			taskCacheParam: "never",                                           // task says never
			configMap:      map[string]string{"default-cache-mode": "always"}, // ConfigMap says always
			bundleRef:      "registry.io/repo@sha256:abcdef",                  // irrelevant
			expected:       false,                                             // task wins: never = no cache
			description:    "Task says never, ConfigMap says always, task should win",
		},
		{
			name:           "configmap_never_task_always",
			taskCacheParam: "always",                                         // task says always
			configMap:      map[string]string{"default-cache-mode": "never"}, // ConfigMap says never
			bundleRef:      "registry.io/repo:latest",                        // irrelevant
			expected:       true,                                             // task wins: always = cache
			description:    "Task says always, ConfigMap says never, task should win",
		},
		{
			name:           "configmap_auto_task_always",
			taskCacheParam: "always",                                        // task says always
			configMap:      map[string]string{"default-cache-mode": "auto"}, // ConfigMap says auto
			bundleRef:      "registry.io/repo:latest",                       // would be false for auto mode
			expected:       true,                                            // task wins: always = cache
			description:    "Task says always, ConfigMap says auto, task should win",
		},
		{
			name:           "invalid_task_cache_param_falls_back_to_auto",
			taskCacheParam: "invalid-value",                  // task has invalid cache value
			configMap:      map[string]string{},              // no ConfigMap setting
			bundleRef:      "registry.io/repo@sha256:abcdef", // has digest
			expected:       true,                             // invalid defaults to auto + digest = cache
			description:    "Invalid task cache parameter should fall back to auto mode",
		},
		{
			name:           "invalid_configmap_value_falls_back_to_auto",
			taskCacheParam: "",                                               // no task param
			configMap:      map[string]string{"default-cache-mode": "wrong"}, // invalid ConfigMap value
			bundleRef:      "registry.io/repo@sha256:abcdef",                 // has digest
			expected:       true,                                             // invalid defaults to auto + digest = cache
			description:    "Invalid ConfigMap cache mode should fall back to auto mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up context with resolver config
			ctx := t.Context()
			if len(tt.configMap) > 0 {
				ctx = resolutionframework.InjectResolverConfigToContext(ctx, tt.configMap)
			}

			// Set up ResolutionRequestSpec
			req := &v1beta1.ResolutionRequestSpec{
				Params: []pipelinev1.Param{
					{
						Name:  bundleresolution.ParamBundle,
						Value: pipelinev1.ParamValue{StringVal: tt.bundleRef},
					},
				},
			}
			if tt.taskCacheParam != "" {
				req.Params = append(req.Params, pipelinev1.Param{
					Name:  CacheParam,
					Value: pipelinev1.ParamValue{StringVal: tt.taskCacheParam},
				})
			}

			// Test the framework function
			result := ShouldUse(ctx, &resolverFake{}, req.Params, bundleresolution.LabelValueBundleResolverType)

			// Verify result
			if result != tt.expected {
				t.Errorf("ShouldUse() = %v, expected %v\nDescription: %s", result, tt.expected, tt.description)
			}
		})
	}
}

func TestValidateCacheMode(t *testing.T) {
	tests := []struct {
		name      string
		cacheMode string
		wantErr   bool
	}{
		{
			name:      "valid always mode",
			cacheMode: "always",
			wantErr:   false,
		},
		{
			name:      "valid never mode",
			cacheMode: "never",
			wantErr:   false,
		},
		{
			name:      "valid auto mode",
			cacheMode: "auto",
			wantErr:   false,
		},
		{
			name:      "invalid mode",
			cacheMode: "invalid",
			wantErr:   true,
		},
		{
			name:      "empty mode",
			cacheMode: "",
			wantErr:   true,
		},
		{
			name:      "uppercase mode",
			cacheMode: "ALWAYS",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cacheMode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCacheMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUseCache(t *testing.T) {
	tests := []struct {
		name         string
		params       []pipelinev1.Param
		resolverType string
		cacheHit     bool
		resolveErr   error
		description  string
	}{
		{
			name: "cache hit",
			params: []pipelinev1.Param{
				{
					Name:  bundleresolution.ParamBundle,
					Value: pipelinev1.ParamValue{StringVal: "registry.io/repo@sha256:abcdef"},
				},
			},
			resolverType: bundleresolution.LabelValueBundleResolverType,
			cacheHit:     true,
			description:  "Should return cached resource",
		},
		{
			name: "cache miss - successful resolve",
			params: []pipelinev1.Param{
				{
					Name:  bundleresolution.ParamBundle,
					Value: pipelinev1.ParamValue{StringVal: "registry.io/repo:latest"},
				},
			},
			resolverType: bundleresolution.LabelValueBundleResolverType,
			cacheHit:     false,
			description:  "Should resolve and add to cache",
		},
		{
			name: "cache miss - resolve error",
			params: []pipelinev1.Param{
				{
					Name:  bundleresolution.ParamBundle,
					Value: pipelinev1.ParamValue{StringVal: "registry.io/repo:error"},
				},
			},
			resolverType: bundleresolution.LabelValueBundleResolverType,
			cacheHit:     false,
			resolveErr:   errors.New("resolve error"),
			description:  "Should return error from resolver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			// Create mock resolver function
			resolveCalled := false
			resolveFn := func() (resolutionframework.ResolvedResource, error) {
				resolveCalled = true
				if tt.resolveErr != nil {
					return nil, tt.resolveErr
				}
				return &mockResolvedResource{data: []byte("test data")}, nil
			}

			// TODO(twoGiants): doesn't look right => fix test
			// If this is a cache hit test, pre-populate the cache
			if tt.cacheHit {
				// We need to set up the cache first by calling the function once
				_, _ = GetFromCacheOrResolve(ctx, tt.params, tt.resolverType, resolveFn)
				resolveCalled = false // Reset for actual test
			}

			// Run the actual test
			result, err := GetFromCacheOrResolve(ctx, tt.params, tt.resolverType, resolveFn)

			// Verify error handling
			if tt.resolveErr != nil {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify result
			if result == nil {
				t.Errorf("Expected result but got nil")
				return
			}

			// Verify cache behavior
			if tt.cacheHit && resolveCalled {
				t.Errorf("Expected cache hit but resolve function was called")
			}
			if !tt.cacheHit && !resolveCalled {
				t.Errorf("Expected cache miss but resolve function was not called")
			}
		})
	}
}
