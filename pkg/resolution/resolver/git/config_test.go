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
	"testing"

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
