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
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache/injection"
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
	if cluster.IsDisabled(ctx) {
		return nil, errors.New(cluster.DisabledError)
	}

	// Default to no caching
	useCache := false
	if len(req.Params) > 0 {
		paramsMap := make(map[string]string)
		for _, p := range req.Params {
			paramsMap[p.Name] = p.Value.StringVal
		}
		if paramsMap[CacheParam] == CacheModeAlways {
			useCache = true
		}
	}

	var cacheInstance *cache.ResolverCache
	if useCache {
		// Get cache from dependency injection instead of global singleton
		cacheInstance = injection.Get(ctx)

		// Generate cache key from request params
		cacheKey, err := cache.GenerateCacheKey(LabelValueClusterResolverType, req.Params)
		if err != nil {
			return nil, err
		}

		// Check cache first
		if cached, ok := cacheInstance.Get(cacheKey); ok {
			if resource, ok := cached.(resolutionframework.ResolvedResource); ok {
				return cache.NewAnnotatedResource(resource, LabelValueClusterResolverType, cache.CacheOperationRetrieve), nil
			}
		}
	}

	// If not caching or cache miss, resolve from params
	resource, err := cluster.ResolveFromParams(ctx, req.Params, r.pipelineClientSet)
	if err != nil {
		return nil, err
	}

	// Cache the result if caching is enabled
	if useCache {
		cacheKey, _ := cache.GenerateCacheKey(LabelValueClusterResolverType, req.Params)
		// Store annotated resource with store operation
		annotatedResource := cache.NewAnnotatedResource(resource, LabelValueClusterResolverType, cache.CacheOperationStore)
		cacheInstance.Add(cacheKey, annotatedResource)
		// Return annotated resource to indicate it was stored in cache
		return annotatedResource, nil
	}

	return resource, nil
}

var _ resolutionframework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the cluster resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return configMapName
}

// ShouldUseCache determines whether caching should be used based on the cache mode parameter.
// For cluster resolver, caching is only enabled when cache mode is "always".
func ShouldUseCache(params map[string]string, checksum []byte) bool {
	cacheMode, exists := params[CacheParam]
	if !exists {
		return false // No cache parameter means no caching
	}

	switch cacheMode {
	case CacheModeAlways:
		return true
	case CacheModeNever:
		return false
	case CacheModeAuto:
		return false // Cluster resolver doesn't cache in auto mode
	default:
		return false // Invalid cache mode means no caching
	}
}
