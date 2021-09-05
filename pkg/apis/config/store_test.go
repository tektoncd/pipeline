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
	artifactBucketConfig := test.ConfigMapFromTestFile(t, "config-artifact-bucket")
	artifactPVCConfig := test.ConfigMapFromTestFile(t, "config-artifact-pvc")
	metricsConfig := test.ConfigMapFromTestFile(t, "config-observability")

	expectedDefaults, _ := config.NewDefaultsFromConfigMap(defaultConfig)
	expectedFeatures, _ := config.NewFeatureFlagsFromConfigMap(featuresConfig)
	expectedArtifactBucket, _ := config.NewArtifactBucketFromConfigMap(artifactBucketConfig)
	expectedArtifactPVC, _ := config.NewArtifactPVCFromConfigMap(artifactPVCConfig)
	metrics, _ := config.NewMetricsFromConfigMap(metricsConfig)

	expected := &config.Config{
		Defaults:       expectedDefaults,
		FeatureFlags:   expectedFeatures,
		ArtifactBucket: expectedArtifactBucket,
		ArtifactPVC:    expectedArtifactPVC,
		Metrics:        metrics,
	}

	store := config.NewStore(logtesting.TestLogger(t))
	store.OnConfigChanged(defaultConfig)
	store.OnConfigChanged(featuresConfig)
	store.OnConfigChanged(artifactBucketConfig)
	store.OnConfigChanged(artifactPVCConfig)
	store.OnConfigChanged(metricsConfig)

	cfg := config.FromContext(store.ToContext(context.Background()))

	if d := cmp.Diff(cfg, expected); d != "" {
		t.Errorf("Unexpected config %s", diff.PrintWantGot(d))
	}
}
