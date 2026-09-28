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

package resource_test

import (
	"errors"
	"testing"

	"github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/test"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktesting "k8s.io/client-go/testing"
)

// TestCRDRequesterSubmitAlreadyExistsIsNotSpanError verifies that when Create
// returns AlreadyExists (the informer cache lagging behind an existing
// ResolutionRequest), neither the Submit nor the createResolutionRequest span
// records it as an error. AlreadyExists is an expected in-progress outcome.
func TestCRDRequesterSubmitAlreadyExistsIsNotSpanError(t *testing.T) {
	ownerRef := mustParseOwnerReference(t, `
apiVersion: tekton.dev/v1beta1
blockOwnerDeletion: true
controller: true
kind: TaskRun
name: git-clone
uid: 727019c3-4066-4d8b-919e-90660dfd8b55
`)
	request := mustParseRawRequest(t, `
resolverPayload:
  name: git-ec247f5592afcaefa8485e34d2bd80c6
  namespace: namespace
  resolutionSpec:
    params:
    - name: url
      value: https://github.com/tektoncd/catalog
    - name: revision
      value: main
    - name: pathInRepo
      value: task/git-clone/0.6/git-clone.yaml
    url: "https://foo/bar"
`)

	// No seeded ResolutionRequest, so Submit takes the create path.
	testAssets, cancel := getCRDRequester(t, test.Data{})
	defer cancel()
	clients := testAssets.Clients

	// Force Create to return AlreadyExists, mimicking the informer cache
	// lagging behind a request that already exists in the cluster.
	clients.ResolutionRequests.PrependReactor("create", "resolutionrequests",
		func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewAlreadyExists(
				schema.GroupResource{Group: "resolution.tekton.dev", Resource: "resolutionrequests"},
				request.ResolverPayload.Name)
		})

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	// The CRD requester derives its tracer provider from the span in ctx, so
	// start a parent span backed by the recorder before calling Submit.
	ctx, parent := tp.Tracer("test").Start(testAssets.Ctx, "parent")

	crdRequester := resource.NewCRDRequester(clients.ResolutionRequests, testAssets.Informers.ResolutionRequest.Lister())
	requestWithOwner := &ownerRequest{
		Request:  request.Request(),
		ownerRef: *ownerRef,
	}

	_, err := crdRequester.Submit(ctx, resolutioncommon.ResolverName("git"), requestWithOwner)
	parent.End()

	if !errors.Is(err, resolutioncommon.ErrRequestInProgress) {
		t.Fatalf("expected ErrRequestInProgress for AlreadyExists, got %v", err)
	}

	seen := map[string]bool{}
	for _, s := range recorder.Ended() {
		switch s.Name() {
		case "Submit", "createResolutionRequest":
			seen[s.Name()] = true
			if s.Status().Code == codes.Error {
				t.Errorf("span %q: expected no error status for AlreadyExists, got code=%q description=%q",
					s.Name(), s.Status().Code, s.Status().Description)
			}
			for _, e := range s.Events() {
				if e.Name == "exception" {
					t.Errorf("span %q: expected no recorded error for AlreadyExists, but found an exception event", s.Name())
				}
			}
		}
	}
	for _, name := range []string{"Submit", "createResolutionRequest"} {
		if !seen[name] {
			t.Errorf("expected span %q to be recorded, but it was not", name)
		}
	}
}
