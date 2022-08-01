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
	"time"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/client/injection/kube/client"
)

// LabelValueBundleResolverType is the value to use for the
// resolution.tekton.dev/type label on resource requests
const LabelValueBundleResolverType string = "bundles"

// TODO(sbwsg): This should be exposed as a configurable option for
// admins (e.g. via ConfigMap)
const timeoutDuration = time.Minute

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
	return "bundleresolver"
}

// GetConfigName returns the name of the bundle resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return "bundleresolver-config"
}

// GetSelector returns a map of labels to match requests to this Resolver.
func (r *Resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueBundleResolverType,
	}
}

// ValidateParams ensures parameters from a request are as expected.
func (r *Resolver) ValidateParams(ctx context.Context, params map[string]v1beta1.ArrayOrString) error {
	if _, err := OptionsFromParams(ctx, params); err != nil {
		return err
	}
	return nil
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, params map[string]v1beta1.ArrayOrString) (framework.ResolvedResource, error) {
	opts, err := OptionsFromParams(ctx, params)
	if err != nil {
		return nil, err
	}
	namespace := common.RequestNamespace(ctx)
	kc, err := k8schain.New(ctx, r.kubeClientSet, k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: opts.ServiceAccount,
	})
	ctx, cancelFn := context.WithTimeout(ctx, timeoutDuration)
	defer cancelFn()
	return GetEntry(ctx, kc, opts)
}
