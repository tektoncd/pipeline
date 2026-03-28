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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// Multi-URL tests: per-resolution URL param override

func TestValidateURLParam(t *testing.T) {
	testCases := []struct {
		testName     string
		kind         string
		version      string
		catalog      string
		resourceName string
		hubType      string
		hubURL       string
		expectedErr  error
	}{
		{
			testName:     "tekton type with url override bypasses env var check",
			kind:         "task",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      TektonHubType,
			hubURL:       "https://custom-tekton-hub.example.com",
		},
		{
			testName:     "artifact type with url override",
			kind:         "task",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      ArtifactHubType,
			hubURL:       "https://internal-hub.example.com",
		},
		{
			testName:     "invalid url param",
			kind:         "task",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      ArtifactHubType,
			hubURL:       "not-a-url",
			expectedErr:  errors.New("failed to validate params: invalid url param: url must be a valid absolute URL: not-a-url"),
		},
		{
			testName:     "file scheme rejected",
			kind:         "task",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      ArtifactHubType,
			hubURL:       "file:///etc/passwd",
			expectedErr:  errors.New("failed to validate params: invalid url param: url must be a valid absolute URL: file:///etc/passwd"),
		},
		{
			testName:     "ftp scheme rejected",
			kind:         "task",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      ArtifactHubType,
			hubURL:       "ftp://example.com",
			expectedErr:  errors.New("failed to validate params: invalid url param: url must use http or https scheme: ftp://example.com"),
		},
		{
			testName:     "gopher scheme rejected",
			kind:         "task",
			resourceName: "foo",
			version:      "bar",
			catalog:      "baz",
			hubType:      ArtifactHubType,
			hubURL:       "gopher://example.com",
			expectedErr:  errors.New("failed to validate params: invalid url param: url must use http or https scheme: gopher://example.com"),
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
			if tc.hubURL != "" {
				params[ParamURL] = tc.hubURL
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

func TestResolveURLParamOverride(t *testing.T) {
	testCases := []struct {
		name         string
		kind         string
		imageName    string
		version      string
		catalog      string
		hubType      string
		input        string
		expectedRes  []byte
		expectedPath string
	}{
		{
			name:         "url param overrides artifact hub default",
			kind:         "task",
			imageName:    "foo",
			version:      "baz",
			catalog:      "Tekton",
			hubType:      ArtifactHubType,
			input:        `{"data":{"manifestRaw":"from custom hub"}}`,
			expectedRes:  []byte("from custom hub"),
			expectedPath: "/" + fmt.Sprintf(hubresolver.ArtifactHubYamlEndpoint, "task", "Tekton", "foo", "baz"),
		},
		{
			name:         "url param overrides tekton hub default",
			kind:         "task",
			imageName:    "foo",
			version:      "baz",
			catalog:      "Tekton",
			hubType:      TektonHubType,
			input:        `{"data":{"yaml":"from custom tekton hub"}}`,
			expectedRes:  []byte("from custom tekton hub"),
			expectedPath: "/" + fmt.Sprintf(hubresolver.TektonHubYamlEndpoint, "Tekton", "task", "foo", "baz"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			overrideSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.expectedPath {
					t.Errorf("expected request path %s but got %s", tc.expectedPath, r.URL.Path)
				}
				fmt.Fprint(w, tc.input)
			}))
			defer overrideSvr.Close()

			defaultSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("default server should not be called when url param is set")
			}))
			defer defaultSvr.Close()

			resolver := &Resolver{
				TektonHubURL:   defaultSvr.URL,
				ArtifactHubURL: defaultSvr.URL,
			}

			params := map[string]string{
				hubresolver.ParamKind:    tc.kind,
				hubresolver.ParamName:    tc.imageName,
				hubresolver.ParamVersion: tc.version,
				hubresolver.ParamCatalog: tc.catalog,
				hubresolver.ParamType:    tc.hubType,
				ParamURL:                 overrideSvr.URL,
			}
			req := v1beta1.ResolutionRequestSpec{
				Params: toParams(params),
			}

			output, err := resolver.Resolve(contextWithConfig(), &req)
			if err != nil {
				t.Fatalf("unexpected error resolving: %v", err)
			}
			if d := cmp.Diff(tc.expectedRes, output.Data()); d != "" {
				t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
			}

			if output.RefSource() == nil {
				t.Fatal("expected non-nil RefSource")
			}
			if !strings.HasPrefix(output.RefSource().URI, overrideSvr.URL) {
				t.Errorf("expected RefSource URI to start with override URL %s but got %s", overrideSvr.URL, output.RefSource().URI)
			}
		})
	}
}

func TestResolveURLParamWithNoDefaultTektonHub(t *testing.T) {
	overrideSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"yaml":"from override"}}`)
	}))
	defer overrideSvr.Close()

	resolver := &Resolver{
		TektonHubURL:   "",
		ArtifactHubURL: "https://artifacthub.io",
	}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "foo",
		hubresolver.ParamVersion: "baz",
		hubresolver.ParamCatalog: "Tekton",
		hubresolver.ParamType:    TektonHubType,
		ParamURL:                 overrideSvr.URL,
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	output, err := resolver.Resolve(contextWithConfig(), &req)
	if err != nil {
		t.Fatalf("unexpected error: should resolve with url param even when TEKTON_HUB_API is empty: %v", err)
	}
	if d := cmp.Diff([]byte("from override"), output.Data()); d != "" {
		t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
	}
}

func TestResolveURLParamWithVersionConstraint(t *testing.T) {
	testCases := []struct {
		name        string
		kind        string
		taskName    string
		version     string
		catalog     string
		hubType     string
		resultList  any
		resultTask  any
		expectedRes string
	}{
		{
			name:     "artifact hub version constraint with URL override",
			kind:     "task",
			taskName: "something",
			version:  ">= 0.1",
			catalog:  "Tekton",
			hubType:  ArtifactHubType,
			resultList: &artifactHubListResult{
				AvailableVersions: []artifactHubavailableVersionsResults{
					{Version: "0.1.0"},
					{Version: "0.2.0"},
				},
			},
			resultTask: &artifactHubResponse{
				Data: artifactHubDataResponse{YAML: "from override"},
			},
			expectedRes: "from override",
		},
		{
			name:     "tekton hub version constraint with URL override",
			kind:     "task",
			taskName: "something",
			version:  ">= 0.1",
			catalog:  "Tekton",
			hubType:  TektonHubType,
			resultList: &tektonHubListResult{
				Data: tektonHubListDataResult{
					Versions: []tektonHubListResultVersion{
						{Version: "0.1"},
						{Version: "0.2"},
					},
				},
			},
			resultTask: &tektonHubResponse{
				Data: tektonHubDataResponse{YAML: "from override"},
			},
			expectedRes: "from override",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			overrideSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var ret any
				listURL := fmt.Sprintf(hubresolver.ArtifactHubListTasksEndpoint, tc.kind, tc.catalog, tc.taskName)
				if tc.hubType == TektonHubType {
					listURL = fmt.Sprintf(hubresolver.TektonHubListTasksEndpoint, tc.catalog, tc.kind, tc.taskName)
				}
				if r.URL.Path == "/"+listURL {
					ret = tc.resultList
				} else {
					ret = tc.resultTask
				}
				output, _ := json.Marshal(ret)
				fmt.Fprint(w, string(output))
			}))
			defer overrideSvr.Close()

			defaultSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("default server should not be called when url param is set")
			}))
			defer defaultSvr.Close()

			resolver := &Resolver{
				TektonHubURL:   defaultSvr.URL,
				ArtifactHubURL: defaultSvr.URL,
			}

			params := map[string]string{
				hubresolver.ParamKind:    tc.kind,
				hubresolver.ParamName:    tc.taskName,
				hubresolver.ParamVersion: tc.version,
				hubresolver.ParamCatalog: tc.catalog,
				hubresolver.ParamType:    tc.hubType,
				ParamURL:                 overrideSvr.URL,
			}
			req := v1beta1.ResolutionRequestSpec{
				Params: toParams(params),
			}

			output, err := resolver.Resolve(contextWithConfig(), &req)
			if err != nil {
				t.Fatalf("unexpected error resolving: %v", err)
			}
			if d := cmp.Diff(tc.expectedRes, string(output.Data())); d != "" {
				t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolveURLParamTrailingSlash(t *testing.T) {
	overrideSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "//") {
			t.Errorf("request path contains double slash: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":{"manifestRaw":"from override"}}`)
	}))
	defer overrideSvr.Close()

	resolver := &Resolver{
		TektonHubURL:   "https://should-not-be-used.example.com",
		ArtifactHubURL: "https://should-not-be-used.example.com",
	}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "foo",
		hubresolver.ParamVersion: "baz",
		hubresolver.ParamCatalog: "Tekton",
		hubresolver.ParamType:    ArtifactHubType,
		ParamURL:                 overrideSvr.URL + "/",
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	output, err := resolver.Resolve(contextWithConfig(), &req)
	if err != nil {
		t.Fatalf("unexpected error resolving: %v", err)
	}
	if d := cmp.Diff([]byte("from override"), output.Data()); d != "" {
		t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
	}
}

// Multi-URL tests: ConfigMap URL list and fallback

func TestParseURLList(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    []string
		expectedErr bool
	}{
		{name: "single URL", input: "- https://hub.example.com", expected: []string{"https://hub.example.com"}},
		{name: "multiple URLs", input: "- https://internal-hub.example.com/\n- https://artifacthub.io/", expected: []string{"https://internal-hub.example.com", "https://artifacthub.io"}},
		{name: "trailing slashes removed", input: "- https://hub.example.com///", expected: []string{"https://hub.example.com"}},
		{name: "empty string", input: "", expected: nil},
		{name: "whitespace only", input: "   \n  ", expected: nil},
		{name: "invalid YAML", input: "not: a: list: {[}", expectedErr: true},
		{name: "ftp URL rejected", input: "- ftp://example.com", expectedErr: true},
		{name: "non-URL string rejected", input: "- not-a-url", expectedErr: true},
		{name: "mixed valid and invalid URLs rejected", input: "- https://good.example.com\n- ftp://bad.example.com", expectedErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseURLList(tc.input)
			if tc.expectedErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d := cmp.Diff(tc.expected, result); d != "" {
				t.Errorf("unexpected result: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidateHubURL(t *testing.T) {
	testCases := []struct {
		name      string
		url       string
		expectErr bool
	}{
		{name: "valid http URL", url: "http://example.com"},
		{name: "valid https URL", url: "https://example.com"},
		{name: "valid https URL with path", url: "https://hub.example.com/api/v1"},
		{name: "ftp scheme rejected", url: "ftp://example.com", expectErr: true},
		{name: "gopher scheme rejected", url: "gopher://example.com", expectErr: true},
		{name: "missing host", url: "https://", expectErr: true},
		{name: "malformed URL", url: "not-a-url", expectErr: true},
		{name: "file scheme rejected", url: "file:///etc/passwd", expectErr: true},
		{name: "empty string", url: "", expectErr: true},
		{name: "URL with port", url: "https://example.com:8080"},
		{name: "URL with query params", url: "https://example.com/api/v1?key=val"},
		{name: "data URL rejected", url: "data:text/html,<h1>hi</h1>", expectErr: true},
		{name: "javascript URL rejected", url: "javascript:alert(1)", expectErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHubURL(tc.url)
			if tc.expectErr && err == nil {
				t.Errorf("expected error for URL %q but got nil", tc.url)
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error for URL %q: %v", tc.url, err)
			}
		})
	}
}

func TestResolveHubURLs(t *testing.T) {
	testCases := []struct {
		name      string
		conf      map[string]string
		envVarURL string
		configKey string
		expected  []string
	}{
		{
			name:      "ConfigMap URLs take precedence over env var",
			conf:      map[string]string{ConfigArtifactHubURLs: "- https://internal.example.com\n- https://artifacthub.io"},
			envVarURL: "https://env-var.example.com",
			configKey: ConfigArtifactHubURLs,
			expected:  []string{"https://internal.example.com", "https://artifacthub.io"},
		},
		{
			name:      "falls back to env var when ConfigMap key absent",
			conf:      map[string]string{},
			envVarURL: "https://env-var.example.com",
			configKey: ConfigArtifactHubURLs,
			expected:  []string{"https://env-var.example.com"},
		},
		{
			name:      "falls back to env var when ConfigMap key empty",
			conf:      map[string]string{ConfigArtifactHubURLs: ""},
			envVarURL: "https://env-var.example.com",
			configKey: ConfigArtifactHubURLs,
			expected:  []string{"https://env-var.example.com"},
		},
		{
			name:      "returns nil when no URL configured",
			conf:      map[string]string{},
			envVarURL: "",
			configKey: ConfigTektonHubURLs,
			expected:  nil,
		},
		{
			name:      "falls back to env var on invalid YAML",
			conf:      map[string]string{ConfigTektonHubURLs: "not: valid: yaml: {[}"},
			envVarURL: "https://env-var.example.com",
			configKey: ConfigTektonHubURLs,
			expected:  []string{"https://env-var.example.com"},
		},
		{
			name:      "falls back to env var on invalid URL scheme in ConfigMap",
			conf:      map[string]string{ConfigArtifactHubURLs: "- ftp://example.com"},
			envVarURL: "https://env-var.example.com",
			configKey: ConfigArtifactHubURLs,
			expected:  []string{"https://env-var.example.com"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result := resolveHubURLs(ctx, tc.conf, tc.envVarURL, tc.configKey)
			if d := cmp.Diff(tc.expected, result); d != "" {
				t.Errorf("unexpected result: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolveFallback(t *testing.T) {
	testCases := []struct {
		name        string
		hubType     string
		server1Code int
		server1Body string
		server2Body string
		expectedRes string
	}{
		{
			name:        "first server fails, second succeeds (artifact)",
			hubType:     ArtifactHubType,
			server1Code: http.StatusNotFound,
			server2Body: `{"data":{"manifestRaw":"from second"}}`,
			expectedRes: "from second",
		},
		{
			name:        "first server succeeds, second not contacted (artifact)",
			hubType:     ArtifactHubType,
			server1Code: http.StatusOK,
			server1Body: `{"data":{"manifestRaw":"from first"}}`,
			server2Body: `{"data":{"manifestRaw":"from second"}}`,
			expectedRes: "from first",
		},
		{
			name:        "first server fails, second succeeds (tekton)",
			hubType:     TektonHubType,
			server1Code: http.StatusNotFound,
			server2Body: `{"data":{"yaml":"from second"}}`,
			expectedRes: "from second",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svr1Called := false
			svr1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				svr1Called = true
				if tc.server1Code == http.StatusNotFound {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				fmt.Fprint(w, tc.server1Body)
			}))
			defer svr1.Close()

			svr2Called := false
			svr2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				svr2Called = true
				fmt.Fprint(w, tc.server2Body)
			}))
			defer svr2.Close()

			urlYAML := fmt.Sprintf("- %s\n- %s", svr1.URL, svr2.URL)
			configKey := ConfigArtifactHubURLs
			if tc.hubType == TektonHubType {
				configKey = ConfigTektonHubURLs
			}
			config := map[string]string{
				"default-tekton-hub-catalog":            "Tekton",
				"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
				"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
				"default-type":                          "artifact",
				configKey:                               urlYAML,
			}
			ctx := resolutionframework.InjectResolverConfigToContext(context.Background(), config)

			resolver := &Resolver{}

			params := map[string]string{
				hubresolver.ParamKind:    "task",
				hubresolver.ParamName:    "foo",
				hubresolver.ParamVersion: "baz",
				hubresolver.ParamCatalog: "Tekton",
				hubresolver.ParamType:    tc.hubType,
			}
			req := v1beta1.ResolutionRequestSpec{
				Params: toParams(params),
			}

			output, err := resolver.Resolve(ctx, &req)
			if err != nil {
				t.Fatalf("unexpected error resolving: %v", err)
			}

			if !svr1Called {
				t.Error("expected first server to be called")
			}
			if tc.server1Code == http.StatusNotFound && !svr2Called {
				t.Error("expected second server to be called when first fails")
			}
			if tc.server1Code == http.StatusOK && svr2Called {
				t.Error("second server should not be called when first succeeds")
			}

			if d := cmp.Diff(tc.expectedRes, string(output.Data())); d != "" {
				t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolveFallbackAllFail(t *testing.T) {
	svr1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer svr1.Close()

	svr2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer svr2.Close()

	urlYAML := fmt.Sprintf("- %s\n- %s", svr1.URL, svr2.URL)
	config := map[string]string{
		"default-tekton-hub-catalog":            "Tekton",
		"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
		"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
		"default-type":                          "artifact",
		ConfigArtifactHubURLs:                   urlYAML,
	}
	ctx := resolutionframework.InjectResolverConfigToContext(context.Background(), config)

	resolver := &Resolver{}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "foo",
		hubresolver.ParamVersion: "baz",
		hubresolver.ParamCatalog: "Tekton",
		hubresolver.ParamType:    ArtifactHubType,
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	_, err := resolver.Resolve(ctx, &req)
	if err == nil {
		t.Fatal("expected error when all hub URLs fail")
	}
	if !strings.Contains(err.Error(), "failed to fetch resource from any configured hub URL") {
		t.Errorf("expected aggregated error message, got: %v", err)
	}
}

func TestResolveFallbackVersionConstraintNoVersionStopsFallback(t *testing.T) {
	svr1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		listURL := fmt.Sprintf(hubresolver.ArtifactHubListTasksEndpoint, "task", "Tekton", "something")
		if r.URL.Path == "/"+listURL {
			resp := artifactHubListResult{
				AvailableVersions: []artifactHubavailableVersionsResults{
					{Version: "0.1.0"},
				},
			}
			output, _ := json.Marshal(resp)
			fmt.Fprint(w, string(output))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer svr1.Close()

	svr2Called := false
	svr2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svr2Called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer svr2.Close()

	urlYAML := fmt.Sprintf("- %s\n- %s", svr1.URL, svr2.URL)
	config := map[string]string{
		"default-tekton-hub-catalog":            "Tekton",
		"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
		"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
		"default-type":                          "artifact",
		ConfigArtifactHubURLs:                   urlYAML,
	}
	ctx := resolutionframework.InjectResolverConfigToContext(context.Background(), config)

	resolver := &Resolver{}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "something",
		hubresolver.ParamVersion: ">= 0.2.0",
		hubresolver.ParamCatalog: "Tekton",
		hubresolver.ParamType:    ArtifactHubType,
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	_, err := resolver.Resolve(ctx, &req)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "no version found") {
		t.Errorf("expected 'no version found' error, got: %v", err)
	}
	if svr2Called {
		t.Error("second hub should not have been contacted when first hub had no matching version")
	}
}

func TestResolveFallbackVersionConstraintUnreachableFallsThrough(t *testing.T) {
	svr1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer svr1.Close()

	svr2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		listURL := fmt.Sprintf(hubresolver.ArtifactHubListTasksEndpoint, "task", "Tekton", "something")
		if r.URL.Path == "/"+listURL {
			resp := artifactHubListResult{
				AvailableVersions: []artifactHubavailableVersionsResults{
					{Version: "0.1.0"},
					{Version: "0.2.0"},
				},
			}
			output, _ := json.Marshal(resp)
			fmt.Fprint(w, string(output))
		} else {
			resp := artifactHubResponse{
				Data: artifactHubDataResponse{YAML: "from second hub"},
			}
			output, _ := json.Marshal(resp)
			fmt.Fprint(w, string(output))
		}
	}))
	defer svr2.Close()

	urlYAML := fmt.Sprintf("- %s\n- %s", svr1.URL, svr2.URL)
	config := map[string]string{
		"default-tekton-hub-catalog":            "Tekton",
		"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
		"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
		"default-type":                          "artifact",
		ConfigArtifactHubURLs:                   urlYAML,
	}
	ctx := resolutionframework.InjectResolverConfigToContext(context.Background(), config)

	resolver := &Resolver{}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "something",
		hubresolver.ParamVersion: ">= 0.2.0",
		hubresolver.ParamCatalog: "Tekton",
		hubresolver.ParamType:    ArtifactHubType,
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	output, err := resolver.Resolve(ctx, &req)
	if err != nil {
		t.Fatalf("unexpected error resolving: %v", err)
	}
	if d := cmp.Diff("from second hub", string(output.Data())); d != "" {
		t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
	}
}

func TestResolveWithConfigMapURLs(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"manifestRaw":"from configmap url"}}`)
	}))
	defer svr.Close()

	config := map[string]string{
		"default-tekton-hub-catalog":            "Tekton",
		"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
		"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
		"default-type":                          "artifact",
		ConfigArtifactHubURLs:                   "- " + svr.URL,
	}
	ctx := resolutionframework.InjectResolverConfigToContext(context.Background(), config)

	resolver := &Resolver{
		ArtifactHubURL: "https://should-not-be-used.example.com",
	}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "foo",
		hubresolver.ParamVersion: "baz",
		hubresolver.ParamCatalog: "Tekton",
		hubresolver.ParamType:    ArtifactHubType,
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	output, err := resolver.Resolve(ctx, &req)
	if err != nil {
		t.Fatalf("unexpected error resolving: %v", err)
	}
	if d := cmp.Diff([]byte("from configmap url"), output.Data()); d != "" {
		t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
	}
}

func TestResolveSingleConfigMapURLErrorFormat(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer svr.Close()

	config := map[string]string{
		"default-tekton-hub-catalog":            "Tekton",
		"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
		"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
		"default-type":                          "artifact",
		ConfigArtifactHubURLs:                   "- " + svr.URL,
	}
	ctx := resolutionframework.InjectResolverConfigToContext(context.Background(), config)

	resolver := &Resolver{}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "foo",
		hubresolver.ParamVersion: "baz",
		hubresolver.ParamCatalog: "Tekton",
		hubresolver.ParamType:    ArtifactHubType,
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	_, err := resolver.Resolve(ctx, &req)
	if err == nil {
		t.Fatal("expected error when single ConfigMap URL fails")
	}
	if strings.Contains(err.Error(), "failed to fetch resource from any configured hub URL") {
		t.Errorf("single URL error should not be wrapped in aggregate message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "fail to fetch Artifact Hub resource") {
		t.Errorf("expected direct Artifact Hub error, got: %v", err)
	}
}

func TestResolveURLParamOverridesConfigMapURLs(t *testing.T) {
	configMapSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("ConfigMap URL server should not be called when url param is set")
	}))
	defer configMapSvr.Close()

	overrideSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"manifestRaw":"from url param"}}`)
	}))
	defer overrideSvr.Close()

	config := map[string]string{
		"default-tekton-hub-catalog":            "Tekton",
		"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
		"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
		"default-type":                          "artifact",
		ConfigArtifactHubURLs:                   "- " + configMapSvr.URL,
	}
	ctx := resolutionframework.InjectResolverConfigToContext(context.Background(), config)

	resolver := &Resolver{}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "foo",
		hubresolver.ParamVersion: "baz",
		hubresolver.ParamCatalog: "Tekton",
		hubresolver.ParamType:    ArtifactHubType,
		ParamURL:                 overrideSvr.URL,
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	output, err := resolver.Resolve(ctx, &req)
	if err != nil {
		t.Fatalf("unexpected error resolving: %v", err)
	}
	if d := cmp.Diff([]byte("from url param"), output.Data()); d != "" {
		t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
	}
}

func TestValidateParamsTektonTypeWithConfigMapURLs(t *testing.T) {
	config := map[string]string{
		"default-tekton-hub-catalog":            "Tekton",
		"default-artifact-hub-task-catalog":     "tekton-catalog-tasks",
		"default-artifact-hub-pipeline-catalog": "tekton-catalog-pipelines",
		"default-type":                          "artifact",
		ConfigTektonHubURLs:                     "- https://tekton-hub.example.com",
	}
	ctx := resolutionframework.InjectResolverConfigToContext(context.Background(), config)

	resolver := Resolver{TektonHubURL: ""}

	params := map[string]string{
		hubresolver.ParamKind:    "task",
		hubresolver.ParamName:    "foo",
		hubresolver.ParamVersion: "bar",
		hubresolver.ParamCatalog: "baz",
		hubresolver.ParamType:    TektonHubType,
	}
	req := v1beta1.ResolutionRequestSpec{
		Params: toParams(params),
	}

	err := resolver.Validate(ctx, &req)
	if err != nil {
		t.Fatalf("expected no error when ConfigMap has tekton-hub-urls, got: %v", err)
	}
}
