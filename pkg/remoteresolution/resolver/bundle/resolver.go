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

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	bundleresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

const (
	// LabelValueBundleResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueBundleResolverType string = "bundles"

	// BundleResolverName is the name that the bundle resolver should be associated with.
	BundleResolverName string = "Bundles"
)

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
type Resolver struct {
	kubeClientSet      kubernetes.Interface
	resolveRequestFunc func(context.Context, kubernetes.Interface, *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error)
}

// Ensure Resolver implements ImmutabilityChecker
var _ framework.ImmutabilityChecker = (*Resolver)(nil)

// Ensure Resolver implements ConfigWatcher
var _ resolutionframework.ConfigWatcher = (*Resolver)(nil)

// Initialize sets up any dependencies needed by the Resolver. None atm.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClientSet = kubeclient.Get(ctx)
	if r.resolveRequestFunc == nil {
		r.resolveRequestFunc = bundleresolution.ResolveRequest
	}
	return nil
}

// GetName returns a string name to refer to this Resolver by.
func (r *Resolver) GetName(ctx context.Context) string {
	return BundleResolverName
}

// GetConfigName returns the name of the bundle resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return bundleresolution.ConfigMapName
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
	return bundleresolution.ValidateParams(ctx, req.Params)
}

// IsImmutable implements ImmutabilityChecker.IsImmutable
// Returns true if the bundle parameter contains a digest reference (@sha256:...)
func (r *Resolver) IsImmutable(ctx context.Context, params []pipelinev1.Param) bool {
	var bundleRef string
	for _, param := range params {
		if param.Name == bundleresolution.ParamBundle {
			bundleRef = param.Value.StringVal
			break
		}
	}

	return bundleresolution.IsOCIPullSpecByDigest(bundleRef)
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	// Guard: validate request has parameters
	if len(req.Params) == 0 {
		return nil, errors.New("no params")
	}

	// Ensure resolve function is set
	if r.resolveRequestFunc == nil {
		r.resolveRequestFunc = bundleresolution.ResolveRequest
	}

	// Use framework cache operations if caching is enabled
	if !framework.ShouldUseCache(ctx, r, req.Params, LabelValueBundleResolverType) {
		return r.resolveRequestFunc(ctx, r.kubeClientSet, req)
	}

	return framework.RunCacheOperations(
		ctx,
		req.Params,
		LabelValueBundleResolverType,
		func() (resolutionframework.ResolvedResource, error) {
			return r.resolveRequestFunc(ctx, r.kubeClientSet, req)
		},
	)
}
