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

package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	frtesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(context.Background())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != LabelValueHubResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
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
		}, {
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
				ParamKind:    tc.kind,
				ParamName:    tc.resourceName,
				ParamVersion: tc.version,
				ParamCatalog: tc.catalog,
				ParamType:    tc.hubType,
			}

			err := resolver.ValidateParams(contextWithConfig(), toParams(params))
			if tc.expectedErr != nil {
				checkExpectedErr(t, tc.expectedErr, err)
			} else if err != nil {
				t.Fatalf("unexpected error validating params: %v", err)
			}
		})
	}
}

func TestValidateParamsDisabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	params := map[string]string{
		ParamKind:    "task",
		ParamName:    "foo",
		ParamVersion: "bar",
		ParamCatalog: "baz",
	}
	err = resolver.ValidateParams(resolverDisabledContext(), toParams(params))
	if err == nil {
		t.Fatalf("expected missing name err")
	}

	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

func TestValidateParamsMissing(t *testing.T) {
	resolver := Resolver{}

	var err error

	paramsMissingName := map[string]string{
		ParamKind:    "foo",
		ParamVersion: "bar",
	}
	err = resolver.ValidateParams(contextWithConfig(), toParams(paramsMissingName))
	if err == nil {
		t.Fatalf("expected missing name err")
	}

	paramsMissingVersion := map[string]string{
		ParamKind: "foo",
		ParamName: "bar",
	}
	err = resolver.ValidateParams(contextWithConfig(), toParams(paramsMissingVersion))
	if err == nil {
		t.Fatalf("expected missing version err")
	}
}

func TestValidateParamsConflictingKindName(t *testing.T) {
	testCases := []struct {
		kind    string
		name    string
		version string
		catalog string
		hubType string
	}{
		{
			kind:    "not-taskpipeline",
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
				ParamKind:    tc.kind,
				ParamName:    tc.name,
				ParamVersion: tc.version,
				ParamCatalog: tc.catalog,
				ParamType:    tc.hubType,
			}
			err := resolver.ValidateParams(contextWithConfig(), toParams(params))
			if err == nil {
				t.Fatalf("expected err due to conflicting param")
			}
		})
	}
}

func TestResolveConstraint(t *testing.T) {
	tests := []struct {
		name                string
		wantErr             bool
		kind                string
		version             string
		catalog             string
		taskName            string
		hubType             string
		resultTask          any
		resultList          any
		expectedRes         string
		expectedTaskVersion string
		expectedErr         error
	}{
		{
			name:        "good/tekton hub/versions constraints",
			kind:        "task",
			version:     ">= 0.1",
			catalog:     "Tekton",
			taskName:    "something",
			hubType:     TektonHubType,
			expectedRes: "some content",
			resultTask: &tektonHubResponse{
				Data: tektonHubDataResponse{
					YAML: "some content",
				},
			},
			resultList: &tektonHubListResult{
				Data: tektonHubListDataResult{
					Versions: []tektonHubListResultVersion{
						{
							Version: "0.1",
						},
					},
				},
			},
		}, {
			name:        "good/tekton hub/only the greatest of the constraint",
			kind:        "task",
			version:     ">= 0.1",
			catalog:     "Tekton",
			taskName:    "something",
			hubType:     TektonHubType,
			expectedRes: "some content",
			resultTask: &tektonHubResponse{
				Data: tektonHubDataResponse{
					YAML: "some content",
				},
			},
			resultList: &tektonHubListResult{
				Data: tektonHubListDataResult{
					Versions: []tektonHubListResultVersion{
						{
							Version: "0.1",
						},
						{
							Version: "0.2",
						},
					},
				},
			},
			expectedTaskVersion: "0.2",
		}, {
			name:        "good/artifact hub/only the greatest of the constraint",
			kind:        "task",
			version:     ">= 0.1",
			catalog:     "Tekton",
			taskName:    "something",
			hubType:     ArtifactHubType,
			expectedRes: "some content",
			resultTask: &artifactHubResponse{
				Data: artifactHubDataResponse{
					YAML: "some content",
				},
			},
			resultList: &artifactHubListResult{
				AvailableVersions: []artifactHubavailableVersionsResults{
					{
						Version: "0.1.0",
					},
					{
						Version: "0.2.0",
					},
				},
			},
			expectedTaskVersion: "0.2.0",
		}, {
			name:        "good/artifact hub/versions constraints",
			kind:        "task",
			version:     ">= 0.1.0",
			catalog:     "Tekton",
			taskName:    "something",
			hubType:     ArtifactHubType,
			expectedRes: "some content",
			resultTask: &artifactHubResponse{
				Data: artifactHubDataResponse{
					YAML: "some content",
				},
			},
			resultList: &artifactHubListResult{
				AvailableVersions: []artifactHubavailableVersionsResults{
					{
						Version: "0.1.0",
					},
				},
			},
		}, {
			name:     "bad/artifact hub/no matching constraints",
			kind:     "task",
			version:  ">= 0.2.0",
			catalog:  "Tekton",
			taskName: "something",
			hubType:  ArtifactHubType,
			resultList: &artifactHubListResult{
				AvailableVersions: []artifactHubavailableVersionsResults{
					{
						Version: "0.1.0",
					},
				},
			},
			expectedErr: errors.New("no version found for constraint >= 0.2.0"),
		}, {
			name:     "bad/tekton hub/no matching constraints",
			kind:     "task",
			version:  ">= 0.2.0",
			catalog:  "Tekton",
			taskName: "something",
			hubType:  ArtifactHubType,
			resultList: &tektonHubListResult{
				Data: tektonHubListDataResult{
					Versions: []tektonHubListResultVersion{
						{
							Version: "0.1",
						},
					},
				},
			},
			expectedErr: errors.New("no version found for constraint >= 0.2.0"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ret any
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				listURL := fmt.Sprintf(ArtifactHubListTasksEndpoint, tt.kind, tt.catalog, tt.taskName)
				if tt.hubType == TektonHubType {
					listURL = fmt.Sprintf(TektonHubListTasksEndpoint, tt.catalog, tt.kind, tt.taskName)
				}
				if r.URL.Path == "/"+listURL {
					// encore result list as json
					ret = tt.resultList
				} else {
					if tt.expectedTaskVersion != "" {
						version := filepath.Base(r.URL.Path)
						if tt.hubType == TektonHubType {
							version = strings.Split(r.URL.Path, "/")[6]
						}
						if tt.expectedTaskVersion != version {
							t.Fatalf("unexpected version: %s wanted: %s", version, tt.expectedTaskVersion)
						}
					}
					ret = tt.resultTask
				}
				output, _ := json.Marshal(ret)
				fmt.Fprintf(w, string(output))
			}))

			resolver := &Resolver{
				TektonHubURL:   svr.URL,
				ArtifactHubURL: svr.URL,
			}
			params := map[string]string{
				ParamKind:    tt.kind,
				ParamName:    tt.taskName,
				ParamVersion: tt.version,
				ParamCatalog: tt.catalog,
				ParamType:    tt.hubType,
			}
			output, err := resolver.Resolve(contextWithConfig(), toParams(params))
			if tt.expectedErr != nil {
				checkExpectedErr(t, tt.expectedErr, err)
			} else {
				if err != nil {
					t.Fatalf("unexpected error resolving: %v", err)
				}
				if d := cmp.Diff(tt.expectedRes, string(output.Data())); d != "" {
					t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestResolveVersion(t *testing.T) {
	testCases := []struct {
		name        string
		version     string
		hubType     string
		expectedVer string
		expectedErr error
	}{
		{
			name:        "semver to Tekton Hub",
			version:     "0.6.0",
			hubType:     TektonHubType,
			expectedVer: "0.6",
		},
		{
			name:        "simplified semver to Tekton Hub",
			version:     "0.6",
			hubType:     TektonHubType,
			expectedVer: "0.6",
		},
		{
			name:        "semver to Artifact Hub",
			version:     "0.6.0",
			hubType:     ArtifactHubType,
			expectedVer: "0.6.0",
		},
		{
			name:        "simplified semver to Artifact Hub",
			version:     "0.6",
			hubType:     ArtifactHubType,
			expectedVer: "0.6.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resVer, err := resolveVersion(tc.version, tc.hubType)
			if tc.expectedErr != nil {
				checkExpectedErr(t, tc.expectedErr, err)
			} else {
				if err != nil {
					t.Fatalf("unexpected error resolving, %v", err)
				} else {
					if d := cmp.Diff(tc.expectedVer, resVer); d != "" {
						t.Fatalf("expected version '%v' but got '%v'", tc.expectedVer, resVer)
					}
				}
			}
		})
	}
}

func TestResolveCatalogName(t *testing.T) {
	testCases := []struct {
		name        string
		inputCat    string
		kind        string
		hubType     string
		expectedCat string
	}{
		{
			name:        "tekton type default catalog",
			kind:        "task",
			hubType:     "tekton",
			expectedCat: "Tekton",
		},
		{
			name:        "artifact type default task catalog",
			kind:        "task",
			hubType:     "artifact",
			expectedCat: "tekton-catalog-tasks",
		},
		{
			name:        "artifact type default pipeline catalog",
			kind:        "pipeline",
			hubType:     "artifact",
			expectedCat: "tekton-catalog-pipelines",
		},
		{
			name:        "custom catalog",
			inputCat:    "custom-catalog",
			kind:        "task",
			hubType:     "artifact",
			expectedCat: "custom-catalog",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]string{
				ParamKind: tc.kind,
				ParamType: tc.hubType,
			}
			if tc.inputCat != "" {
				params[ParamCatalog] = tc.inputCat
			}

			conf := framework.GetResolverConfigFromContext(contextWithConfig())

			resCatalog, err := resolveCatalogName(params, conf)
			if err != nil {
				t.Fatalf("unexpected error resolving, %v", err)
			} else {
				if d := cmp.Diff(tc.expectedCat, resCatalog); d != "" {
					t.Fatalf("expected catalog name '%v' but got '%v'", tc.expectedCat, resCatalog)
				}
			}
		})
	}
}

func TestResolveDisabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	params := map[string]string{
		ParamKind:    "task",
		ParamName:    "foo",
		ParamVersion: "bar",
		ParamCatalog: "baz",
	}
	_, err = resolver.Resolve(resolverDisabledContext(), toParams(params))
	if err == nil {
		t.Fatalf("expected missing name err")
	}

	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
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
				fmt.Fprintf(w, tc.input)
			}))

			resolver := &Resolver{
				TektonHubURL:   svr.URL,
				ArtifactHubURL: svr.URL,
			}

			params := map[string]string{
				ParamKind:    tc.kind,
				ParamName:    tc.imageName,
				ParamVersion: tc.version,
				ParamCatalog: tc.catalog,
				ParamType:    tc.hubType,
			}

			output, err := resolver.Resolve(contextWithConfig(), toParams(params))
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

func resolverDisabledContext() context.Context {
	return frtesting.ContextWithHubResolverDisabled(context.Background())
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

	return framework.InjectResolverConfigToContext(context.Background(), config)
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
