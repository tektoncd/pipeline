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

package resolution

import (
	"context"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/remote"
	resolution "github.com/tektoncd/pipeline/pkg/remote/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	resource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/kmeta"
)

// Resolver implements remote.Resolver and encapsulates the majority of
// code required to interface with the tektoncd/resolution project. It
// is used to make async requests for resources like pipelines from
// remote places like git repos.
type Resolver struct {
	requester       remoteresource.Requester
	owner           kmeta.OwnerRefable
	resolverName    string
	resolverPayload remoteresource.ResolverPayload
}

var _ remote.Resolver = &Resolver{}

// NewResolver returns an implementation of remote.Resolver capable
// of performing asynchronous remote resolution.
func NewResolver(requester remoteresource.Requester, owner kmeta.OwnerRefable, resolverName string, resolverPayload remoteresource.ResolverPayload) remote.Resolver {
	return &Resolver{
		requester:       requester,
		owner:           owner,
		resolverName:    resolverName,
		resolverPayload: resolverPayload,
	}
}

// Get implements remote.Resolver.
func (resolver *Resolver) Get(ctx context.Context, _, _ string) (runtime.Object, *v1.RefSource, error) {
	resolverName := remoteresource.ResolverName(resolver.resolverName)
	req, err := buildRequest(resolver.resolverName, resolver.owner, &resolver.resolverPayload)
	if err != nil {
		return nil, nil, fmt.Errorf("error building request for remote resource: %w", err)
	}
	resolved, err := resolver.requester.Submit(ctx, resolverName, req)
	return resolution.ResolvedRequest(resolved, err)
}

// List implements remote.Resolver but is unused for remote resolution.
func (resolver *Resolver) List(_ context.Context) ([]remote.ResolvedObject, error) {
	return nil, nil
}

func buildRequest(resolverName string, owner kmeta.OwnerRefable, resolverPayload *remoteresource.ResolverPayload) (*resolutionRequest, error) {
	var name string
	var namespace string
	var params v1.Params
	if resolverPayload != nil {
		name = resolverPayload.Name
		namespace = resolverPayload.Namespace
		if resolverPayload.ResolutionSpec != nil {
			params = resolverPayload.ResolutionSpec.Params
		}
	}
	name, namespace, err := resource.GetNameAndNamespace(resolverName, owner, name, namespace, params)
	if err != nil {
		return nil, err
	}
	resolverPayload.Name = name
	resolverPayload.Namespace = namespace
	req := &resolutionRequest{
		Request: remoteresource.NewRequest(*resolverPayload),
		owner:   owner,
	}
	return req, nil
}
