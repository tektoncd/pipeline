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

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"github.com/tektoncd/pipeline/pkg/remote"
	resolutioncommon "github.com/tektoncd/resolution/pkg/common"
	remoteresource "github.com/tektoncd/resolution/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/kmeta"
)

// Resolver implements remote.Resolver and encapsulates the majority of
// code required to interface with the tektoncd/resolution project. It
// is used to make async requests for resources like pipelines from
// remote places like git repos.
type Resolver struct {
	requester    remoteresource.Requester
	owner        kmeta.OwnerRefable
	resolverName string
	params       map[string]string
}

var _ remote.Resolver = &Resolver{}

// NewResolver returns an implementation of remote.Resolver capable
// of performing asynchronous remote resolution.
func NewResolver(requester remoteresource.Requester, owner kmeta.OwnerRefable, resolverName string, params map[string]string) remote.Resolver {
	return &Resolver{
		requester:    requester,
		owner:        owner,
		resolverName: resolverName,
		params:       params,
	}
}

// Get implements remote.Resolver.
func (resolver *Resolver) Get(ctx context.Context, _, _ string) (runtime.Object, error) {
	resolverName := remoteresource.ResolverName(resolver.resolverName)
	req, err := buildRequest(resolver.resolverName, resolver.owner, resolver.params)
	if err != nil {
		return nil, fmt.Errorf("error building request for remote resource: %w", err)
	}
	resolved, err := resolver.requester.Submit(ctx, resolverName, req)
	switch {
	case errors.Is(err, resolutioncommon.ErrorRequestInProgress):
		return nil, remote.ErrorRequestInProgress
	case err != nil:
		return nil, fmt.Errorf("error requesting remote resource: %w", err)
	case resolved == nil:
		return nil, ErrorRequestedResourceIsNil
	default:
	}
	data, err := resolved.Data()
	if err != nil {
		return nil, &ErrorAccessingData{original: err}
	}
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(data, nil, nil)
	if err != nil {
		return nil, &ErrorInvalidRuntimeObject{original: err}
	}
	return obj, nil
}

// List implements remote.Resolver but is unused for remote resolution.
func (resolver *Resolver) List(_ context.Context) ([]remote.ResolvedObject, error) {
	return nil, nil
}

func buildRequest(resolverName string, owner kmeta.OwnerRefable, params map[string]string) (*resolutionRequest, error) {
	name := owner.GetObjectMeta().GetName()
	namespace := owner.GetObjectMeta().GetNamespace()
	if namespace == "" {
		namespace = "default"
	}
	// Generating a deterministic name for the resource request
	// prevents multiple requests being issued for the same
	// pipelinerun's pipelineRef or taskrun's taskRef.
	remoteResourceBaseName := namespace + "/" + name
	name, err := remoteresource.GenerateDeterministicName(resolverName, remoteResourceBaseName, params)
	if err != nil {
		return nil, fmt.Errorf("error generating name for taskrun %s/%s: %w", namespace, name, err)
	}
	req := &resolutionRequest{
		Request: remoteresource.NewRequest(name, namespace, params),
		owner:   owner,
	}
	return req, nil
}
