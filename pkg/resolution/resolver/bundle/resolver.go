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

package bundle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	common "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/client/injection/kube/client"
)

const (
	disabledError = "cannot handle resolution request, enable-bundles-resolver feature flag not true"

	// LabelValueBundleResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueBundleResolverType string = "bundles"

	// BundleResolverName is the name that the bundle resolver should be associated with.
	BundleResolverName = "bundleresolver"

	// ConfigMapName is the bundle resolver's config map
	ConfigMapName = "bundleresolver-config"
)

var _ framework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the git resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return ConfigMapName
}

var _ framework.TimedResolution = &Resolver{}

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
//
// Deprecated: Use [github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/bundle.Resolver] instead.
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

// GetSelector returns a map of labels to match requests to this Resolver.
func (r *Resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueBundleResolverType,
	}
}

// ValidateParams ensures parameters from a request are as expected.
func (r *Resolver) ValidateParams(ctx context.Context, params []v1.Param) error {
	return ValidateParams(ctx, params)
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, params []v1.Param) (framework.ResolvedResource, error) {
	return ResolveRequest(ctx, r.kubeClientSet, &v1beta1.ResolutionRequestSpec{Params: params})
}

// Resolve uses the given params to resolve the requested file or resource.
func ResolveRequest(ctx context.Context, kubeClientSet kubernetes.Interface, req *v1beta1.ResolutionRequestSpec) (framework.ResolvedResource, error) {
	if isDisabled(ctx) {
		return nil, errors.New(disabledError)
	}
	opts, err := OptionsFromParams(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	var imagePullSecrets []string
	if opts.ImagePullSecret != "" {
		imagePullSecrets = append(imagePullSecrets, opts.ImagePullSecret)
	}
	namespace := common.RequestNamespace(ctx)
	kc, err := k8schain.New(ctx, kubeClientSet, k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: opts.ServiceAccount,
		ImagePullSecrets:   imagePullSecrets,
	})
	if err != nil {
		return nil, err
	}
	return GetEntry(ctx, kc, opts)
}

func ValidateParams(ctx context.Context, params []v1.Param) error {
	if isDisabled(ctx) {
		return errors.New(disabledError)
	}
	if _, err := OptionsFromParams(ctx, params); err != nil {
		return err
	}
	return nil
}

func isDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.EnableBundleResolver
}

// GetResolutionTimeout returns a time.Duration for the amount of time a
// single bundle fetch may take. This can be configured with the
// fetch-timeout field in the bundle-resolver-config ConfigMap.
func (r *Resolver) GetResolutionTimeout(ctx context.Context, defaultTimeout time.Duration, params map[string]string) (time.Duration, error) {
	conf := framework.GetResolverConfigFromContext(ctx)

	timeout := defaultTimeout
	if v, ok := conf[ConfigTimeoutKey]; ok {
		var err error
		timeout, err = time.ParseDuration(v)
		if err != nil {
			return time.Duration(0), fmt.Errorf("error parsing bundle timeout value %s: %w", v, err)
		}
	}

	return timeout, nil
}
