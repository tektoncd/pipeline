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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/cache"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	clusterresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/cluster"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	// LabelValueClusterResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueClusterResolverType = "cluster"

	// ClusterResolverName is the name that the cluster resolver should be
	// associated with
	ClusterResolverName = "Cluster"
)

var _ framework.Resolver = (*Resolver)(nil)
var _ resolutionframework.ConfigWatcher = (*Resolver)(nil)
var _ cache.ImmutabilityChecker = (*Resolver)(nil)

// Resolver implements a framework.Resolver that can fetch resources from the same cluster.
type Resolver struct {
	pipelineClientSet versioned.Interface
}

// Initialize sets up any dependencies needed by the Resolver.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.pipelineClientSet = pipelineclient.Get(ctx)
	return nil
}

// GetName returns a string name to refer to this Resolver by.
func (r *Resolver) GetName(_ context.Context) string {
	return ClusterResolverName
}

// GetSelector returns a map of labels to match against tasks requesting
// resolution from this Resolver.
func (r *Resolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: LabelValueClusterResolverType,
	}
}

// GetConfigName returns the name of the cluster resolver's configmap.
func (r *Resolver) GetConfigName(_ context.Context) string {
	return clusterresolution.ConfigMapName
}

// Validate ensures parameters from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	return clusterresolution.ValidateParams(ctx, req.Params)
}

// IsImmutable implements ImmutabilityChecker.IsImmutable
// Returns false because cluster resources don't have immutable references
func (r *Resolver) IsImmutable([]v1.Param) bool {
	// Cluster resources (Tasks, Pipelines, etc.) don't have immutable references
	// like Git commit hashes or bundle digests, so we always return false
	return false
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	if cache.ShouldUse(ctx, r, req.Params) {
		return cache.Get(ctx).GetCachedOrResolveFromRemote(
			req.Params,
			LabelValueClusterResolverType,
			func() (resolutionframework.ResolvedResource, error) {
				return clusterresolution.ResolveFromParams(ctx, req.Params, r.pipelineClientSet)
			},
		)
	}

	return clusterresolution.ResolveFromParams(ctx, req.Params, r.pipelineClientSet)
}
