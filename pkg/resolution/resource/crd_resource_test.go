/*
Copyright 2023 The Tekton Authors

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
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
	"sigs.k8s.io/yaml"
)

// getCRDRequester returns an instance of the CRDRequester that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getCRDRequester(t *testing.T, d test.Data) (test.Assets, func()) {
	t.Helper()
	return initializeCRDRequesterAssets(t, d)
}

func initializeCRDRequesterAssets(t *testing.T, d test.Data) (test.Assets, func()) {
	t.Helper()
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	c, informers := test.SeedTestData(t, ctx, d)

	return test.Assets{
		Logger:    logging.FromContext(ctx),
		Clients:   c,
		Informers: informers,
		Ctx:       ctx,
	}, cancel
}

func TestCRDRequesterSubmit(t *testing.T) {
	ownerRef := mustParseOwnerReference(t, `
apiVersion: tekton.dev/v1beta1
blockOwnerDeletion: true
controller: true
kind: TaskRun
name: git-clone
uid: 727019c3-4066-4d8b-919e-90660dfd8b55
`)
	request := mustParseRawRequest(t, `
name: git-ec247f5592afcaefa8485e34d2bd80c6
namespace: namespace
params:
  - name: url
    value: https://github.com/tektoncd/catalog
  - name: revision
    value: main
  - name: pathInRepo
    value: task/git-clone/0.6/git-clone.yaml
`)
	baseRR := mustParseResolutionRequest(t, `
kind: "ResolutionRequest"
apiVersion: "resolution.tekton.dev/v1beta1"
metadata:
  name: "git-ec247f5592afcaefa8485e34d2bd80c6"
  namespace: "namespace"
  labels:
    resolution.tekton.dev/type: "git"
  ownerReferences:
  - apiVersion: tekton.dev/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: TaskRun
    name: git-clone
    uid: 727019c3-4066-4d8b-919e-90660dfd8b55
spec:
  params:
    - name: "url"
      value: "https://github.com/tektoncd/catalog"
    - name: "revision"
      value: "main"
    - name: "pathInRepo"
      value: "task/git-clone/0.6/git-clone.yaml"
`)
	createdRR := baseRR.DeepCopy()
	//
	unknownRR := baseRR.DeepCopy()
	unknownRR.Status = *mustParseResolutionRequestStatus(t, `
conditions:
  - lastTransitionTime: "2023-03-26T10:31:29Z"
    status: "Unknown"
    type: Succeeded
`)
	//
	failedRR := baseRR.DeepCopy()
	failedRR.Status = *mustParseResolutionRequestStatus(t, `
conditions:
  - lastTransitionTime: "2023-03-26T10:31:29Z"
    status: "Failed"
    type: Succeeded
    message: "error message"
`)
	//
	successRR := baseRR.DeepCopy()
	successRR.Status = *mustParseResolutionRequestStatus(t, `
annotations:
  resolution.tekton.dev/content-type: application/x-yaml
  resolution.tekton.dev/path: task/git-clone/0.6/git-clone.yaml
  resolution.tekton.dev/revision: main
  resolution.tekton.dev/url: https://github.com/tektoncd/catalog
conditions:
  - lastTransitionTime: "2023-03-26T10:31:29Z"
    status: "True"
    type: Succeeded
    data: e30=
`)
	//
	successWithoutAnnotationsRR := baseRR.DeepCopy()
	successWithoutAnnotationsRR.Status = *mustParseResolutionRequestStatus(t, `
conditions:
  - lastTransitionTime: "2023-03-26T10:31:29Z"
    status: "True"
    type: Succeeded
    data: e30=
`)

	testCases := []struct {
		name                      string
		inputRequest              *test.RawRequest
		inputResolutionRequest    *v1beta1.ResolutionRequest
		expectedResolutionRequest *v1beta1.ResolutionRequest
		expectedResolvedResource  *v1beta1.ResolutionRequest
		expectedErr               error
	}{
		{
			name:                      "resolution request does not exist and needs to be created",
			inputRequest:              request,
			inputResolutionRequest:    nil,
			expectedResolutionRequest: createdRR.DeepCopy(),
			expectedResolvedResource:  nil,
			expectedErr:               resolutioncommon.ErrRequestInProgress,
		},
		{
			name:                      "resolution request exist and status is unknown",
			inputRequest:              request,
			inputResolutionRequest:    unknownRR.DeepCopy(),
			expectedResolutionRequest: nil,
			expectedResolvedResource:  nil,
			expectedErr:               resolutioncommon.ErrRequestInProgress,
		},
		{
			name:                      "resolution request exist and status is succeeded",
			inputRequest:              request,
			inputResolutionRequest:    successRR.DeepCopy(),
			expectedResolutionRequest: nil,
			expectedResolvedResource:  successRR.DeepCopy(),
			expectedErr:               nil,
		},
		{
			name:                      "resolution request exist and status is succeeded but annotations is nil",
			inputRequest:              request,
			inputResolutionRequest:    successWithoutAnnotationsRR.DeepCopy(),
			expectedResolutionRequest: nil,
			expectedResolvedResource:  successWithoutAnnotationsRR.DeepCopy(),
			expectedErr:               nil,
		},
		{
			name:                      "resolution request exist and status is failed",
			inputRequest:              request,
			inputResolutionRequest:    failedRR.DeepCopy(),
			expectedResolutionRequest: nil,
			expectedResolvedResource:  nil,
			expectedErr:               resolutioncommon.NewError(resolutioncommon.ReasonResolutionFailed, errors.New("error message")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{}
			if tc.inputResolutionRequest != nil {
				d.ResolutionRequests = []*v1beta1.ResolutionRequest{tc.inputResolutionRequest}
			}

			testAssets, cancel := getCRDRequester(t, d)
			defer cancel()
			ctx := testAssets.Ctx
			clients := testAssets.Clients

			resolver := resolutioncommon.ResolverName("git")
			crdRequester := resource.NewCRDRequester(clients.ResolutionRequests, testAssets.Informers.ResolutionRequest.Lister())
			requestWithOwner := &ownerRequest{
				Request:  tc.inputRequest.Request(),
				ownerRef: *ownerRef,
			}
			resolvedResource, err := crdRequester.Submit(ctx, resolver, requestWithOwner)

			// check the error
			if err != nil || tc.expectedErr != nil {
				if err == nil || tc.expectedErr == nil {
					t.Errorf("expected error %v, but got %v", tc.expectedErr, err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("expected error %v, but got %v", tc.expectedErr, err)
				}
			}

			// check the resolved resource
			switch {
			case tc.expectedResolvedResource == nil:
				// skipping check of resolved resources.
			case tc.expectedResolvedResource != nil:
				if resolvedResource == nil {
					t.Errorf("expected resolved resource equal %v, but got %v", tc.expectedResolvedResource, resolvedResource)
					break
				}
				rr := tc.expectedResolvedResource
				data, err := base64.StdEncoding.Strict().DecodeString(rr.Status.Data)
				if err != nil {
					t.Errorf("unexpected error decoding expected resource data: %v", err)
				}
				expectedResolvedResource := test.NewResolvedResource(data, rr.Status.Annotations, rr.Status.RefSource, nil)
				assertResolvedResourceEqual(t, expectedResolvedResource, resolvedResource)
			}

			// check the resolution request
			if tc.expectedResolutionRequest != nil {
				resolutionrequest, err := clients.ResolutionRequests.ResolutionV1beta1().
					ResolutionRequests(tc.inputRequest.Namespace).Get(ctx, tc.inputRequest.Name, metav1.GetOptions{})
				if err != nil {
					t.Errorf("unexpected error getting resource requests: %v", err)
				}
				if d := cmp.Diff(tc.expectedResolutionRequest, resolutionrequest); d != "" {
					t.Errorf("expected resolution request to match %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

type ownerRequest struct {
	resolutioncommon.Request
	ownerRef metav1.OwnerReference
}

func (r *ownerRequest) OwnerRef() metav1.OwnerReference {
	return r.ownerRef
}

func mustParseRawRequest(t *testing.T, yamlStr string) *test.RawRequest {
	t.Helper()
	output := &test.RawRequest{}
	if err := yaml.Unmarshal([]byte(yamlStr), output); err != nil {
		t.Errorf("parsing raw request %s: %v", yamlStr, err)
	}
	return output
}

func mustParseOwnerReference(t *testing.T, yamlStr string) *metav1.OwnerReference {
	t.Helper()
	output := &metav1.OwnerReference{}
	if err := yaml.Unmarshal([]byte(yamlStr), output); err != nil {
		t.Errorf("parsing owner reference %s: %v", yamlStr, err)
	}
	return output
}

func mustParseResolutionRequest(t *testing.T, yamlStr string) *v1beta1.ResolutionRequest {
	t.Helper()
	output := &v1beta1.ResolutionRequest{}
	if err := yaml.Unmarshal([]byte(yamlStr), output); err != nil {
		t.Errorf("parsing resolution request %s: %v", yamlStr, err)
	}
	return output
}

func mustParseResolutionRequestStatus(t *testing.T, yamlStr string) *v1beta1.ResolutionRequestStatus {
	t.Helper()
	output := &v1beta1.ResolutionRequestStatus{}
	if err := yaml.Unmarshal([]byte(yamlStr), output); err != nil {
		t.Errorf("parsing resolution request status %s: %v", yamlStr, err)
	}
	return output
}

func assertResolvedResourceEqual(t *testing.T, expected, actual resolutioncommon.ResolvedResource) {
	t.Helper()
	expectedBytes, err := expected.Data()
	if err != nil {
		t.Errorf("unexpected error getting expected resource data: %v", err)
	}
	actualBytes, err := actual.Data()
	if err != nil {
		t.Errorf("unexpected error getting acutal resource data: %v", err)
	}
	if d := cmp.Diff(expectedBytes, actualBytes); d != "" {
		t.Errorf("expected resolved resource Data to match %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(expected.Annotations(), actual.Annotations()); d != "" {
		t.Errorf("expected resolved resource Annotations to match %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(expected.RefSource(), actual.RefSource()); d != "" {
		t.Errorf("expected resolved resource Source to match %s", diff.PrintWantGot(d))
	}
}
