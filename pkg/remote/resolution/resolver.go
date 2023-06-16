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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"github.com/tektoncd/pipeline/pkg/remote"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
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
	params          v1.Params
	targetName      string
	targetNamespace string
}

var _ remote.Resolver = &Resolver{}

// NewResolver returns an implementation of remote.Resolver capable
// of performing asynchronous remote resolution.
func NewResolver(requester remoteresource.Requester, owner kmeta.OwnerRefable, resolverName string, targetName string, targetNamespace string, params v1.Params) remote.Resolver {
	return &Resolver{
		requester:       requester,
		owner:           owner,
		resolverName:    resolverName,
		params:          params,
		targetName:      targetName,
		targetNamespace: targetNamespace,
	}
}

// Get implements remote.Resolver.
func (resolver *Resolver) Get(ctx context.Context, _, _ string) (runtime.Object, *v1.RefSource, error) {
	resolverName := remoteresource.ResolverName(resolver.resolverName)
	req, err := buildRequest(resolver.resolverName, resolver.owner, resolver.targetName, resolver.targetNamespace, resolver.params)
	if err != nil {
		return nil, nil, fmt.Errorf("error building request for remote resource: %w", err)
	}
	resolved, err := resolver.requester.Submit(ctx, resolverName, req)
	switch {
	case errors.Is(err, resolutioncommon.ErrRequestInProgress):
		return nil, nil, remote.ErrRequestInProgress
	case err != nil:
		return nil, nil, fmt.Errorf("error requesting remote resource: %w", err)
	case resolved == nil:
		return nil, nil, ErrNilResource
	default:
	}
	data, err := resolved.Data()
	if err != nil {
		return nil, nil, &DataAccessError{original: err}
	}
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(data, nil, nil)
	if err != nil {
		return nil, nil, &InvalidRuntimeObjectError{original: err}
	}
	return obj, resolved.RefSource(), nil
}

// List implements remote.Resolver but is unused for remote resolution.
func (resolver *Resolver) List(_ context.Context) ([]remote.ResolvedObject, error) {
	return nil, nil
}

func buildRequest(resolverName string, owner kmeta.OwnerRefable, name string, namespace string, params v1.Params) (*resolutionRequest, error) {
	if name == "" {
		name = owner.GetObjectMeta().GetName()
		namespace = owner.GetObjectMeta().GetNamespace()
	}
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
