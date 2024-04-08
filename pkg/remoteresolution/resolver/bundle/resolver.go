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

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/client/injection/kube/client"
)

const (
	// LabelValueBundleResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueBundleResolverType string = "bundles"

	// BundleResolverName is the name that the bundle resolver should be associated with.
	BundleResolverName = "bundleresolver"
)

var _ framework.Resolver = &Resolver{}

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
type Resolver struct {
	kubeClientSet kubernetes.Interface
}

// Initialize sets up any dependencies needed by the Resolver. None atm.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClientSet = client.Get(ctx)
	return nil
}

// GetName returns a string name to refer to this Resolver by.
func (r *Resolver) GetName(context.Context) string {
	return BundleResolverName
}

// GetConfigName returns the name of the bundle resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return bundle.ConfigMapName
}

// GetSelector returns a map of labels to match requests to this Resolver.
func (r *Resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueBundleResolverType,
	}
}

// Validate ensures reqolution request spec from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if len(req.Params) > 0 {
		return bundle.ValidateParams(ctx, req.Params)
	}
	// Remove this error once validate url has been implemented.
	return errors.New("cannot validate request. the Validate method has not been implemented.")
}

// Resolve uses the given request spec resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	if len(req.Params) > 0 {
		return bundle.ResolveRequest(ctx, r.kubeClientSet, req)
	}
	// Remove this error once resolution of url has been implemented.
	return nil, errors.New("the Resolve method has not been implemented.")
}
