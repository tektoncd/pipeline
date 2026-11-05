/*
Copyright 2026 The Tekton Authors

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

package scmclient

import "testing"

func TestNew_UnknownProvider(t *testing.T) {
	_, err := New("unknownscm", "", "token")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestNew_KnownProviders(t *testing.T) {
	providers := []string{"github", "gitlab", "gitea", "bitbucketcloud", "bitbucketserver", "azure"}
	for _, p := range providers {
		_, err := New(p, "", "token")
		if err != nil {
			t.Errorf("unexpected error for provider %q: %v", p, err)
		}
	}
}

func TestEnsureGHEEndpoint(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"https://github.com", "https://api.github.com"},
		{"https://github.com/", "https://api.github.com"},
		{"http://github.com", "https://api.github.com"},
		{"https://api.github.com", "https://api.github.com"},
		{"https://github.test_company.com", "https://github.test_company.com/api/v3"},
		{"https://github.test_company.com/", "https://github.test_company.com/api/v3"},
		{"https://github.test_company.com/api/v3", "https://github.test_company.com/api/v3"},
		{"https://github.test_company.com/api/v3/", "https://github.test_company.com/api/v3/"},
	}
	for _, tc := range tests {
		if got := ensureGHEEndpoint(tc.in); got != tc.want {
			t.Errorf("ensureGHEEndpoint(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEnsureBBCEndpoint(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"https://bitbucket.org", "https://api.bitbucket.org"},
		{"http://bitbucket.org/", "https://api.bitbucket.org"},
		{"https://api.bitbucket.org", "https://api.bitbucket.org"},
		{"https://bitbucket.test_company.com", "https://bitbucket.test_company.com"},
	}
	for _, tc := range tests {
		if got := ensureBBCEndpoint(tc.in); got != tc.want {
			t.Errorf("ensureBBCEndpoint(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNew_NormalizesGitHubEnterpriseURL(t *testing.T) {
	c, err := New("github", "https://github.test_company.com/", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := c.(*githubClient).baseURL, "https://github.test_company.com/api/v3"; got != want {
		t.Errorf("baseURL = %q, want %q", got, want)
	}
}

func TestNew_NormalizesBitbucketCloudURL(t *testing.T) {
	c, err := New("bitbucketcloud", "https://bitbucket.org", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := c.(*bitbucketCloudClient).baseURL, "https://api.bitbucket.org"; got != want {
		t.Errorf("baseURL = %q, want %q", got, want)
	}
}
