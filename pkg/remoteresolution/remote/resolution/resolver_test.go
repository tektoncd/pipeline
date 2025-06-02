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

package resolution

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resv1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"

	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/remote/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/test/diff"
	test "github.com/tektoncd/pipeline/test/remoteresolution"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
)

var pipelineBytes = []byte(`
kind: Pipeline
apiVersion: tekton.dev/v1beta1
metadata:
  name: foo
spec:
  tasks:
  - name: task1
    taskSpec:
      steps:
      - name: step1
        image: docker.io/library/ubuntu
        script: |
          echo "hello world!"
`)

func TestGet_Successful(t *testing.T) {
	for _, tc := range []struct {
		resolvedData        []byte
		resolvedAnnotations map[string]string
	}{{
		resolvedData:        pipelineBytes,
		resolvedAnnotations: nil,
	}} {
		ctx := t.Context()
		owner := &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		}
		resolved := &test.ResolvedResource{
			ResolvedData:        tc.resolvedData,
			ResolvedAnnotations: tc.resolvedAnnotations,
		}
		requester := &test.Requester{
			SubmitErr:        nil,
			ResolvedResource: resolved,
		}
		resolver := NewResolver(requester, owner, "git", remoteresource.ResolverPayload{})
		if _, _, err := resolver.Get(ctx, "foo", "bar"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestGet_Errors(t *testing.T) {
	genericError := errors.New("uh oh something bad happened")
	notARuntimeObject := &test.ResolvedResource{
		ResolvedData:        []byte(">:)"),
		ResolvedAnnotations: nil,
	}
	invalidDataResource := &test.ResolvedResource{
		DataErr:             errors.New("data access error"),
		ResolvedAnnotations: nil,
	}
	for _, tc := range []struct {
		submitErr        error
		expectedGetErr   error
		resolvedResource remoteresource.ResolvedResource
	}{{
		submitErr:        common.ErrRequestInProgress,
		expectedGetErr:   remote.ErrRequestInProgress,
		resolvedResource: nil,
	}, {
		submitErr:        nil,
		expectedGetErr:   resolution.ErrNilResource,
		resolvedResource: nil,
	}, {
		submitErr:        genericError,
		expectedGetErr:   genericError,
		resolvedResource: nil,
	}, {
		submitErr:        nil,
		expectedGetErr:   &resolution.InvalidRuntimeObjectError{},
		resolvedResource: notARuntimeObject,
	}, {
		submitErr:        nil,
		expectedGetErr:   &resolution.DataAccessError{},
		resolvedResource: invalidDataResource,
	}} {
		ctx := t.Context()
		owner := &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		}
		requester := &test.Requester{
			SubmitErr:        tc.submitErr,
			ResolvedResource: tc.resolvedResource,
		}
		resolver := NewResolver(requester, owner, "git", remoteresource.ResolverPayload{})
		obj, refSource, err := resolver.Get(ctx, "foo", "bar")
		if obj != nil {
			t.Errorf("received unexpected resolved resource")
		}
		if refSource != nil {
			t.Errorf("expected refSource is nil, but received %v", refSource)
		}
		if !errors.Is(err, tc.expectedGetErr) {
			t.Fatalf("expected %v received %v", tc.expectedGetErr, err)
		}
	}
}

func TestBuildRequestV2(t *testing.T) {
	for _, tc := range []struct {
		name            string
		targetName      string
		targetNamespace string
		url             string
	}{{
		name: "just owner",
	}, {
		name:            "with target name and namespace",
		targetName:      "some-object",
		targetNamespace: "some-ns",
	}, {
		name:            "with target name, namespace, and url",
		targetName:      "some-object",
		targetNamespace: "some-ns",
		url:             "scheme://value",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			owner := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}

			rr := &remoteresource.ResolverPayload{Name: tc.targetName, Namespace: tc.targetNamespace}
			rr.ResolutionSpec = &resv1beta1.ResolutionRequestSpec{URL: tc.url}
			req, err := buildRequest("git", owner, rr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d := cmp.Diff(*kmeta.NewControllerRef(owner), req.OwnerRef()); d != "" {
				t.Errorf("expected matching owner ref but got %s", diff.PrintWantGot(d))
			}
			reqNameBase := owner.Namespace + "/" + owner.Name
			if tc.targetName != "" {
				reqNameBase = tc.targetNamespace + "/" + tc.targetName
			}
			expectedReqName, err := resource.GenerateDeterministicNameFromSpec("git", reqNameBase, rr.ResolutionSpec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if expectedReqName != req.ResolverPayload().Name {
				t.Errorf("expected request name %s, but was %s", expectedReqName, req.ResolverPayload().Name)
			}
		})
	}
}
