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

package testing

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// ConfigMapFromTestFile creates a v1.ConfigMap from a YAML file
// It loads the YAML file from the testdata folder.
func ConfigMapFromTestFile(t *testing.T, name string) *corev1.ConfigMap {
	t.Helper()

	b, err := os.ReadFile(fmt.Sprintf("testdata/%s.yaml", name))
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}

	var cm corev1.ConfigMap

	// Use "sigs.k8s.io/yaml" since it reads json struct
	// tags so things unmarshal properly
	if err := yaml.Unmarshal(b, &cm); err != nil {
		t.Fatalf("yaml.Unmarshal() = %v", err)
	}

	return &cm
}

// EnableFeatureFlagField enables a boolean feature flag in an existing context (for use in testing).
func EnableFeatureFlagField(ctx context.Context, t *testing.T, flagName string) context.Context {
	t.Helper()
	featureFlags, err := config.NewFeatureFlagsFromMap(map[string]string{
		flagName: "true",
	})

	if err != nil {
		t.Fatalf("Fail to create a feature config: %v", err)
	}

	cfg := &config.Config{
		FeatureFlags: featureFlags,
	}
	return config.ToContext(ctx, cfg)
}
