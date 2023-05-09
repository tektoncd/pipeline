/*
Copyright 2019 The Tekton Authors

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

package config_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test/diff"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestStoreLoadWithContext(t *testing.T) {
	defaultConfig := test.ConfigMapFromTestFile(t, "config-defaults")
	featuresConfig := test.ConfigMapFromTestFile(t, "feature-flags-all-flags-set")
	metricsConfig := test.ConfigMapFromTestFile(t, "config-observability")
	spireConfig := test.ConfigMapFromTestFile(t, "config-spire")

	expectedDefaults, _ := config.NewDefaultsFromConfigMap(defaultConfig)
	expectedFeatures, _ := config.NewFeatureFlagsFromConfigMap(featuresConfig)
	metrics, _ := config.NewMetricsFromConfigMap(metricsConfig)
	expectedSpireConfig, _ := config.NewSpireConfigFromConfigMap(spireConfig)

	expected := &config.Config{
		Defaults:     expectedDefaults,
		FeatureFlags: expectedFeatures,
		Metrics:      metrics,
		SpireConfig:  expectedSpireConfig,
	}

	store := config.NewStore(logtesting.TestLogger(t))
	store.OnConfigChanged(defaultConfig)
	store.OnConfigChanged(featuresConfig)
	store.OnConfigChanged(metricsConfig)
	store.OnConfigChanged(spireConfig)

	cfg := config.FromContext(store.ToContext(context.Background()))

	if d := cmp.Diff(cfg, expected); d != "" {
		t.Errorf("Unexpected config %s", diff.PrintWantGot(d))
	}
}

func TestStoreLoadWithContext_Empty(t *testing.T) {
	want := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
		Metrics:      config.DefaultMetrics.DeepCopy(),
		SpireConfig:  config.DefaultSpire.DeepCopy(),
	}

	store := config.NewStore(logtesting.TestLogger(t))

	got := config.FromContext(store.ToContext(context.Background()))

	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Unexpected config %s", diff.PrintWantGot(d))
	}
}
