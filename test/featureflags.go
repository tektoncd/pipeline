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

package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
)

// requireAnyGate returns a setup func that will skip the current
// test if none of the feature-flags in the given map match
// what's in the feature-flags ConfigMap. It will fatally fail
// the test if it cannot get the feature-flag configmap.
func requireAnyGate(gates map[string]string) func(context.Context, *testing.T, *clients, string) {
	return func(ctx context.Context, t *testing.T, c *clients, namespace string) {
		t.Helper()
		featureFlagsCM, err := c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get ConfigMap `%s`: %s", config.GetFeatureFlagsConfigName(), err)
		}
		resolverFeatureFlagsCM, err := c.KubeClient.CoreV1().ConfigMaps(resolverconfig.ResolversNamespace(system.Namespace())).
			Get(ctx, resolverconfig.GetFeatureFlagsConfigName(), metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			t.Fatalf("Failed to get ConfigMap `%s`: %s", resolverconfig.GetFeatureFlagsConfigName(), err)
		}
		resolverMap := make(map[string]string)
		if resolverFeatureFlagsCM != nil {
			resolverMap = resolverFeatureFlagsCM.Data
		}
		pairs := []string{}
		for name, value := range gates {
			actual, ok := featureFlagsCM.Data[name]
			if ok && value == actual {
				return
			}
			actual, ok = resolverMap[name]
			if ok && value == actual {
				return
			}
			pairs = append(pairs, fmt.Sprintf("%q: %q", name, value))
		}
		t.Skipf("No feature flag in namespace %q matching %s\nExisting feature flag: %#v\nExisting resolver feature flag (in namespace %q): %#v",
			system.Namespace(), strings.Join(pairs, " or "), featureFlagsCM.Data,
			resolverconfig.ResolversNamespace(system.Namespace()), resolverMap)
	}
}

// requireAllgates returns a setup func that will skip the current
// test if all of the feature-flags in the given map don't match
// what's in the feature-flags ConfigMap. It will fatally fail
// the test if it cannot get the feature-flag configmap.
func requireAllGates(gates map[string]string) func(context.Context, *testing.T, *clients, string) {
	return func(ctx context.Context, t *testing.T, c *clients, namespace string) {
		t.Helper()
		featureFlagsCM, err := c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get ConfigMap `%s`: %s", config.GetFeatureFlagsConfigName(), err)
		}
		resolverFeatureFlagsCM, err := c.KubeClient.CoreV1().ConfigMaps(resolverconfig.ResolversNamespace(system.Namespace())).
			Get(ctx, resolverconfig.GetFeatureFlagsConfigName(), metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			t.Fatalf("Failed to get ConfigMap `%s`: %s", resolverconfig.GetFeatureFlagsConfigName(), err)
		}
		resolverMap := make(map[string]string)
		if resolverFeatureFlagsCM != nil {
			resolverMap = resolverFeatureFlagsCM.Data
		}
		pairs := []string{}
		for name, value := range gates {
			actual, ok := featureFlagsCM.Data[name]
			if !ok {
				actual, ok = resolverMap[name]
				if !ok || value != actual {
					pairs = append(pairs, fmt.Sprintf("%q is %q, want %s", name, actual, value))
				}
			} else if value != actual {
				pairs = append(pairs, fmt.Sprintf("%q is %q, want %s", name, actual, value))
			}
		}
		if len(pairs) > 0 {
			t.Skipf("One or more feature flags not matching required: %s", strings.Join(pairs, "; "))
		}
	}
}
