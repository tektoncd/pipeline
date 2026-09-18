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

package bundle_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	bundleresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
)

// TestMain installs a permissive registry transport for the duration of
// the package's test suite so the broader bundle test cases that talk to
// loopback httptest registries keep working. The dial guard is exercised
// explicitly by TestBundleRetrieveImageRejectsPrivateNetwork.
func TestMain(m *testing.M) {
	bundleresolution.SetRegistryTransportForTest(http.DefaultTransport)
	os.Exit(m.Run())
}

// TestBundleRetrieveImageRejectsPrivateNetwork mirrors the address-class
// table from the deprecated bundle resolver.
func TestBundleRetrieveImageRejectsPrivateNetwork(t *testing.T) {
	prev := bundleresolution.ResetRegistryTransportForTest()
	t.Cleanup(func() { bundleresolution.SetRegistryTransportForTest(prev) })

	cases := []struct {
		name string
		ref  string
	}{
		{name: "loopback v4 literal", ref: "127.0.0.1:5000/foo:bar"},
		{name: "rfc1918 10.x", ref: "10.0.0.1:5000/foo:bar"},
		{name: "rfc1918 172.16", ref: "172.16.0.5:5000/foo:bar"},
		{name: "rfc1918 192.168", ref: "192.168.1.1:5000/foo:bar"},
		{name: "link-local 169.254", ref: "169.254.169.254:5000/foo:bar"},
		{name: "cgnat 100.64", ref: "100.64.0.1:5000/foo:bar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := bundleresolution.RequestOptions{Bundle: tc.ref, EntryName: "n", Kind: "task"}
			_, err := bundleresolution.GetEntry(context.Background(), authn.DefaultKeychain, opts)
			if err == nil {
				t.Fatalf("expected dial guard to reject %s", tc.ref)
			}
			if !strings.Contains(err.Error(), "restricted dial") {
				t.Fatalf("expected restricted-dial error, got %v", err)
			}
		})
	}
}
