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

package cachetransform

import (
	"context"
	"strconv"
	"sync"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

// featureFlag is the feature-flags ConfigMap key that toggles the cache
// transforms. It is enabled by default; operators can opt out by setting it to
// "false". Changes require a controller restart to take effect.
const featureFlag = "enable-informer-cache-transforms"

var (
	// enabledOnce ensures we only read the ConfigMap once at startup.
	enabledOnce sync.Once
	// enabledValue caches the result of the ConfigMap check.
	enabledValue bool
)

// Setup applies a cache transform to an informer if the feature is enabled.
//
// It must be called before the informer starts (i.e. in the controller's
// NewController constructor). SetTransform returns an error only if the
// informer has already started, which would be a programming error, so we fail
// fast. When called multiple times on the same informer the last call wins;
// this is harmless because all callers use the same transform function.
func Setup(ctx context.Context, informer cache.SharedIndexInformer, transformFn cache.TransformFunc) {
	if !enabled(ctx) {
		return
	}
	if err := informer.SetTransform(transformFn); err != nil {
		logging.FromContext(ctx).Fatalf("Failed to set informer cache transform: %v", err)
	}
}

// enabled reports whether informer cache transforms are enabled, reading the
// enable-informer-cache-transforms key from the feature-flags ConfigMap.
//
// The ConfigMap is read directly via the Kubernetes API because Setup runs
// during controller construction, before the config store is attached to
// context. The result is cached with sync.Once since the value cannot change
// without a controller restart and Setup is called for several informers.
func enabled(ctx context.Context) bool {
	enabledOnce.Do(func() {
		enabledValue = readEnabledFromConfigMap(ctx)
		if enabledValue {
			logging.FromContext(ctx).Info("Informer cache transforms enabled")
		} else {
			logging.FromContext(ctx).Info("Informer cache transforms disabled")
		}
	})
	return enabledValue
}

// readEnabledFromConfigMap reads the feature flag from the feature-flags
// ConfigMap, defaulting to enabled when the ConfigMap or key is missing or
// invalid.
func readEnabledFromConfigMap(ctx context.Context) bool {
	logger := logging.FromContext(ctx)
	kubeClient := kubeclient.Get(ctx)

	cm, err := kubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{})
	if err != nil {
		logger.Warnw("Failed to read feature-flags ConfigMap, defaulting to enabled informer cache transforms", "error", err)
		return true
	}

	val, ok := cm.Data[featureFlag]
	if !ok {
		return true
	}

	enabled, err := strconv.ParseBool(val)
	if err != nil {
		logger.Warnw("Invalid value for "+featureFlag+", defaulting to enabled", "value", val, "error", err)
		return true
	}
	return enabled
}
