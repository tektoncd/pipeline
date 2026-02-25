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
	"strconv"
	"sync"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	externalversions "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	client "github.com/tektoncd/pipeline/pkg/client/injection/client"
	factory "github.com/tektoncd/pipeline/pkg/client/injection/informers/factory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

var (
	// transformsEnabledOnce ensures we only read the ConfigMap once at startup
	transformsEnabledOnce sync.Once
	// transformsEnabledValue caches the result of the ConfigMap check
	transformsEnabledValue bool
)

func init() {
	injection.Default.RegisterInformerFactory(withTransformFactory)
}

// withTransformFactory creates a new informer factory with the cache transform
// applied. This factory replaces the default generated factory by using the
// same context key.
//
// This approach ensures that all Tekton informers use the transform to reduce
// memory usage without modifying the generated code.
//
// The transform can be disabled by setting enable-informer-cache-transforms=false
// in the feature-flags ConfigMap.
func withTransformFactory(ctx context.Context) context.Context {
	c := client.Get(ctx)
	opts := make([]externalversions.SharedInformerOption, 0, 2)

	if injection.HasNamespaceScope(ctx) {
		opts = append(opts, externalversions.WithNamespace(injection.GetNamespaceScope(ctx)))
	}

	// Add the cache transform to strip unnecessary fields from cached objects
	// if the feature is enabled (default: true)
	if informerCacheTransformsEnabled(ctx) {
		opts = append(opts, externalversions.WithTransform(TransformForCache))
		logging.FromContext(ctx).Info("Informer cache transforms enabled for Tekton resources")
	} else {
		logging.FromContext(ctx).Info("Informer cache transforms disabled for Tekton resources")
	}

	// Use the same Key{} as the generated factory to override it
	return context.WithValue(ctx, factory.Key{},
		externalversions.NewSharedInformerFactoryWithOptions(c, controller.GetResyncPeriod(ctx), opts...))
}

// informerCacheTransformsEnabled checks the feature-flags ConfigMap to determine
// if informer cache transforms are enabled. This function reads the ConfigMap
// directly via API call because it runs during informer factory initialization,
// before the config store is attached to context.
//
// The result is cached using sync.Once since multiple informer factories may
// call this function during startup, and the ConfigMap value won't change.
//
// The transforms are enabled by default. Operators can disable the memory
// optimization by setting enable-informer-cache-transforms=false in the
// feature-flags ConfigMap. Changes require a controller restart to take effect.
func informerCacheTransformsEnabled(ctx context.Context) bool {
	transformsEnabledOnce.Do(func() {
		transformsEnabledValue = readTransformsEnabledFromConfigMap(ctx)
	})
	return transformsEnabledValue
}

// readTransformsEnabledFromConfigMap reads the enable-informer-cache-transforms
// setting from the feature-flags ConfigMap.
func readTransformsEnabledFromConfigMap(ctx context.Context) bool {
	logger := logging.FromContext(ctx)
	kubeClient := kubeclient.Get(ctx)

	cm, err := kubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{})
	if err != nil {
		// If we can't read the ConfigMap, default to enabled
		logger.Warnw("Failed to read feature-flags ConfigMap, defaulting to enabled informer cache transforms", "error", err)
		return true
	}

	if val, ok := cm.Data["enable-informer-cache-transforms"]; ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			logger.Warnw("Invalid value for enable-informer-cache-transforms, defaulting to enabled", "value", val, "error", err)
			return true
		}
		return enabled
	}

	// Not set in ConfigMap, default to enabled
	return true
}
