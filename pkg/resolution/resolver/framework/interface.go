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
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// Resolver is the interface to implement for type-specific resource
// resolution. It fetches resources from a given type of remote location
// and returns their content along with any associated annotations.
type Resolver interface {
	// Initialize is called at the moment the resolver controller is
	// instantiated and is a good place to setup things like
	// resource listers.
	Initialize(context.Context) error

	// GetName should give back the name of the resolver. E.g. "Git"
	GetName(context.Context) string

	// GetSelector returns the labels that are used to direct resolution
	// requests to this resolver.
	GetSelector(context.Context) map[string]string

	// ValidateParams is given the parameters from a resource
	// request and should return an error if any are missing or invalid.
	ValidateParams(context.Context, []pipelinev1.Param) error

	// Resolve receives the parameters passed via a resource request
	// and returns the resolved data along with any annotations
	// to include in the response. If resolution fails then an error
	// should be returned instead. If a resolution.Error
	// is returned then its Reason and Message are used as part of the
	// response to the request.
	Resolve(context.Context, []pipelinev1.Param) (ResolvedResource, error)
}

// ConfigWatcher is the interface to implement if your resolver accepts
// additional configuration from an admin. Examples of how this
// might be used:
// - your resolver might require an allow-list of repositories or registries
// - your resolver might allow request timeout settings to be configured
// - your resolver might need an API endpoint or base url to be set
//
// When your resolver implements this interface it will be able to
// access configuration from the context it receives in calls to
// ValidateParams and Resolve.
type ConfigWatcher interface {
	// GetConfigName should return a string name for its
	// configuration to be referenced by. This will map to the name
	// of a ConfigMap in the same namespace as the resolver.
	GetConfigName(context.Context) string
}

// TimedResolution is an optional interface that a resolver can
// implement to override the default resolution request timeout.
//
// There are two timeouts that a resolution request adheres to: First
// there is a global timeout that the core ResolutionRequest reconciler
// enforces on _all_ requests. This prevents zombie requests (such as
// those with a misconfigured `type`) sticking around in perpetuity.
// Second there are resolver-specific timeouts that default to 1 minute.
//
// A resolver implemeting the TimedResolution interface sets the maximum
// duration of any single request to this resolver.
//
// The core ResolutionRequest reconciler's global timeout overrides any
// resolver-specific timeout.
type TimedResolution interface {
	// GetResolutionTimeout receives the current request's context
	// object, which includes any request-scoped data like
	// resolver config and the request's originating namespace,
	// along with a default.
	GetResolutionTimeout(context.Context, time.Duration) time.Duration
}

// ResolvedResource returns the data and annotations of a successful
// resource fetch.
type ResolvedResource interface {
	Data() []byte
	Annotations() map[string]string
	RefSource() *pipelinev1.RefSource
}
