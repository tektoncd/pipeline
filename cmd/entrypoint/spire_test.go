//go:build !disable_spire

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

package main

import (
	"context"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/pkg/spire"
	"github.com/tektoncd/pipeline/pkg/spire/config"
	"github.com/tektoncd/pipeline/pkg/spire/test"
	"github.com/tektoncd/pipeline/pkg/spire/test/fakeworkloadapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInitializeSpireAPI_Disabled(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		socket  string
	}{{
		name:   "flag disabled",
		socket: "unix:///unused",
	}, {
		name:    "socket empty",
		enabled: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			oldEnabled, oldSocket := *enableSpire, *socketPath
			t.Cleanup(func() {
				*enableSpire, *socketPath = oldEnabled, oldSocket
			})
			*enableSpire, *socketPath = tc.enabled, tc.socket

			if got := initializeSpireAPI(); got != nil {
				t.Fatal("expected no SPIRE client")
			}
		})
	}
}

func TestInitializeSpireAPI_SignsResults(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	td := spiffeid.RequireTrustDomainFromString("example.org")
	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()
	wl.SetX509Bundles(ca.X509Bundle())
	wl.SetX509SVIDResponse(&fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  []*x509svid.SVID{ca.CreateX509SVID(spiffeid.RequireFromPath(td, "/ns/default/taskrun/task"))},
	})
	oldEnabled, oldSocket := *enableSpire, *socketPath
	t.Cleanup(func() {
		*enableSpire, *socketPath = oldEnabled, oldSocket
	})
	*enableSpire, *socketPath = true, wl.Addr()

	client := initializeSpireAPI()
	if client == nil {
		t.Fatal("expected a SPIRE client")
	}
	defer client.Close()
	results := []result.RunResult{{Key: "answer", Value: "42", ResultType: result.TaskRunResultType}}

	signed, err := client.Sign(ctx, results)
	if err != nil {
		t.Fatal(err)
	}

	verifier := spire.GetControllerAPIClient(ctx)
	verifier.SetConfig(config.SpireConfig{SocketPath: wl.Addr(), ServerAddr: "unused", TrustDomain: td.String()})
	defer verifier.Close()
	tr := &v1beta1.TaskRun{ObjectMeta: metav1.ObjectMeta{Name: "task", Namespace: "default"}}
	if err := verifier.VerifyTaskRunResults(ctx, append(results, signed...), tr); err != nil {
		t.Fatalf("verify results signed by the configured entrypoint client: %v", err)
	}
}
