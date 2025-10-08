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

package framework

import (
	"context"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

// Cache mode constants - shared across all resolvers
const (
	CacheModeAlways = "always"
	CacheModeNever  = "never"
	CacheModeAuto   = "auto"
	CacheParam      = "cache"
)

// ImmutabilityChecker extends the base Resolver interface with cache-specific methods.
// Each resolver implements IsImmutable to define what "auto" mode means in their context.
type ImmutabilityChecker interface {
	IsImmutable(params []v1.Param) bool
}

// ShouldUseCache determines whether caching should be used based on:
// 1. Task/Pipeline cache parameter (highest priority)
// 2. ConfigMap default-cache-mode (middle priority)
// 3. System default for resolver type (lowest priority)
func ShouldUseCache(ctx context.Context, resolver ImmutabilityChecker, params []v1.Param, resolverType string) bool {
	// Get cache mode from task parameter
	cacheMode := ""
	for _, param := range params {
		if param.Name == CacheParam {
			cacheMode = param.Value.StringVal
			break
		}
	}

	// If no task parameter, get default from ConfigMap
	if cacheMode == "" {
		conf := resolutionframework.GetResolverConfigFromContext(ctx)
		if defaultMode, ok := conf["default-cache-mode"]; ok {
			cacheMode = defaultMode
		}
	}

	// If still no mode, use system default
	if cacheMode == "" {
		cacheMode = systemDefaultCacheMode(resolverType)
	}

	// Apply cache mode logic
	switch cacheMode {
	case CacheModeAlways:
		return true
	case CacheModeNever:
		return false
	case CacheModeAuto:
		return resolver.IsImmutable(params)
	default:
		// Invalid mode defaults to auto
		return resolver.IsImmutable(params)
	}
}

// systemDefaultCacheMode returns the system default cache mode for a resolver type.
// This can be customized per resolver if needed.
func systemDefaultCacheMode(resolverType string) string {
	return CacheModeAuto
}

// ValidateCacheMode validates cache mode parameters.
// Returns an error for invalid cache modes to ensure consistent validation across all resolvers.
func ValidateCacheMode(cacheMode string) (string, error) {
	switch cacheMode {
	case CacheModeAlways, CacheModeNever, CacheModeAuto:
		return cacheMode, nil // Valid cache mode
	default:
		return "", fmt.Errorf("invalid cache mode '%s', must be one of: always, never, auto", cacheMode)
	}
}
