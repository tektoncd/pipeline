/*
Copyright 2021 The Tekton Authors

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

package version_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/version"
)

func TestValidateEnabledAPIFields(t *testing.T) {
	tcs := []struct {
		name           string
		wantVersion    string
		currentVersion string
	}{{
		name:           "alpha fields w/ alpha",
		wantVersion:    "alpha",
		currentVersion: "alpha",
	}, {
		name:           "beta fields w/ alpha",
		wantVersion:    "beta",
		currentVersion: "alpha",
	}, {
		name:           "beta fields w/ beta",
		wantVersion:    "beta",
		currentVersion: "beta",
	}, {
		name:           "stable fields w/ alpha",
		wantVersion:    "stable",
		currentVersion: "alpha",
	}, {
		name:           "stable fields w/ beta",
		wantVersion:    "stable",
		currentVersion: "beta",
	}, {
		name:           "stable fields w/ stable",
		wantVersion:    "stable",
		currentVersion: "stable",
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			flags, err := config.NewFeatureFlagsFromMap(map[string]string{
				"enable-api-fields": tc.currentVersion,
			})
			if err != nil {
				t.Fatalf("error creating feature flags from map: %v", err)
			}
			cfg := &config.Config{
				FeatureFlags: flags,
			}
			ctx := config.ToContext(context.Background(), cfg)
			if err := version.ValidateEnabledAPIFields(ctx, "test feature", tc.wantVersion); err != nil {
				t.Errorf("unexpected error for compatible feature gates: %q", err)
			}
		})
	}
}

func TestValidateEnabledAPIFieldsError(t *testing.T) {
	tcs := []struct {
		name           string
		wantVersion    string
		currentVersion string
	}{{
		name:           "alpha fields w/ stable",
		wantVersion:    "alpha",
		currentVersion: "stable",
	}, {
		name:           "alpha fields w/ beta",
		wantVersion:    "alpha",
		currentVersion: "beta",
	}, {
		name:           "beta fields w/ stable",
		wantVersion:    "beta",
		currentVersion: "stable",
	}, {
		name:           "invalid wantVersion",
		wantVersion:    "foo",
		currentVersion: "stable",
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			flags, err := config.NewFeatureFlagsFromMap(map[string]string{
				"enable-api-fields": tc.currentVersion,
			})
			if err != nil {
				t.Fatalf("error creating feature flags from map: %v", err)
			}
			cfg := &config.Config{
				FeatureFlags: flags,
			}
			ctx := config.ToContext(context.Background(), cfg)
			fieldErr := version.ValidateEnabledAPIFields(ctx, "test feature", tc.wantVersion)

			if fieldErr == nil {
				t.Errorf("error expected for incompatible feature gates")
			}
		})
	}
}
