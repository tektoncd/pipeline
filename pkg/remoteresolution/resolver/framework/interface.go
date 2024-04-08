/*
Copyright 2022 The Tekton Authors

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

package framework

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

// Resolver is the interface to implement for type-specific resource
// resolution. It fetches resources from a given type of remote location
// and returns their content along with any associated annotations.
type Resolver interface {
	// Initialize is called at the moment the resolver controller is
	// instantiated and is a good place to setup things like
	// resource listers.
	Initialize(ctx context.Context) error

	// GetName should give back the name of the resolver. E.g. "Git"
	GetName(ctx context.Context) string

	// GetSelector returns the labels that are used to direct resolution
	// requests to this resolver.
	GetSelector(ctx context.Context) map[string]string

	// Validate is given the ressolution request spec
	// should return an error if the resolver cannot resolve it.
	Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error

	// ResolveRequest receives the resolution request spec
	// and returns the resolved data along with any annotations
	// to include in the response. If resolution fails then an error
	// should be returned instead. If a resolution.Error
	// is returned then its Reason and Message are used as part of the
	// response to the request.
	Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (framework.ResolvedResource, error)
}
