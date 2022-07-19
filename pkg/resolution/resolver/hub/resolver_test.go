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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	frtesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(resolverContext())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != LabelValueHubResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	resolver := Resolver{}

	paramsWithTask := map[string]v1beta1.ArrayOrString{
		ParamKind:    *v1beta1.NewArrayOrString("task"),
		ParamName:    *v1beta1.NewArrayOrString("foo"),
		ParamVersion: *v1beta1.NewArrayOrString("bar"),
		ParamCatalog: *v1beta1.NewArrayOrString("baz"),
	}
	if err := resolver.ValidateParams(resolverContext(), paramsWithTask); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}

	paramsWithPipeline := map[string]v1beta1.ArrayOrString{
		ParamKind:    *v1beta1.NewArrayOrString("pipeline"),
		ParamName:    *v1beta1.NewArrayOrString("foo"),
		ParamVersion: *v1beta1.NewArrayOrString("bar"),
		ParamCatalog: *v1beta1.NewArrayOrString("baz"),
	}
	if err := resolver.ValidateParams(resolverContext(), paramsWithPipeline); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}
}

func TestValidateParamsDisabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	params := map[string]v1beta1.ArrayOrString{
		ParamKind:    *v1beta1.NewArrayOrString("task"),
		ParamName:    *v1beta1.NewArrayOrString("foo"),
		ParamVersion: *v1beta1.NewArrayOrString("bar"),
		ParamCatalog: *v1beta1.NewArrayOrString("baz"),
	}
	err = resolver.ValidateParams(context.Background(), params)
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

	paramsMissingName := map[string]v1beta1.ArrayOrString{
		ParamKind:    *v1beta1.NewArrayOrString("foo"),
		ParamVersion: *v1beta1.NewArrayOrString("bar"),
	}
	err = resolver.ValidateParams(resolverContext(), paramsMissingName)
	if err == nil {
		t.Fatalf("expected missing name err")
	}

	paramsMissingVersion := map[string]v1beta1.ArrayOrString{
		ParamKind: *v1beta1.NewArrayOrString("foo"),
		ParamName: *v1beta1.NewArrayOrString("bar"),
	}
	err = resolver.ValidateParams(resolverContext(), paramsMissingVersion)
	if err == nil {
		t.Fatalf("expected missing version err")
	}
}

func TestValidateParamsConflictingKindName(t *testing.T) {
	resolver := Resolver{}
	params := map[string]v1beta1.ArrayOrString{
		ParamKind:    *v1beta1.NewArrayOrString("not-taskpipeline"),
		ParamName:    *v1beta1.NewArrayOrString("foo"),
		ParamVersion: *v1beta1.NewArrayOrString("bar"),
		ParamCatalog: *v1beta1.NewArrayOrString("baz"),
	}
	err := resolver.ValidateParams(resolverContext(), params)
	if err == nil {
		t.Fatalf("expected err due to conflicting kind param")
	}
}

func TestResolveDisabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	params := map[string]v1beta1.ArrayOrString{
		ParamKind:    *v1beta1.NewArrayOrString("task"),
		ParamName:    *v1beta1.NewArrayOrString("foo"),
		ParamVersion: *v1beta1.NewArrayOrString("bar"),
		ParamCatalog: *v1beta1.NewArrayOrString("baz"),
	}
	_, err = resolver.Resolve(context.Background(), params)
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
		input       string
		expectedRes []byte
		expectedErr error
	}{
		{
			name:        "valid response from hub",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "tekton",
			input:       `{"data":{"yaml":"some content"}}`,
			expectedRes: []byte("some content"),
		},
		{
			name:        "not-found response from hub",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "tekton",
			input:       `{"name":"not-found","id":"aaaaaaaa","message":"resource not found","temporary":false,"timeout":false,"fault":false}`,
			expectedRes: []byte(""),
		},
		{
			name:        "response with bad formatting error",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "tekton",
			input:       `value`,
			expectedErr: fmt.Errorf("error unmarshalling json response: invalid character 'v' looking for beginning of value"),
		},
		{
			name:        "response with empty body error",
			kind:        "task",
			imageName:   "foo",
			version:     "baz",
			catalog:     "tekton",
			expectedErr: fmt.Errorf("error unmarshalling json response: unexpected end of JSON input"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, tc.input)
			}))

			resolver := &Resolver{HubURL: svr.URL + "/" + YamlEndpoint}

			params := map[string]v1beta1.ArrayOrString{
				ParamKind:    *v1beta1.NewArrayOrString(tc.kind),
				ParamName:    *v1beta1.NewArrayOrString(tc.imageName),
				ParamVersion: *v1beta1.NewArrayOrString(tc.version),
				ParamCatalog: *v1beta1.NewArrayOrString(tc.catalog),
			}

			output, err := resolver.Resolve(resolverContext(), params)
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected err '%v' but didn't get one", tc.expectedErr)
				}
				if d := cmp.Diff(tc.expectedErr.Error(), err.Error()); d != "" {
					t.Fatalf("expected err '%v' but got '%v'", tc.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error resolving: %v", err)
				}

				expectedResource := &ResolvedHubResource{
					Content: tc.expectedRes,
				}

				if d := cmp.Diff(expectedResource, output); d != "" {
					t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func resolverContext() context.Context {
	return frtesting.ContextWithHubResolverEnabled(context.Background())
}
