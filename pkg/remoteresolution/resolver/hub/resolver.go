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

package hub

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/hub"
)

const (
	// LabelValueHubResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueHubResolverType string = "hub"

	// ArtifactHubType is the value to use setting the type field to artifact
	ArtifactHubType string = "artifact"

	// TektonHubType is the value to use setting the type field to tekton
	TektonHubType string = "tekton"
)

var _ framework.Resolver = &Resolver{}

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
type Resolver struct {
	// TektonHubURL is the URL for hub resolver with type tekton
	TektonHubURL string
	// ArtifactHubURL is the URL for hub resolver with type artifact
	ArtifactHubURL string
}

// Initialize sets up any dependencies needed by the resolver. None atm.
func (r *Resolver) Initialize(context.Context) error {
	return nil
}

// GetName returns a string name to refer to this resolver by.
func (r *Resolver) GetName(context.Context) string {
	return "Hub"
}

// GetConfigName returns the name of the bundle resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return "hubresolver-config"
}

// GetSelector returns a map of labels to match requests to this resolver.
func (r *Resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueHubResolverType,
	}
}

// Validate ensures parameters from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	return hub.ValidateParams(ctx, req.Params, r.TektonHubURL)
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	return hub.Resolve(ctx, req.Params, r.TektonHubURL, r.ArtifactHubURL)
}
