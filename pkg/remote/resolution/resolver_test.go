/*
Copyright 2022 The Tekton Authors
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
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remote"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/test/diff"
	resolution "github.com/tektoncd/pipeline/test/resolution"
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
        image: ubuntu
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
		ctx := context.Background()
		owner := &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		}
		resolved := &resolution.ResolvedResource{
			ResolvedData:        tc.resolvedData,
			ResolvedAnnotations: tc.resolvedAnnotations,
		}
		requester := &resolution.Requester{
			SubmitErr:        nil,
			ResolvedResource: resolved,
		}
		resolver := NewResolver(requester, owner, "git", "", "", nil)
		if _, _, err := resolver.Get(ctx, "foo", "bar"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

var invalidPipelineBytes = []byte(`
kind: Pipeline
apiVersion: tekton.dev/v1
metadata:
  name: foo
spec:
  tasks:
  - name: task1
    taskSpec:
      foo: bar
      steps:
      - name: step1
        image: ubuntu
        script: |
          echo "hello world!"
        foo: bar
`)

var invalidTaskBytes = []byte(`
kind: Task
apiVersion: tekton.dev/v1
metadata:
  name: foo
spec:
  foo: bar
  steps:
    - name: step1
      image: ubuntu
      script: |
        echo "hello world!"
`)

var invalidStepActionBytes = []byte(`
kind: StepAction
apiVersion: tekton.dev/v1beta1
metadata:
  name: foo
spec:
  image: ubuntu
  script: |
    echo "hello world!"
  foo: bar
`)

func TestGet_Errors(t *testing.T) {
	genericError := errors.New("uh oh something bad happened")
	notARuntimeObject := &resolution.ResolvedResource{
		ResolvedData:        []byte(">:)"),
		ResolvedAnnotations: nil,
	}
	invalidDataResource := &resolution.ResolvedResource{
		DataErr:             errors.New("data access error"),
		ResolvedAnnotations: nil,
	}
	invalidPipeline := &resolution.ResolvedResource{
		ResolvedData:        invalidPipelineBytes,
		DataErr:             errors.New(`spec.tasks[0].taskSpec.foo", unknown field "spec.tasks[0].taskSpec.steps[0].foo`),
		ResolvedAnnotations: nil,
	}
	invalidTask := &resolution.ResolvedResource{
		ResolvedData:        invalidTaskBytes,
		DataErr:             errors.New(`spec.foo", unknown field "spec.steps[0].foo`),
		ResolvedAnnotations: nil,
	}
	invalidStepAction := &resolution.ResolvedResource{
		ResolvedData:        invalidStepActionBytes,
		DataErr:             errors.New(`unknown field "spec.foo`),
		ResolvedAnnotations: nil,
	}
	for _, tc := range []struct {
		submitErr        error
		expectedGetErr   error
		resolvedResource remoteresource.ResolvedResource
	}{{
		submitErr:        resolutioncommon.ErrRequestInProgress,
		expectedGetErr:   remote.ErrRequestInProgress,
		resolvedResource: nil,
	}, {
		submitErr:        nil,
		expectedGetErr:   ErrNilResource,
		resolvedResource: nil,
	}, {
		submitErr:        genericError,
		expectedGetErr:   genericError,
		resolvedResource: nil,
	}, {
		submitErr:        nil,
		expectedGetErr:   &InvalidRuntimeObjectError{},
		resolvedResource: notARuntimeObject,
	}, {
		submitErr:        nil,
		expectedGetErr:   &DataAccessError{},
		resolvedResource: invalidDataResource,
	}, {
		submitErr:        nil,
		expectedGetErr:   &DataAccessError{},
		resolvedResource: invalidPipeline,
	}, {
		submitErr:        nil,
		expectedGetErr:   &DataAccessError{},
		resolvedResource: invalidTask,
	}, {
		submitErr:        nil,
		expectedGetErr:   &DataAccessError{},
		resolvedResource: invalidStepAction,
	}} {
		ctx := context.Background()
		owner := &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		}
		requester := &resolution.Requester{
			SubmitErr:        tc.submitErr,
			ResolvedResource: tc.resolvedResource,
		}
		resolver := NewResolver(requester, owner, "git", "", "", nil)
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

func TestBuildRequest(t *testing.T) {
	for _, tc := range []struct {
		name            string
		targetName      string
		targetNamespace string
	}{{
		name: "just owner",
	}, {
		name:            "with target name and namespace",
		targetName:      "some-object",
		targetNamespace: "some-ns",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			owner := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}

			req, err := buildRequest("git", owner, tc.targetName, tc.targetNamespace, nil)
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
			expectedReqName, err := remoteresource.GenerateDeterministicName("git", reqNameBase, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if expectedReqName != req.Name() {
				t.Errorf("expected request name %s, but was %s", expectedReqName, req.Name())
			}
		})
	}
}
