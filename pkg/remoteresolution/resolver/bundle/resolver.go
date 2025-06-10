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

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/client/injection/kube/client"
)

const (
	// LabelValueBundleResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueBundleResolverType string = "bundles"

	// BundleResolverName is the name that the bundle resolver should be associated with.
	BundleResolverName = "bundleresolver"

	// ConfigMapName is the bundle resolver's config map
	ConfigMapName = "bundleresolver-config"

	// CacheModeAlways means always use cache regardless of bundle reference
	CacheModeAlways = "always"
	// CacheModeNever means never use cache regardless of bundle reference
	CacheModeNever = "never"
	// CacheModeAuto means use cache only when bundle reference has a digest
	CacheModeAuto = "auto"

	// CacheParam is the key for the cache mode in the params map
	CacheParam = "cache"
	// BundleParam is the key for the bundle reference in the params map
	BundleParam = "bundle"
)

var _ framework.Resolver = &Resolver{}

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
type Resolver struct {
	kubeClientSet kubernetes.Interface
}

// Initialize sets up any dependencies needed by the Resolver. None atm.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClientSet = client.Get(ctx)
	return nil
}

// GetName returns a string name to refer to this Resolver by.
func (r *Resolver) GetName(context.Context) string {
	return BundleResolverName
}

// GetConfigName returns the name of the bundle resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return ConfigMapName
}

// GetSelector returns the labels that resource requests are required to have for
// the bundle resolver to process them.
func (r *Resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueBundleResolverType,
	}
}

// Validate ensures parameters from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if len(req.Params) > 0 {
		return bundle.ValidateParams(ctx, req.Params)
	}
	// Remove this error once validate url has been implemented.
	return errors.New("cannot validate request. the Validate method has not been implemented.")
}

// ShouldUseCache determines if caching should be used based on the cache mode and bundle reference.
func ShouldUseCache(opts bundle.RequestOptions) bool {
	cacheMode := opts.Cache
	bundleRef := opts.Bundle
	switch cacheMode {
	case CacheModeAlways:
		return true
	case CacheModeNever:
		return false
	case CacheModeAuto, "": // default to auto if not specified
		return IsOCIPullSpecByDigest(bundleRef)
	default: // invalid cache mode defaults to auto
		return IsOCIPullSpecByDigest(bundleRef)
	}
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	if len(req.Params) > 0 {
		if bundle.IsDisabled(ctx) {
			return nil, errors.New(bundle.DisabledError)
		}

		opts, err := bundle.OptionsFromParams(ctx, req.Params)
		if err != nil {
			return nil, err
		}

		// Determine if caching should be used based on cache mode
		useCache := ShouldUseCache(opts)

		if useCache {
			// Initialize cache logger
			cache.GetGlobalCache().InitializeLogger(ctx)

			// Generate cache key
			cacheKey, err := cache.GenerateCacheKey(LabelValueBundleResolverType, req.Params)
			if err != nil {
				return nil, err
			}

			// Check cache first
			if cached, ok := cache.GetGlobalCache().Get(cacheKey); ok {
				if resource, ok := cached.(resolutionframework.ResolvedResource); ok {
					return resource, nil
				}
			}
		}

		resource, err := bundle.ResolveRequest(ctx, r.kubeClientSet, req)
		if err != nil {
			return nil, err
		}

		// Cache the result if caching is enabled
		if useCache {
			cacheKey, _ := cache.GenerateCacheKey(LabelValueBundleResolverType, req.Params)
			cache.GetGlobalCache().Add(cacheKey, resource)
		}

		return resource, nil
	}
	// Remove this error once resolution of url has been implemented.
	return nil, errors.New("the Resolve method has not been implemented.")
}

// IsOCIPullSpecByDigest checks if the given OCI pull spec contains a digest.
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
