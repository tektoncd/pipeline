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
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/system"
)

// requireAnyGate returns a setup func that will skip the current
// test if none of the feature-flags in the given map match
// what's in the feature-flags ConfigMap. It will fatally fail
// the test if it cannot get the feature-flag configmap.
func requireAnyGate(gates map[string]string) func(context.Context, *testing.T, *clients, string) {
	return func(ctx context.Context, t *testing.T, c *clients, namespace string) {
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

// GetEmbeddedStatus gets the current value for the "embedded-status" feature flag.
// If the flag is not set, it returns the default value.
func GetEmbeddedStatus(ctx context.Context, t *testing.T, kubeClient kubernetes.Interface) string {
	featureFlagsCM, err := kubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap `%s`: %s", config.GetFeatureFlagsConfigName(), err)
	}
	val := featureFlagsCM.Data["embedded-status"]
	if val == "" {
		return config.DefaultEmbeddedStatus
	}
	return val
}
