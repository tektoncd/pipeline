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

package spire

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"strings"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/pkg/spire/config"
	"github.com/tektoncd/pipeline/pkg/spire/test"
	"github.com/tektoncd/pipeline/pkg/spire/test/fakeworkloadapi"
)

func TestSignRejectsUnsupportedSigningKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	td := spiffeid.RequireTrustDomainFromString("example.org")
	id := spiffeid.RequireFromPath(td, "/ns/default/taskrun/task")
	root, rootKey := test.CreateCACertificate(t, nil, nil)
	template, _ := test.CreateX509SVID(t, root, rootKey, id)
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	leaf := test.CreateCertificate(t, template, root, key.Public(), rootKey)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()
	wl.SetX509SVIDResponse(&fakeworkloadapi.X509SVIDResponse{
		Bundle: x509bundle.FromX509Authorities(td, []*x509.Certificate{root}),
		SVIDs: []*x509svid.SVID{{
			ID:           id,
			Certificates: []*x509.Certificate{leaf},
			PrivateKey:   key,
		}},
	})

	// Validate the identity through the real client before testing signing failures.
	api, err := workloadapi.New(ctx, workloadapi.WithAddr(wl.Addr()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := api.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := api.FetchX509SVID(ctx); err != nil {
		t.Fatalf("fetch valid Ed25519 identity: %v", err)
	}

	for _, tc := range []struct {
		name    string
		results []result.RunResult
	}{{
		name: "result signature",
		results: []result.RunResult{{
			Key:        "answer",
			Value:      "42",
			ResultType: result.TaskRunResultType,
		}},
	}, {
		name: "manifest signature",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewEntrypointerAPIClient(&config.SpireConfig{SocketPath: wl.Addr()})
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			})

			signed, err := client.Sign(ctx, tc.results)

			if err == nil || !strings.Contains(err.Error(), "ed25519") {
				t.Fatalf("Sign() error = %v, want an Ed25519 signing error", err)
			}
			if signed != nil {
				t.Fatalf("Sign() returned partially signed results: %v", signed)
			}
		})
	}
}
