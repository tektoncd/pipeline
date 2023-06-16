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
	"time"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/client/injection/kube/client"
)

const (
	disabledError = "cannot handle resolution request, enable-bundles-resolver feature flag not true"

	// LabelValueBundleResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueBundleResolverType string = "bundles"

	// TODO(sbwsg): This should be exposed as a configurable option for
	// admins (e.g. via ConfigMap)
	timeoutDuration = time.Minute

	// BundleResolverName is the name that the bundle resolver should be associated with.
	BundleResolverName = "bundleresolver"
)

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
	return ConfigMapName
}

// GetSelector returns a map of labels to match requests to this Resolver.
func (r *Resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueBundleResolverType,
	}
}

// ValidateParams ensures parameters from a request are as expected.
func (r *Resolver) ValidateParams(ctx context.Context, params []pipelinev1.Param) error {
	if r.isDisabled(ctx) {
		return errors.New(disabledError)
	}
	if _, err := OptionsFromParams(ctx, params); err != nil {
		return err
	}
	return nil
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, params []pipelinev1.Param) (framework.ResolvedResource, error) {
	if r.isDisabled(ctx) {
		return nil, errors.New(disabledError)
	}
	opts, err := OptionsFromParams(ctx, params)
	if err != nil {
		return nil, err
	}
	namespace := common.RequestNamespace(ctx)
	kc, _ := k8schain.New(ctx, r.kubeClientSet, k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: opts.ServiceAccount,
	})
	ctx, cancelFn := context.WithTimeout(ctx, timeoutDuration)
	defer cancelFn()
	return GetEntry(ctx, kc, opts)
}

func (r *Resolver) isDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.EnableBundleResolver
}
