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

package hub

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

// TestMain installs a permissive HTTP client for the duration of the
// package's test suite so that existing httptest.Server-based tests
// (bound to 127.0.0.1) continue to work. Tests that exercise the
// private-network dial block explicitly reinstall the restricted
// transport via installRestrictedClientForTest.
func TestMain(m *testing.M) {
	hubHTTPClient = &http.Client{Transport: http.DefaultTransport}
	os.Exit(m.Run())
}

// installRestrictedClientForTest swaps hubHTTPClient for the production
// restricted-transport client for the duration of the calling test.
func installRestrictedClientForTest(t *testing.T) {
	t.Helper()
	original := hubHTTPClient
	hubHTTPClient = &http.Client{Transport: framework.RestrictedHTTPTransport()}
	t.Cleanup(func() { hubHTTPClient = original })
}

// TestFetchHubResourceRejectsPrivateNetwork verifies the dial guard rejects
// requests whose URL resolves to a disallowed address class.
func TestFetchHubResourceRejectsPrivateNetwork(t *testing.T) {
	installRestrictedClientForTest(t)
	cases := []struct {
		name string
		url  string
	}{
		{name: "loopback v4 literal", url: "http://127.0.0.1:80/foo"},
		{name: "loopback v6 literal", url: "http://[::1]:80/foo"},
		{name: "rfc1918 10.x", url: "http://10.0.0.1:80/foo"},
		{name: "rfc1918 172.16", url: "http://172.16.0.5:80/foo"},
		{name: "rfc1918 192.168", url: "http://192.168.1.1:80/foo"},
		{name: "link-local 169.254", url: "http://169.254.169.254:80/foo"},
		{name: "cgnat 100.64", url: "http://100.64.0.1:80/foo"},
		{name: "ipv6 ula", url: "http://[fd00::1]:80/foo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp any
			err := fetchHubResource(context.Background(), tc.url, &resp)
			if err == nil {
				t.Fatalf("expected dial guard to reject %s", tc.url)
			}
			if !strings.Contains(err.Error(), "restricted dial") {
				t.Fatalf("expected restricted-dial error, got %v", err)
			}
		})
	}
}
