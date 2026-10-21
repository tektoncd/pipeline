/*
Copyright 2024 The Tekton Authors

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

package git

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestGetGitResolverConfig(t *testing.T) {
	tests := []struct {
		name           string
		wantErr        bool
		expectedErr    string
		config         map[string]string
		expectedConfig GitResolverConfig
	}{
		{
			name:           "no config",
			config:         map[string]string{},
			expectedConfig: GitResolverConfig{},
		},
		{
			name: "default config",
			config: map[string]string{
				DefaultURLKey:      "https://github.com",
				DefaultRevisionKey: "main",
				DefaultOrgKey:      "tektoncd",
			},
			expectedConfig: GitResolverConfig{
				"default": ScmConfig{
					URL:      "https://github.com",
					Revision: "main",
					Org:      "tektoncd",
				},
			},
		},
		{
			name: "default config with default key",
			config: map[string]string{
				"default." + DefaultURLKey:      "https://github.com",
				"default." + DefaultRevisionKey: "main",
			},
			expectedConfig: GitResolverConfig{
				"default": ScmConfig{
					URL:      "https://github.com",
					Revision: "main",
				},
			},
		},
		{
			name: "config with custom key",
			config: map[string]string{
				"test." + DefaultURLKey:      "https://github.com",
				"test." + DefaultRevisionKey: "main",
			},
			expectedConfig: GitResolverConfig{
				"test": ScmConfig{
					URL:      "https://github.com",
					Revision: "main",
				},
			},
		},
		{
			name: "config with custom key and no key",
			config: map[string]string{
				DefaultURLKey:                "https://github.com",
				DefaultRevisionKey:           "main",
				"test." + DefaultURLKey:      "https://github.com",
				"test." + DefaultRevisionKey: "main",
			},
			expectedConfig: GitResolverConfig{
				"default": ScmConfig{
					URL:      "https://github.com",
					Revision: "main",
				},
				"test": ScmConfig{
					URL:      "https://github.com",
					Revision: "main",
				},
			},
		},
		{
			name: "config with both default and custom key",
			config: map[string]string{
				"default." + DefaultURLKey:      "https://github.com",
				"default." + DefaultRevisionKey: "main",
				"test." + DefaultURLKey:         "https://github.com",
				"test." + DefaultRevisionKey:    "main",
			},
			expectedConfig: GitResolverConfig{
				"default": ScmConfig{
					URL:      "https://github.com",
					Revision: "main",
				},
				"test": ScmConfig{
					URL:      "https://github.com",
					Revision: "main",
				},
			},
		},
		{
			name: "config with invalid format",
			config: map[string]string{
				"default.." + DefaultURLKey: "https://github.com",
			},
			wantErr:        true,
			expectedErr:    "key default..default-url passed in git resolver configmap is invalid",
			expectedConfig: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := resolutionframework.InjectResolverConfigToContext(t.Context(), tc.config)
			gitResolverConfig, err := GetGitResolverConfig(ctx)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("unexpected error parsing git resolver config: %v", err)
				}
				if d := cmp.Diff(tc.expectedErr, err.Error()); d != "" {
					t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
				}
			}
			if d := cmp.Diff(tc.expectedConfig, gitResolverConfig); d != "" {
				t.Errorf("expected config: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetGitResolverBackoffDefaults(t *testing.T) {
	ctx := resolutionframework.InjectResolverConfigToContext(t.Context(), map[string]string{})
	backoff := GetGitResolverBackoff(ctx)
	if backoff.Duration != DefaultBackoffDuration {
		t.Fatalf("expected default duration %v, got %v", DefaultBackoffDuration, backoff.Duration)
	}
	if backoff.Factor != DefaultBackoffFactor {
		t.Fatalf("expected default factor %v, got %v", DefaultBackoffFactor, backoff.Factor)
	}
	if backoff.Jitter != DefaultBackoffJitter {
		t.Fatalf("expected default jitter %v, got %v", DefaultBackoffJitter, backoff.Jitter)
	}
	if backoff.Steps != DefaultBackoffSteps {
		t.Fatalf("expected default steps %v, got %v", DefaultBackoffSteps, backoff.Steps)
	}
	if backoff.Cap != DefaultBackoffCap {
		t.Fatalf("expected default cap %v, got %v", DefaultBackoffCap, backoff.Cap)
	}
}

func TestGetGitResolverBackoffCustom(t *testing.T) {
	configBackoffDuration := 7 * time.Second
	configBackoffFactor := 7.0
	configBackoffJitter := 0.5
	configBackoffSteps := 3
	configBackoffCap := 20 * time.Second
	config := map[string]string{
		ConfigBackoffDuration: configBackoffDuration.String(),
		ConfigBackoffFactor:   strconv.FormatFloat(configBackoffFactor, 'f', -1, 64),
		ConfigBackoffJitter:   strconv.FormatFloat(configBackoffJitter, 'f', -1, 64),
		ConfigBackoffSteps:    strconv.Itoa(configBackoffSteps),
		ConfigBackoffCap:      configBackoffCap.String(),
	}
	ctx := resolutionframework.InjectResolverConfigToContext(t.Context(), config)
	backoff := GetGitResolverBackoff(ctx)
	if backoff.Duration != configBackoffDuration {
		t.Fatalf("expected duration %v, got %v", configBackoffDuration, backoff.Duration)
	}
	if backoff.Factor != configBackoffFactor {
		t.Fatalf("expected factor %v, got %v", configBackoffFactor, backoff.Factor)
	}
	if backoff.Jitter != configBackoffJitter {
		t.Fatalf("expected jitter %v, got %v", configBackoffJitter, backoff.Jitter)
	}
	if backoff.Steps != configBackoffSteps {
		t.Fatalf("expected steps %v, got %v", configBackoffSteps, backoff.Steps)
	}
	if backoff.Cap != configBackoffCap {
		t.Fatalf("expected cap %v, got %v", configBackoffCap, backoff.Cap)
	}
}

func TestGetGitResolverBackoffInvalidValuesFallBackToDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]string
	}{
		{
			name: "unparseable values",
			config: map[string]string{
				ConfigBackoffDuration: "not-a-duration",
				ConfigBackoffFactor:   "not-a-float",
				ConfigBackoffJitter:   "not-a-float",
				ConfigBackoffSteps:    "not-an-int",
				ConfigBackoffCap:      "not-a-duration",
			},
		},
		{
			name: "negative duration",
			config: map[string]string{
				ConfigBackoffDuration: "-1s",
			},
		},
		{
			name: "negative factor",
			config: map[string]string{
				ConfigBackoffFactor: "-1.0",
			},
		},
		{
			name: "negative jitter",
			config: map[string]string{
				ConfigBackoffJitter: "-0.5",
			},
		},
		{
			name: "zero steps",
			config: map[string]string{
				ConfigBackoffSteps: "0",
			},
		},
		{
			name: "negative steps",
			config: map[string]string{
				ConfigBackoffSteps: "-1",
			},
		},
		{
			name: "negative cap",
			config: map[string]string{
				ConfigBackoffCap: "-5s",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := resolutionframework.InjectResolverConfigToContext(t.Context(), tc.config)
			backoff := GetGitResolverBackoff(ctx)
			if backoff.Duration != DefaultBackoffDuration {
				t.Errorf("expected default duration %v, got %v", DefaultBackoffDuration, backoff.Duration)
			}
			if backoff.Factor != DefaultBackoffFactor {
				t.Errorf("expected default factor %v, got %v", DefaultBackoffFactor, backoff.Factor)
			}
			if backoff.Jitter != DefaultBackoffJitter {
				t.Errorf("expected default jitter %v, got %v", DefaultBackoffJitter, backoff.Jitter)
			}
			if backoff.Steps != DefaultBackoffSteps {
				t.Errorf("expected default steps %v, got %v", DefaultBackoffSteps, backoff.Steps)
			}
			if backoff.Cap != DefaultBackoffCap {
				t.Errorf("expected default cap %v, got %v", DefaultBackoffCap, backoff.Cap)
			}
		})
	}
}
