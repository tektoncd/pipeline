package testing

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
)

// SetEmbeddedStatus sets the "embedded-status" feature flag in an existing context (for use in testing)
func SetEmbeddedStatus(ctx context.Context, t *testing.T, embeddedStatus string) context.Context {
	t.Helper()
	flags, err := config.NewFeatureFlagsFromMap(map[string]string{
		"embedded-status": embeddedStatus,
	})
	if err != nil {
		t.Fatalf("error creating feature flags from map: %v", err)
	}
	cfg := &config.Config{
		FeatureFlags: flags,
	}
	ctx = config.ToContext(ctx, cfg)
	return ctx
}
