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
	"context"
	"fmt"
	"slices"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	cacheModeAlways              = "always"
	cacheModeNever               = "never"
	cacheModeAuto                = "auto"
	CacheParam                   = "cache"
	defaultCacheModeConfigMapKey = "default-cache-mode"
)

// ImmutabilityChecker extends the base Resolver interface with cache-specific methods.
// Each resolver implements IsImmutable to define what "auto" mode means in their context.
type ImmutabilityChecker interface {
	IsImmutable(params []v1.Param) bool
}

// ShouldUse determines whether caching should be used based on:
// 1. Task/Pipeline cache parameter (highest priority)
// 2. ConfigMap default-cache-mode (middle priority)
// 3. System default for resolver type (lowest priority)
func ShouldUse(
	ctx context.Context,
	resolver ImmutabilityChecker,
	params []v1.Param,
) bool {
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
		// This can be optionally set in individual resolver ConfigMaps (e.g., bundleresolver-config,
		// git-resolver-config, cluster-resolver-config) to override the system default cache
		// mode for that resolver. Valid values: "always", "never", "auto"
		if defaultMode, ok := conf[defaultCacheModeConfigMapKey]; ok {
			cacheMode = defaultMode
		}
	}

	// If still no mode, use system default
	if cacheMode == "" {
		cacheMode = cacheModeAuto
	}

	switch cacheMode {
	case cacheModeAlways:
		return true
	case cacheModeNever:
		return false
	case cacheModeAuto:
		return resolver.IsImmutable(params)
	default:
		return resolver.IsImmutable(params)
	}
}

// Validate returns an error if the cache mode is not "always", "never",
// "auto", or empty string (which defaults to auto).
func Validate(cacheMode string) error {
	// Empty string is valid - it will default to auto mode in ShouldUse
	if cacheMode == "" {
		return nil
	}

	validCacheModes := []string{cacheModeAlways, cacheModeNever, cacheModeAuto}
	if slices.Contains(validCacheModes, cacheMode) {
		return nil
	}

	return fmt.Errorf("invalid cache mode '%s', must be one of: %v (or empty for default)", cacheMode, validCacheModes)
}

type resolveFn = func() (resolutionframework.ResolvedResource, error)

func GetFromCacheOrResolve(
	ctx context.Context,
	params []v1.Param,
	resolverType string,
	resolve resolveFn,
) (resolutionframework.ResolvedResource, error) {
	cacheInstance := Get(ctx)

	if cached, ok := cacheInstance.Get(resolverType, params); ok {
		return cached, nil
	}

	key := generateCacheKey(resolverType, params)
	result, err, _ := cacheInstance.flightGroup.Do(key, func() (interface{}, error) {
		// If cache miss, resolve from params
		resource, err := resolve()
		if err != nil {
			return nil, err
		}

		// Store annotated resource with store operation and return annotated resource
		// to indicate it was stored in cache
		return cacheInstance.Add(resolverType, params, resource), nil
	})
	if err != nil {
		return nil, err
	}

	return result.(resolutionframework.ResolvedResource), nil
}
