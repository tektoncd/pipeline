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

package cluster

import (
	"context"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache/injection"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	clusterresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/cluster"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	// LabelValueClusterResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueClusterResolverType string = "cluster"

	// ClusterResolverName is the name that the cluster resolver should be
	// associated with
	ClusterResolverName string = "Cluster"

	// Legacy cache constants for backward compatibility with tests
	CacheModeAlways = framework.CacheModeAlways
	CacheModeNever  = framework.CacheModeNever
	CacheModeAuto   = framework.CacheModeAuto
	CacheParam      = framework.CacheParam
)

// Resolver implements a framework.Resolver that can fetch resources from the same cluster.
type Resolver struct {
	pipelineClientSet versioned.Interface
}

// Ensure Resolver implements CacheAwareResolver
var _ framework.CacheAwareResolver = (*Resolver)(nil)

// Initialize sets up any dependencies needed by the Resolver. None atm.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.pipelineClientSet = pipelineclient.Get(ctx)
	return nil
}

// GetName returns a string name to refer to this Resolver by.
func (r *Resolver) GetName(ctx context.Context) string {
	return ClusterResolverName
}

// GetSelector returns a map of labels to match against tasks requesting
// resolution from this Resolver.
func (r *Resolver) GetSelector(ctx context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: LabelValueClusterResolverType,
	}
}

// Validate ensures parameters from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	return clusterresolution.ValidateParams(ctx, req.Params)
}

// IsImmutable implements CacheAwareResolver.IsImmutable
// Returns false because cluster resources don't have immutable references
func (r *Resolver) IsImmutable(ctx context.Context, req *v1beta1.ResolutionRequestSpec) bool {
	// Cluster resources (Tasks, Pipelines, etc.) don't have immutable references
	// like Git commit hashes or bundle digests, so we always return false
	return false
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	// Determine if we should use caching using framework logic
	systemDefault := framework.GetSystemDefaultCacheMode("cluster")
	useCache := framework.ShouldUseCache(ctx, r, req, systemDefault)

	if useCache {
		// Get cache instance
		cacheInstance := injection.Get(ctx)
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
	resource, err := clusterresolution.ResolveFromParams(ctx, req.Params, r.pipelineClientSet)
	if err != nil {
		return nil, err
	}

	// Cache the result if caching is enabled
	if useCache {
		cacheInstance := injection.Get(ctx)
		cacheKey, err := cache.GenerateCacheKey(LabelValueClusterResolverType, req.Params)
		if err == nil {
			// Store annotated resource with store operation
			annotatedResource := cache.NewAnnotatedResource(resource, LabelValueClusterResolverType, cache.CacheOperationStore)
			cacheInstance.Add(cacheKey, annotatedResource)
			// Return annotated resource to indicate it was stored in cache
			return annotatedResource, nil
		}
	}

	return resource, nil
}

var _ resolutionframework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the cluster resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return "cluster-resolver-config"
}

// ShouldUseCache is a legacy function for backward compatibility with existing tests.
// It converts the old-style params map to the new framework API.
func ShouldUseCache(ctx context.Context, params map[string]string, checksum []byte) bool {
	// Convert params map to ResolutionRequestSpec
	var reqParams []pipelinev1.Param
	for key, value := range params {
		reqParams = append(reqParams, pipelinev1.Param{
			Name:  key,
			Value: pipelinev1.ParamValue{StringVal: value},
		})
	}

	req := &v1beta1.ResolutionRequestSpec{
		Params: reqParams,
	}

	resolver := &Resolver{}
	systemDefault := framework.GetSystemDefaultCacheMode("cluster")
	return framework.ShouldUseCache(ctx, resolver, req, systemDefault)
}
