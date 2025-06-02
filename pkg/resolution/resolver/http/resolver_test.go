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
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/internal/resolution"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	common "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	frtesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

type params struct {
	url               string
	authUsername      string
	authSecret        string
	authSecretKey     string
	authSecretContent string
}

const sampleTask = `---
kind: Task
apiVersion: tekton.dev/v1
metadata:
  name: foo
spec:
  steps:
  - name: step1
    image: scratch`
const emptyStr = "empty"

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(t.Context())
	if typ, has := sel[common.LabelKeyResolverType]; !has {
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
			expectedErr: errors.New(`url xttps:ufoo/bar/ is not a valid http(s) url`),
		}, {
			name:        "invalid/url empty",
			url:         "",
			expectedErr: errors.New(`cannot parse url : parse "": empty url`),
		}, {
			name:        "missing/url",
			expectedErr: errors.New(`missing required http resolver params: url`),
			url:         "nourl",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := Resolver{}
			params := map[string]string{}
			if tc.url != "nourl" {
				params[UrlParam] = tc.url
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
			expectedErr: errors.New(`error parsing timeout value xxx: time: invalid duration "xxx"`),
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
				fmt.Fprint(w, tc.input)
			}))
			params := []pipelinev1.Param{}
			if tc.paramSet {
				params = append(params, pipelinev1.Param{
					Name:  UrlParam,
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

func createRequest(params *params) *v1beta1.ResolutionRequest {
	rr := &v1beta1.ResolutionRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resolution.tekton.dev/v1beta1",
			Kind:       "ResolutionRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				common.LabelKeyResolverType: LabelValueHttpResolverType,
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{
			Params: []pipelinev1.Param{{
				Name:  UrlParam,
				Value: *pipelinev1.NewStructuredValues(params.url),
			}},
		},
	}
	if params.authSecret != "" {
		s := params.authSecret
		if s == emptyStr {
			s = ""
		}
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  HttpBasicAuthSecret,
			Value: *pipelinev1.NewStructuredValues(s),
		})
	}

	if params.authUsername != "" {
		s := params.authUsername
		if s == emptyStr {
			s = ""
		}
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  HttpBasicAuthUsername,
			Value: *pipelinev1.NewStructuredValues(s),
		})
	}

	if params.authSecretKey != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  HttpBasicAuthSecretKey,
			Value: *pipelinev1.NewStructuredValues(params.authSecretKey),
		})
	}

	return rr
}

func TestResolverReconcileBasicAuth(t *testing.T) {
	var doNotCreate string = "notcreate"
	var wrongSecretKey string = "wrongsecretk"

	tests := []struct {
		name           string
		params         *params
		taskContent    string
		expectedStatus *v1beta1.ResolutionRequestStatus
		expectedErr    error
	}{
		{
			name:           "good/URL Resolution",
			taskContent:    sampleTask,
			expectedStatus: resolution.CreateResolutionRequestStatusWithData([]byte(sampleTask)),
		},
		{
			name:           "good/URL Resolution with custom basic auth, and custom secret key",
			taskContent:    sampleTask,
			expectedStatus: resolution.CreateResolutionRequestStatusWithData([]byte(sampleTask)),
			params: &params{
				authSecret:        "auth-secret",
				authUsername:      "auth",
				authSecretKey:     "token",
				authSecretContent: "untoken",
			},
		},
		{
			name:           "good/URL Resolution with custom basic auth no custom secret key",
			taskContent:    sampleTask,
			expectedStatus: resolution.CreateResolutionRequestStatusWithData([]byte(sampleTask)),
			params: &params{
				authSecret:        "auth-secret",
				authUsername:      "auth",
				authSecretContent: "untoken",
			},
		},
		{
			name:        "bad/no url found",
			params:      &params{},
			expectedErr: errors.New(`invalid resource request "foo/rr": cannot parse url : parse "": empty url`),
		},
		{
			name: "bad/no secret found",
			params: &params{
				authSecret:   doNotCreate,
				authUsername: "user",
				url:          "https://blah/blah.com",
			},
			expectedErr: errors.New(`error getting "Http" "foo/rr": cannot get API token, secret notcreate not found in namespace foo`),
		},
		{
			name: "bad/no valid secret key",
			params: &params{
				authSecret:    "shhhhh",
				authUsername:  "user",
				authSecretKey: wrongSecretKey,
				url:           "https://blah/blah",
			},
			expectedErr: errors.New(`error getting "Http" "foo/rr": cannot get API token, key wrongsecretk not found in secret shhhhh in namespace foo`),
		},
		{
			name: "bad/missing username params for secret with params",
			params: &params{
				authSecret: "shhhhh",
				url:        "https://blah/blah",
			},
			expectedErr: errors.New(`invalid resource request "foo/rr": missing required param http-username when using http-password-secret`),
		},
		{
			name: "bad/missing password params for secret with username",
			params: &params{
				authUsername: "failure",
				url:          "https://blah/blah",
			},
			expectedErr: errors.New(`invalid resource request "foo/rr": missing required param http-password-secret when using http-username`),
		},
		{
			name: "bad/empty auth username",
			params: &params{
				authUsername: emptyStr,
				authSecret:   "asecret",
				url:          "https://blah/blah",
			},
			expectedErr: errors.New(`invalid resource request "foo/rr": value http-username cannot be empty`),
		},
		{
			name: "bad/empty auth password",
			params: &params{
				authUsername: "auser",
				authSecret:   emptyStr,
				url:          "https://blah/blah",
			},
			expectedErr: errors.New(`invalid resource request "foo/rr": value http-password-secret cannot be empty`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{}
			ctx, _ := ttesting.SetupFakeContext(t)
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.taskContent)
			}))
			p := tt.params
			if p == nil {
				p = &params{}
			}
			if p.url == "" && tt.taskContent != "" {
				p.url = svr.URL
			}
			request := createRequest(p)
			cfg := make(map[string]string)
			d := test.Data{
				ConfigMaps: []*corev1.ConfigMap{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMapName,
						Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
					},
					Data: cfg,
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
						Name:      resolverconfig.GetFeatureFlagsConfigName(),
					},
					Data: map[string]string{
						"enable-http-resolver": "true",
					},
				}},
				ResolutionRequests: []*v1beta1.ResolutionRequest{request},
			}
			var expectedStatus *v1beta1.ResolutionRequestStatus
			if tt.expectedStatus != nil {
				expectedStatus = tt.expectedStatus.DeepCopy()
				if tt.expectedErr == nil {
					if tt.taskContent != "" {
						h := sha256.New()
						h.Write([]byte(tt.taskContent))
						sha256CheckSum := hex.EncodeToString(h.Sum(nil))
						refsrc := &pipelinev1.RefSource{
							URI: svr.URL,
							Digest: map[string]string{
								"sha256": sha256CheckSum,
							},
						}
						expectedStatus.RefSource = refsrc
						expectedStatus.Source = refsrc
					}
				} else {
					expectedStatus.Status.Conditions[0].Message = tt.expectedErr.Error()
				}
			}
			frtesting.RunResolverReconcileTest(ctx, t, d, resolver, request, expectedStatus, tt.expectedErr, func(resolver framework.Resolver, testAssets test.Assets) {
				if err := resolver.Initialize(ctx); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.params == nil {
					return
				}
				if tt.params.authSecret != "" && tt.params.authSecret != doNotCreate {
					secretKey := tt.params.authSecretKey
					if secretKey == wrongSecretKey {
						secretKey = "randomNotOund"
					}
					if secretKey == "" {
						secretKey = defaultBasicAuthSecretKey
					}
					tokenSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      tt.params.authSecret,
							Namespace: request.GetNamespace(),
						},
						Data: map[string][]byte{
							secretKey: []byte(base64.StdEncoding.Strict().EncodeToString([]byte(tt.params.authSecretContent))),
						},
					}
					if _, err := testAssets.Clients.Kube.CoreV1().Secrets(request.GetNamespace()).Create(ctx, tokenSecret, metav1.CreateOptions{}); err != nil {
						t.Fatalf("failed to create test token secret: %v", err)
					}
				}
			})
		})
	}
}

func TestGetName(t *testing.T) {
	resolver := Resolver{}
	ctx := t.Context()

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
		TimeoutKey: timeout,
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
