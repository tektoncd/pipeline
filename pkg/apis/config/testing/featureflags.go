/*
Copyright 2023 The Tekton Authors

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

package testing

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

// SetFeatureFlags sets the feature-flags ConfigMap values in an existing context (for use in testing)
func SetFeatureFlags(ctx context.Context, t *testing.T, data map[string]string) context.Context {
	t.Helper()
	s := config.NewStore(logging.FromContext(ctx).Named("config-store"))
	s.OnConfigChanged(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetFeatureFlagsConfigName(),
		},
		Data: data,
	})
	return s.ToContext(ctx)
}

// EnableAlphaAPIFields enables alpha features in an existing context (for use in testing)
func EnableAlphaAPIFields(ctx context.Context) context.Context {
	return setEnableAPIFields(ctx, config.AlphaAPIFields)
}

// EnableBetaAPIFields enables beta features in an existing context (for use in testing)
func EnableBetaAPIFields(ctx context.Context) context.Context {
	return setEnableAPIFields(ctx, config.BetaAPIFields)
}

// EnableStableAPIFields enables stable features in an existing context (for use in testing)
func EnableStableAPIFields(ctx context.Context) context.Context {
	return setEnableAPIFields(ctx, config.StableAPIFields)
}

func setEnableAPIFields(ctx context.Context, want string) context.Context {
	featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
		"enable-api-fields": want,
	})
	cfg := &config.Config{
		Defaults: &config.Defaults{
			DefaultTimeoutMinutes: 60,
		},
		FeatureFlags: featureFlags,
	}
	return config.ToContext(ctx, cfg)
}
