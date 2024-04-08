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

package resource

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
)

type BasicRequest struct {
	resolverPayload ResolverPayload
}

var _ Request = &BasicRequest{}

// NewRequest returns an instance of a BasicRequestV2 with the given resolverPayload.
func NewRequest(resolverPayload ResolverPayload) Request {
	return &BasicRequest{resolverPayload}
}

var _ Request = &BasicRequest{}

// Params are the map of parameters associated with this request
func (req *BasicRequest) ResolverPayload() ResolverPayload {
	return req.resolverPayload
}

// Requester is the interface implemented by a type that knows how to
// submit requests for remote resources.
type Requester interface {
	// Submit accepts the name of a resolver to submit a request to
	// along with the request itself.
	Submit(ctx context.Context, name ResolverName, req Request) (ResolvedResource, error)
}

// Request is implemented by any type that represents a single request
// for a remote resource. Implementing this interface gives the underlying
// type an opportunity to control properties such as whether the name of
// a request has particular properties, whether the request should be made
// to a specific namespace, and precisely which parameters should be included.
type Request interface {
	ResolverPayload() ResolverPayload
}

// ResolverPayload is the struct which holds the payload to create
// the Resolution Request CRD.
type ResolverPayload struct {
	Name           string
	Namespace      string
	ResolutionSpec *v1beta1.ResolutionRequestSpec
}

// ResolutionRequester is the interface implemented by a type that knows how to
// submit requests for remote resources.
type ResolutionRequester interface {
	// SubmitResolutionRequest accepts the name of a resolver to submit a request to
	// along with the request itself.
	SubmitResolutionRequest(ctx context.Context, name ResolverName, req RequestRemoteResource) (ResolvedResource, error)
}

// RequestRemoteResource is implemented by any type that represents a single request
// for a remote resource. Implementing this interface gives the underlying
// type an opportunity to control properties such as whether the name of
// a request has particular properties, whether the request should be made
// to a specific namespace, and precisely which parameters should be included.
type RequestRemoteResource interface {
	ResolverPayload() ResolverPayload
}
