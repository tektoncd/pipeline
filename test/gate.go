package test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
)

// requireGate returns a setup func that will skip the current
// test if the feature-flag with given name does not equal
// given value. It will fatally fail the test if it cannot get
// the feature-flag configmap.
func requireGate(name, value string) func(context.Context, *testing.T, *clients, string) {
	return func(ctx context.Context, t *testing.T, c *clients, namespace string) {
		featureFlagsCM, err := c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get ConfigMap `%s`: %s", config.GetFeatureFlagsConfigName(), err)
		}
		val, ok := featureFlagsCM.Data[name]
		if !ok || val != value {
			t.Skipf("Skipped because feature gate %q != %q", name, value)
		}
	}
}
