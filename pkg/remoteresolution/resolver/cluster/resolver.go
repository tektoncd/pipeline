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

package cluster

import (
	"context"
	"errors"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/cluster"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	// LabelValueClusterResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueClusterResolverType string = "cluster"

	// ClusterResolverName is the name that the cluster resolver should be
	// associated with
	ClusterResolverName string = "Cluster"

	configMapName = "cluster-resolver-config"

	// CacheModeAlways means always use cache regardless of resource checksum
	CacheModeAlways = "always"
	// CacheModeNever means never use cache regardless of resource checksum
	CacheModeNever = "never"
	// CacheModeAuto means use cache only when resource has a checksum
	CacheModeAuto = "auto"

	// CacheParam is the key for the cache mode in the params map
	CacheParam = "cache"
)

var _ framework.Resolver = &Resolver{}

// ResolverV2 implements a framework.Resolver that can fetch resources from other namespaces.
type Resolver struct {
	pipelineClientSet clientset.Interface
}

// Initialize performs any setup required by the cluster resolver.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.pipelineClientSet = pipelineclient.Get(ctx)
	return nil
}

// GetName returns the string name that the cluster resolver should be
// associated with.
func (r *Resolver) GetName(_ context.Context) string {
	return ClusterResolverName
}

// GetSelector returns the labels that resource requests are required to have for
// the cluster resolver to process them.
func (r *Resolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: LabelValueClusterResolverType,
	}
}

// Validate returns an error if the given parameter map is not
// valid for a resource request targeting the cluster resolver.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if len(req.Params) > 0 {
		return cluster.ValidateParams(ctx, req.Params)
	}
	// Remove this error once validate url has been implemented.
	return errors.New("cannot validate request. the Validate method has not been implemented")
}

// Resolve performs the work of fetching a resource from a namespace with the given
// resolution spec.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	// Guard pattern: early return if no params
	if len(req.Params) == 0 {
		// Remove this error once resolution of url has been implemented.
		return nil, errors.New("the Resolve method has not been implemented")
	}

	// Guard pattern: early return if disabled
	if cluster.IsDisabled(ctx) {
		return nil, errors.New(cluster.DisabledError)
	}

	// Check if caching should be enabled
	useCache := false
	if len(req.Params) > 0 {
		// Convert params to map for cache decision
		paramsMap := make(map[string]string)
		for _, p := range req.Params {
			paramsMap[p.Name] = p.Value.StringVal
		}

		// For auto mode, we need to resolve first to get the checksum
		// Then decide if we should cache based on the checksum
		if paramsMap[CacheParam] == CacheModeAuto || paramsMap[CacheParam] == "" {
			// Resolve first to get checksum
			resource, err := cluster.ResolveFromParams(ctx, req.Params, r.pipelineClientSet)
			if err != nil {
				return nil, err
			}

			// Get checksum from the resolved resource
			if clusterResource, ok := resource.(*cluster.ResolvedClusterResource); ok {
				useCache = ShouldUseCache(paramsMap, clusterResource.Checksum)
			}

			// If we should cache, check cache first
			if useCache {
				// Initialize cache logger
				cache.GetGlobalCache().InitializeLogger(ctx)

				// Generate cache key
				cacheKey, err := cache.GenerateCacheKey(LabelValueClusterResolverType, req.Params)
				if err != nil {
					return nil, err
				}

				// Check cache first
				if cached, ok := cache.GetGlobalCache().Get(cacheKey); ok {
					if cachedResource, ok := cached.(resolutionframework.ResolvedResource); ok {
						// Return annotated resource to indicate it came from cache
						return cache.NewAnnotatedResource(cachedResource, LabelValueClusterResolverType), nil
					}
				}

				// Cache the result
				cacheKey, _ = cache.GenerateCacheKey(LabelValueClusterResolverType, req.Params)
				cache.GetGlobalCache().Add(cacheKey, resource)
			}

			return resource, nil
		} else {
			// For always/never modes, decide upfront
			useCache = ShouldUseCache(paramsMap, nil) // checksum not needed for always/never
		}
	}

	// If caching is enabled, check cache first
	if useCache {
		// Initialize cache logger
		cache.GetGlobalCache().InitializeLogger(ctx)

		// Generate cache key
		cacheKey, err := cache.GenerateCacheKey(LabelValueClusterResolverType, req.Params)
		if err != nil {
			return nil, err
		}

		// Check cache first
		if cached, ok := cache.GetGlobalCache().Get(cacheKey); ok {
			if cachedResource, ok := cached.(resolutionframework.ResolvedResource); ok {
				// Return annotated resource to indicate it came from cache
				return cache.NewAnnotatedResource(cachedResource, LabelValueClusterResolverType), nil
			}
		}
	}

	// Resolve the resource
	resource, err := cluster.ResolveFromParams(ctx, req.Params, r.pipelineClientSet)
	if err != nil {
		return nil, err
	}

	// Cache the result if caching is enabled
	if useCache {
		cacheKey, _ := cache.GenerateCacheKey(LabelValueClusterResolverType, req.Params)
		cache.GetGlobalCache().Add(cacheKey, resource)
	}

	return resource, nil
}

var _ resolutionframework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the cluster resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return configMapName
}

// HasResourceChecksum checks if the given resource has a valid checksum.
// This is used to determine if caching should be enabled in auto mode.
func HasResourceChecksum(checksum []byte) bool {
	return len(checksum) > 0
}

// ShouldUseCache determines if caching should be used based on the cache mode and resource checksum.
func ShouldUseCache(params map[string]string, checksum []byte) bool {
	cacheMode := params[CacheParam]
	switch cacheMode {
	case CacheModeAlways:
		return true
	case CacheModeNever:
		return false
	case CacheModeAuto, "": // default to auto if not specified
		return HasResourceChecksum(checksum)
	default: // invalid cache mode defaults to auto
		return HasResourceChecksum(checksum)
	}
}
