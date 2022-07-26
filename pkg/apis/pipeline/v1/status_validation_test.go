/*
Copyright 2022 The Tekton Authors

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

package v1

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
)

func TestValidateEmbeddedStatus(t *testing.T) {
	status := "minimal"
	flags, err := config.NewFeatureFlagsFromMap(map[string]string{
		"embedded-status": status,
	})
	if err != nil {
		t.Fatalf("error creating feature flags from map: %v", err)
	}
	cfg := &config.Config{
		FeatureFlags: flags,
	}
	ctx := config.ToContext(context.Background(), cfg)
	if err := ValidateEmbeddedStatus(ctx, "test feature", status); err != nil {
		t.Errorf("unexpected error for compatible feature gates: %q", err)
	}
}

func TestValidateEmbeddedStatusError(t *testing.T) {
	flags, err := config.NewFeatureFlagsFromMap(map[string]string{
		"embedded-status": config.FullEmbeddedStatus,
	})
	if err != nil {
		t.Fatalf("error creating feature flags from map: %v", err)
	}
	cfg := &config.Config{
		FeatureFlags: flags,
	}
	ctx := config.ToContext(context.Background(), cfg)
	err = ValidateEmbeddedStatus(ctx, "test feature", config.MinimalEmbeddedStatus)
	if err == nil {
		t.Errorf("error expected for incompatible feature gates")
	}
}
