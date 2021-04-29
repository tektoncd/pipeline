package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		pairs := []string{}
		for name, value := range gates {
			actual, ok := featureFlagsCM.Data[name]
			if ok && value == actual {
				return
			}
			pairs = append(pairs, fmt.Sprintf("%q: %q", name, value))
		}
		t.Skipf("No feature flag matching %s", strings.Join(pairs, " or "))
	}
}
