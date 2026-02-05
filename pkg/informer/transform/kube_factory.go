/*
Copyright 2026 The Tekton Authors

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

package transform

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	client "knative.dev/pkg/client/injection/kube/client"
	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

func init() {
	// Register our factory AFTER the default one to override it.
	// This works because Knative's injection framework allows multiple
	// registrations with the same key, and later ones override earlier ones.
	injection.Default.RegisterInformerFactory(withTransformFilteredFactory)
}

// withTransformFilteredFactory creates a new filtered informer factory with the
// Pod cache transform applied. This factory replaces the default Knative filtered
// factory by using the same context key.
//
// This approach ensures that all Kubernetes Pod informers used by Tekton apply
// the transform to reduce memory usage without modifying the generated code or
// the Knative pkg.
//
// The transform can be disabled by setting enable-informer-cache-transforms=false
// in the feature-flags ConfigMap or by setting the ENABLE_INFORMER_CACHE_TRANSFORMS
// environment variable to "false".
func withTransformFilteredFactory(ctx context.Context) context.Context {
	c := client.Get(ctx)

	// Get the label selectors from context (set by filteredinformerfactory.WithSelectors)
	untyped := ctx.Value(filteredFactory.LabelKey{})
	if untyped == nil {
		// No selectors configured, let the default factory handle it
		logging.FromContext(ctx).Debug("No label selectors configured for filtered factory, skipping transform")
		return ctx
	}

	// Check if transforms are enabled
	transformsEnabled := config.InformerCacheTransformsEnabled()
	if transformsEnabled {
		logging.FromContext(ctx).Info("Informer cache transforms enabled for Pod resources")
	} else {
		logging.FromContext(ctx).Info("Informer cache transforms disabled for Pod resources")
	}

	labelSelectors := untyped.([]string)
	for _, selector := range labelSelectors {
		selectorVal := selector
		opts := []informers.SharedInformerOption{}

		// Handle namespace scoping
		if injection.HasNamespaceScope(ctx) {
			opts = append(opts, informers.WithNamespace(injection.GetNamespaceScope(ctx)))
		}

		// Add label selector filter
		opts = append(opts, informers.WithTweakListOptions(func(l *metav1.ListOptions) {
			l.LabelSelector = selectorVal
		}))

		// Add our Pod cache transform if enabled
		if transformsEnabled {
			opts = append(opts, informers.WithTransform(TransformPodForCache))
		}

		// Store using the same Key as the default factory to override it
		//nolint:fatcontext // Intentional: we need to add multiple values to context, one per selector
		ctx = context.WithValue(ctx, filteredFactory.Key{Selector: selectorVal},
			informers.NewSharedInformerFactoryWithOptions(c, controller.GetResyncPeriod(ctx), opts...))
	}

	return ctx
}
