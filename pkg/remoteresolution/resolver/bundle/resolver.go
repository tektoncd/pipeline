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

package bundle

import (
	"context"
	"errors"
	"strings"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache/injection"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	bundleresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"knative.dev/pkg/logging"
)

const (
	disabledError = "cannot handle resolution request, enable-bundles-resolver feature flag not true"

	// LabelValueBundleResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueBundleResolverType string = "bundles"
)

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
type Resolver struct{}

// Ensure Resolver implements CacheAwareResolver
var _ framework.CacheAwareResolver = (*Resolver)(nil)

// Initialize sets up any dependencies needed by the Resolver. None atm.
func (r *Resolver) Initialize(ctx context.Context) error {
	return nil
}

// GetName returns a string name to refer to this Resolver by.
func (r *Resolver) GetName(ctx context.Context) string {
	return "Bundles"
}

// GetSelector returns a map of labels to match against tasks requesting
// resolution from this Resolver.
func (r *Resolver) GetSelector(ctx context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: LabelValueBundleResolverType,
	}
}

// Validate ensures parameters from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if len(req.Params) == 0 {
		return errors.New("no params")
	}

	return bundleresolution.ValidateParams(ctx, req.Params)
}

// IsImmutable implements CacheAwareResolver.IsImmutable
// Returns true if the bundle parameter contains a digest reference (@sha256:...)
func (r *Resolver) IsImmutable(ctx context.Context, req *v1beta1.ResolutionRequestSpec) bool {
	var bundleRef string
	for _, param := range req.Params {
		if param.Name == bundleresolution.ParamBundle {
			bundleRef = param.Value.StringVal
			break
		}
	}

	return IsOCIPullSpecByDigest(bundleRef)
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	// Guard pattern: early return if no params
	if len(req.Params) == 0 {
		return nil, errors.New("no params")
	}

	logger := logging.FromContext(ctx)

	// Determine if we should use caching using framework logic
	systemDefault := framework.GetSystemDefaultCacheMode("bundle")
	useCache := framework.ShouldUseCache(ctx, r, req, systemDefault)

	if useCache {
		// Get cache instance
		cacheInstance := injection.Get(ctx)
		cacheKey, err := cache.GenerateCacheKey(LabelValueBundleResolverType, req.Params)
		if err != nil {
			logger.Warnf("Failed to generate cache key: %v", err)
		} else {
			// Check cache first
			if cached, ok := cacheInstance.Get(cacheKey); ok {
				if resource, ok := cached.(resolutionframework.ResolvedResource); ok {
					return cache.NewAnnotatedResource(resource, LabelValueBundleResolverType, cache.CacheOperationRetrieve), nil
				}
			}
		}
	}

	// If not caching or cache miss, resolve from params
	resource, err := bundleresolution.ResolveRequest(ctx, nil, req)
	if err != nil {
		return nil, err
	}

	// Cache the result if caching is enabled
	if useCache {
		cacheInstance := injection.Get(ctx)
		cacheKey, err := cache.GenerateCacheKey(LabelValueBundleResolverType, req.Params)
		if err == nil {
			// Store annotated resource with store operation
			annotatedResource := cache.NewAnnotatedResource(resource, LabelValueBundleResolverType, cache.CacheOperationStore)
			cacheInstance.Add(cacheKey, annotatedResource)
			// Return annotated resource to indicate it was stored in cache
			return annotatedResource, nil
		}
	}

	return resource, nil
}

// IsOCIPullSpecByDigest checks if the given string looks like an OCI pull spec by digest.
// A digest is typically in the format of @sha256:<hash> or :<tag>@sha256:<hash>
func IsOCIPullSpecByDigest(pullSpec string) bool {
	// Check for @sha256: pattern
	if strings.Contains(pullSpec, "@sha256:") {
		return true
	}
	// Check for :<tag>@sha256: pattern
	if strings.Contains(pullSpec, ":") && strings.Contains(pullSpec, "@sha256:") {
		return true
	}
	return false
}

// Legacy functions - kept for backward compatibility and tests
// TODO: Remove these once all tests are updated

// ShouldUseCache is kept for backward compatibility with existing tests
// New code should use framework.ShouldUseCache instead
func ShouldUseCache(ctx context.Context, opts bundleresolution.RequestOptions) bool {
	// Convert RequestOptions to ResolutionRequestSpec for framework compatibility
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: bundleresolution.ParamBundle, Value: pipelinev1.ParamValue{StringVal: opts.Bundle}},
			{Name: bundleresolution.ParamName, Value: pipelinev1.ParamValue{StringVal: opts.EntryName}},
			{Name: bundleresolution.ParamKind, Value: pipelinev1.ParamValue{StringVal: opts.Kind}},
		},
	}

	// Add cache parameter if provided
	if opts.Cache != "" {
		req.Params = append(req.Params, pipelinev1.Param{
			Name:  framework.CacheParam,
			Value: pipelinev1.ParamValue{StringVal: opts.Cache},
		})
	}

	r := &Resolver{}
	systemDefault := framework.GetSystemDefaultCacheMode("bundle")
	return framework.ShouldUseCache(ctx, r, req, systemDefault)
}
