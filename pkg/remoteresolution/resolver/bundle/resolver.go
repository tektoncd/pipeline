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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/cache"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	bundleresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

const (
	// LabelValueBundleResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueBundleResolverType = "bundles"

	// BundleResolverName is the name that the bundle resolver should be associated with.
	BundleResolverName = "bundleresolver"
)

var _ framework.Resolver = (*Resolver)(nil)
var _ resolutionframework.ConfigWatcher = (*Resolver)(nil)
var _ cache.ImmutabilityChecker = (*Resolver)(nil)

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
type Resolver struct {
	kubeClientSet      kubernetes.Interface
	resolveRequestFunc func(context.Context, kubernetes.Interface, *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error)
}

// Initialize sets up any dependencies needed by the Resolver. None atm.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClientSet = kubeclient.Get(ctx)
	if r.resolveRequestFunc == nil {
		r.resolveRequestFunc = bundleresolution.ResolveRequest
	}
	return nil
}

// GetName returns a string name to refer to this Resolver by.
func (r *Resolver) GetName(_ context.Context) string {
	return BundleResolverName
}

// GetSelector returns a map of labels to match against tasks requesting
// resolution from this Resolver.
func (r *Resolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: LabelValueBundleResolverType,
	}
}

// GetConfigName returns the name of the bundle resolver's configmap.
func (r *Resolver) GetConfigName(_ context.Context) string {
	return bundleresolution.ConfigMapName
}

// Validate ensures parameters from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	return bundleresolution.ValidateParams(ctx, req.Params)
}

// IsImmutable implements ImmutabilityChecker.IsImmutable
// Returns true if the bundle parameter contains a digest reference (@sha256:...)
func (r *Resolver) IsImmutable(params []v1.Param) bool {
	var bundleRef string
	for _, param := range params {
		if param.Name == bundleresolution.ParamBundle {
			bundleRef = param.Value.StringVal
			break
		}
	}

	// Checks if the given string looks like an OCI pull spec by digest.
	// A digest is typically in the format of @sha256:<hash> or :<tag>@sha256:<hash>
	// Check for @sha256: pattern
	if strings.Contains(bundleRef, "@sha256:") {
		return true
	}
	// TODO(twoGiants): doesn't look right => fix this & write test
	// Check for :<tag>@sha256: pattern
	if strings.Contains(bundleRef, ":") && strings.Contains(bundleRef, "@sha256:") {
		return true
	}

	return false
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	if len(req.Params) == 0 {
		return nil, errors.New("no params")
	}

	// Verify resolver was initialized - kubeClientSet must be set by Initialize()
	if r.kubeClientSet == nil {
		return nil, errors.New("bundle resolver not properly initialized: Initialize() must be called before Resolve()")
	}

	// Defensive: set default resolveRequestFunc if somehow nil
	resolveFunc := r.resolveRequestFunc
	if resolveFunc == nil {
		resolveFunc = bundleresolution.ResolveRequest
	}

	if cache.ShouldUse(ctx, r, req.Params, LabelValueBundleResolverType) {
		return cache.GetFromCacheOrResolve(
			ctx,
			req.Params,
			LabelValueBundleResolverType,
			func() (resolutionframework.ResolvedResource, error) {
				return resolveFunc(ctx, r.kubeClientSet, req)
			},
		)
	}

	return resolveFunc(ctx, r.kubeClientSet, req)
}
