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

package http

import (
	"context"
	"errors"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/http"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

const (
	// LabelValueHttpResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueHttpResolverType = "http"
	disabledError              = "cannot handle resolution request, enable-http-resolver feature flag not true"
	httpResolverName           = "Http"
	configMapName              = "http-resolver-config"
	defaultHttpTimeoutValue    = "1m"
	defaultBasicAuthSecretKey  = "password" // default key in the HTTP password secret
)

var _ framework.Resolver = (*Resolver)(nil)
var _ resolutionframework.ConfigWatcher = (*Resolver)(nil)

// Resolver implements a framework.Resolver that can fetch files from an HTTP URL
type Resolver struct {
	kubeClient kubernetes.Interface
	logger     *zap.SugaredLogger
}

func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClient = kubeclient.Get(ctx)
	r.logger = logging.FromContext(ctx)
	return nil
}

// GetName returns a string name to refer to this resolver by.
func (r *Resolver) GetName(_ context.Context) string {
	return httpResolverName
}

// GetConfigName returns the name of the http resolver's configmap.
func (r *Resolver) GetConfigName(_ context.Context) string {
	return configMapName
}

// GetSelector returns a map of labels to match requests to this resolver.
func (r *Resolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueHttpResolverType,
	}
}

// Validate ensures parameters from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	return http.ValidateParams(ctx, req.Params)
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	if http.IsDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	params, err := http.PopulateDefaultParams(ctx, req.Params)
	if err != nil {
		return nil, err
	}

	return http.FetchHttpResource(ctx, params, r.kubeClient, r.logger)
}
