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

package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	} else if typ != LabelValueHttpResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		expectedErr error
	}{
		{
			name: "valid/url",
			url:  "https://raw.githubusercontent.com/tektoncd/catalog/main/task/git-clone/0.4/git-clone.yaml",
		}, {
			name:        "invalid/url",
			url:         "xttps:ufoo/bar/",
			expectedErr: fmt.Errorf(`url xttps:ufoo/bar/ is not a valid http(s) url`),
		}, {
			name:        "invalid/url empty",
			url:         "",
			expectedErr: fmt.Errorf(`cannot parse url : parse "": empty url`),
		}, {
			name:        "missing/url",
			expectedErr: fmt.Errorf(`missing required http resolver params: url`),
			url:         "nourl",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := Resolver{}
			params := map[string]string{}
			if tc.url != "nourl" {
				params[urlParam] = tc.url
			}
			err := resolver.ValidateParams(contextWithConfig(defaultHttpTimeoutValue), toParams(params))
			if tc.expectedErr != nil {
				checkExpectedErr(t, tc.expectedErr, err)
			} else if err != nil {
				t.Fatalf("unexpected error validating params: %v", err)
			}
		})
	}
}

func TestMakeHTTPClient(t *testing.T) {
	tests := []struct {
		name        string
		expectedErr error
		duration    string
	}{
		{
			name:     "good/duration",
			duration: "30s",
		},
		{
			name:        "bad/duration",
			duration:    "xxx",
			expectedErr: fmt.Errorf(`error parsing timeout value xxx: time: invalid duration "xxx"`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := makeHttpClient(contextWithConfig(tc.duration))
			if tc.expectedErr != nil {
				checkExpectedErr(t, tc.expectedErr, err)
				return
			} else if err != nil {
				t.Fatalf("unexpected error creating http client: %v", err)
			}
			if client == nil {
				t.Fatalf("expected an http client but got nil")
			}
			if client.Timeout.String() != tc.duration {
				t.Fatalf("expected timeout %v but got %v", tc.duration, client.Timeout)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name           string
		expectedErr    string
		input          string
		paramSet       bool
		expectedStatus int
	}{
		{
			name:     "good/params set",
			input:    "task",
			paramSet: true,
		}, {
			name:        "bad/params not set",
			input:       "task",
			expectedErr: `missing required http resolver params: url`,
		}, {
			name:           "bad/not found",
			input:          "task",
			paramSet:       true,
			expectedStatus: http.StatusNotFound,
			expectedErr:    `requested URL 'http://([^']*)' is not found`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.expectedStatus != 0 {
					w.WriteHeader(tc.expectedStatus)
				}
				fmt.Fprintf(w, tc.input)
			}))
			params := []pipelinev1.Param{}
			if tc.paramSet {
				params = append(params, pipelinev1.Param{
					Name:  urlParam,
					Value: *pipelinev1.NewStructuredValues(svr.URL),
				})
			}
			resolver := Resolver{}
			output, err := resolver.Resolve(contextWithConfig(defaultHttpTimeoutValue), params)
			if tc.expectedErr != "" {
				re := regexp.MustCompile(tc.expectedErr)
				if !re.MatchString(err.Error()) {
					t.Fatalf("expected error '%v' but got '%v'", tc.expectedErr, err)
				}
				return
			} else if err != nil {
				t.Fatalf("unexpected error validating params: %v", err)
			}
			if o := cmp.Diff(tc.input, string(output.Data())); o != "" {
				t.Fatalf("expected output '%v' but got '%v'", tc.input, string(output.Data()))
			}
			if o := cmp.Diff(svr.URL, output.RefSource().URI); o != "" {
				t.Fatalf("expected url '%v' but got '%v'", svr.URL, output.RefSource().URI)
			}

			eSum := sha256.New()
			eSum.Write([]byte(tc.input))
			eSha256 := hex.EncodeToString(eSum.Sum(nil))
			if o := cmp.Diff(eSha256, output.RefSource().Digest["sha256"]); o != "" {
				t.Fatalf("expected sha256 '%v' but got '%v'", eSha256, output.RefSource().Digest["sha256"])
			}

			if output.Annotations() != nil {
				t.Fatalf("output annotations should be nil")
			}
		})
	}
}

func TestResolveNotEnabled(t *testing.T) {
	var err error
	resolver := Resolver{}
	someParams := map[string]string{}
	_, err = resolver.Resolve(resolverDisabledContext(), toParams(someParams))
	if err == nil {
		t.Fatalf("expected disabled err")
	}
	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
	err = resolver.ValidateParams(resolverDisabledContext(), toParams(map[string]string{}))
	if err == nil {
		t.Fatalf("expected disabled err")
	}
	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

func TestInitialize(t *testing.T) {
	resolver := Resolver{}
	err := resolver.Initialize(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetName(t *testing.T) {
	resolver := Resolver{}
	ctx := context.Background()

	if d := cmp.Diff(httpResolverName, resolver.GetName(ctx)); d != "" {
		t.Errorf("invalid name: %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(configMapName, resolver.GetConfigName(ctx)); d != "" {
		t.Errorf("invalid config map name: %s", diff.PrintWantGot(d))
	}
}

func resolverDisabledContext() context.Context {
	return frtesting.ContextWithHttpResolverDisabled(context.Background())
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

func contextWithConfig(timeout string) context.Context {
	config := map[string]string{
		timeoutKey: timeout,
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
