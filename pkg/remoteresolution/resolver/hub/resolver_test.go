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

package hub

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	hubresolver "github.com/tektoncd/pipeline/pkg/resolution/resolver/hub"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(t.Context())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != LabelValueHubResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		testName     string
		kind         string
		version      string
		catalog      string
		resourceName string
		hubType      string
		expectedErr  error
	}{
		{
			testName:     "artifact type validation",
			kind:         "task",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      ArtifactHubType,
		},
		{
			testName:     "stepaction validation",
			kind:         "stepaction",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      ArtifactHubType,
		},
		{
			testName:     "tekton type validation",
			kind:         "task",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      TektonHubType,
			expectedErr:  errors.New("failed to validate params: please configure TEKTON_HUB_API env variable to use tekton type"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			resolver := Resolver{}
			params := map[string]string{
				hubresolver.ParamKind:    tc.kind,
				hubresolver.ParamName:    tc.resourceName,
				hubresolver.ParamVersion: tc.version,
				hubresolver.ParamCatalog: tc.catalog,
				hubresolver.ParamType:    tc.hubType,
			}
			req := v1beta1.ResolutionRequestSpec{
				Params: toParams(params),
			}
			err := resolver.Validate(contextWithConfig(), &req)
			if tc.expectedErr != nil {
				checkExpectedErr(t, tc.expectedErr, err)
			} else if err != nil {
				t.Fatalf("unexpected error validating params: %v", err)
			}
		})
	}
}

func TestValidateMissing(t *testing.T) {
	resolver := Resolver{}

	var err error

	paramsMissingName := map[string]string{
		hubresolver.ParamKind:    "foo",
		hubresolver.ParamVersion: "bar",
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(paramsMissingName),
	}
	err = resolver.Validate(contextWithConfig(), &req)
	if err == nil {
		t.Fatalf("expected missing name err")
	}

	paramsMissingVersion := map[string]string{
		hubresolver.ParamKind: "foo",
		hubresolver.ParamName: "bar",
	}
	req = v1beta1.ResolutionRequestSpec{
		Params: toParams(paramsMissingVersion),
	}
	err = resolver.Validate(contextWithConfig(), &req)

	if err == nil {
		t.Fatalf("expected missing version err")
	}
}

func TestValidateConflictingKindName(t *testing.T) {
	testCases := []struct {
		kind    string
		name    string
		version string
		catalog string
		hubType string
	}{
		{
			kind:    "not-taskpipelineorstepaction",
			name:    "foo",
			version: "bar",
			catalog: "baz",
			hubType: TektonHubType,
		},
		{
			kind:    "task",
			name:    "foo",
			version: "bar",
			catalog: "baz",
			hubType: "not-tekton-artifact",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := Resolver{}
			params := map[string]string{
				hubresolver.ParamKind:    tc.kind,
				hubresolver.ParamName:    tc.name,
				hubresolver.ParamVersion: tc.version,
				hubresolver.ParamCatalog: tc.catalog,
				hubresolver.ParamType:    tc.hubType,
			}
			req := v1beta1.ResolutionRequestSpec{
				Params: toParams(params),
			}
			err := resolver.Validate(contextWithConfig(), &req)
			if err == nil {
				t.Fatalf("expected err due to conflicting param")
			}
		})
	}
}

func TestResolve(t *testing.T) {
	testCases := []struct {
		name        string
		kind        string
		imageName   string
		version     string
		catalog     string
		hubType     string
		input       string
		expectedRes []byte
		expectedErr error
	}{
		{
			name:        "valid response from Tekton Hub",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "Tekton",
			hubType:     TektonHubType,
			input:       `{"data":{"yaml":"some content"}}`,
			expectedRes: []byte("some content"),
		},
		{
			name:        "valid response from Artifact Hub",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "Tekton",
			hubType:     ArtifactHubType,
			input:       `{"data":{"manifestRaw":"some content"}}`,
			expectedRes: []byte("some content"),
		},
		{
			name:        "not-found response from hub",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "Tekton",
			hubType:     TektonHubType,
			input:       `{"name":"not-found","id":"aaaaaaaa","message":"resource not found","temporary":false,"timeout":false,"fault":false}`,
			expectedRes: []byte(""),
		},
		{
			name:        "response with bad formatting error",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "Tekton",
			hubType:     TektonHubType,
			input:       `value`,
			expectedErr: errors.New("fail to fetch Tekton Hub resource: error unmarshalling json response: invalid character 'v' looking for beginning of value"),
		},
		{
			name:        "response with empty body error from Tekton Hub",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "Tekton",
			hubType:     TektonHubType,
			expectedErr: errors.New("fail to fetch Tekton Hub resource: error unmarshalling json response: unexpected end of JSON input"),
		},
		{
			name:        "response with empty body error from Artifact Hub",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "Tekton",
			hubType:     ArtifactHubType,
			expectedErr: errors.New("fail to fetch Artifact Hub resource: error unmarshalling json response: unexpected end of JSON input"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tc.input)
			}))

			resolver := &Resolver{
				TektonHubURL:   svr.URL,
				ArtifactHubURL: svr.URL,
			}

			params := map[string]string{
				hubresolver.ParamKind:    tc.kind,
				hubresolver.ParamName:    tc.imageName,
				hubresolver.ParamVersion: tc.version,
				hubresolver.ParamCatalog: tc.catalog,
				hubresolver.ParamType:    tc.hubType,
			}
			req := v1beta1.ResolutionRequestSpec{
				Params: toParams(params),
			}
			output, err := resolver.Resolve(contextWithConfig(), &req)
			if tc.expectedErr != nil {
				checkExpectedErr(t, tc.expectedErr, err)
			} else {
				if err != nil {
					t.Fatalf("unexpected error resolving: %v", err)
				}
				if d := cmp.Diff(tc.expectedRes, output.Data()); d != "" {
					t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func toParams(m map[string]string) []pipelinev1.Param {
	var params []pipelinev1.Param

	for k, v := range m {
		params = append(params, pipelinev1.Param{
			Name:  k,
			Value: *pipelinev1.NewStructuredValues(v),
		})
	}

	return params
}

func contextWithConfig() context.Context {
	config := map[string]string{
		"default-tekton-hub-catalog":            "Tekton",
		"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
		"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
		"default-type":                          "artifact",
	}

	return resolutionframework.InjectResolverConfigToContext(context.Background(), config)
}

func checkExpectedErr(t *testing.T, expectedErr, actualErr error) {
	t.Helper()
	if actualErr == nil {
		t.Fatalf("expected err '%v' but didn't get one", expectedErr)
	}
	if d := cmp.Diff(expectedErr.Error(), actualErr.Error()); d != "" {
		t.Fatalf("expected err '%v' but got '%v'", expectedErr, actualErr)
	}
}
