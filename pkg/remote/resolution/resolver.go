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
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	tknreconciler "github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/remote"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"knative.dev/pkg/kmap"
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
	decoder := serializer.
		NewCodecFactory(scheme.Scheme, serializer.EnableStrict).
		UniversalDeserializer()
	return ResolvedRequest(resolved, decoder, err)
}

// List implements remote.Resolver but is unused for remote resolution.
func (resolver *Resolver) List(_ context.Context) ([]remote.ResolvedObject, error) {
	return nil, nil
}

func buildRequest(resolverName string, owner kmeta.OwnerRefable, name string, namespace string, params v1.Params) (*resolutionRequest, error) {
	rr := &v1beta1.ResolutionRequestSpec{
		Params: params,
	}
	name, namespace, err := remoteresource.GetNameAndNamespace(resolverName, owner, name, namespace, rr)
	if err != nil {
		return nil, err
	}
	req := &resolutionRequest{
		Request: remoteresource.NewRequest(name, namespace, params),
		owner:   owner,
	}
	return req, nil
}

func ResolvedRequest(resolved resolutioncommon.ResolvedResource, decoder runtime.Decoder, err error) (runtime.Object, *v1.RefSource, error) {
	if errors.Is(err, resolutioncommon.ErrRequestInProgress) {
		return nil, nil, remote.ErrRequestInProgress
	}

	if err != nil {
		return nil, nil, fmt.Errorf("error requesting remote resource: %w", err)
	}

	if resolved == nil {
		return nil, nil, ErrNilResource
	}

	data, err := resolved.Data()
	if err != nil {
		return nil, nil, &DataAccessError{Original: err}
	}

	obj, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, nil, &InvalidRuntimeObjectError{Original: err}
	}

	if len(resolved.Annotations()) == 0 {
		return obj, resolved.RefSource(), nil
	}

	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return nil, nil, &UnsupportedObjectTypeError{ObjectType: fmt.Sprintf("%T", obj)}
	}

	mergedAnnotations := kmap.Union(kmap.ExcludeKeys(resolved.Annotations(), tknreconciler.KubectlLastAppliedAnnotationKey), metaObj.GetAnnotations())
	metaObj.SetAnnotations(mergedAnnotations)

	return obj, resolved.RefSource(), nil
}
