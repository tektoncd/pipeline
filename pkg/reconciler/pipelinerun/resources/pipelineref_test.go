/*
 Copyright 2020 The Tekton Authors

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

package resources_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
)

var (
	dummyPipeline = &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummy",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pipeline",
			APIVersion: "tekton.dev/v1beta1",
		},
	}

	sampleRefSource = &v1beta1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}

	unsignedV1Pipeline = &pipelinev1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Pipeline"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pipeline",
			Namespace:   "trusted-resources",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: pipelinev1.PipelineSpec{
			Tasks: []pipelinev1.PipelineTask{
				{
					Name: "task",
				},
			},
		},
	}

	verificationResultCmp = cmp.Comparer(func(x, y trustedresources.VerificationResult) bool {
		return x.VerificationResultType == y.VerificationResultType && (errors.Is(x.Err, y.Err) || errors.Is(y.Err, x.Err))
	})
)

func TestLocalPipelineRef(t *testing.T) {
	testcases := []struct {
		name      string
		pipelines []runtime.Object
		ref       *v1beta1.PipelineRef
		expected  runtime.Object
		wantErr   bool
	}{
		{
			name:      "local-pipeline",
			pipelines: []runtime.Object{simplePipeline(), dummyPipeline},
			ref: &v1beta1.PipelineRef{
				Name: "simple",
			},
			expected: simplePipeline(),
			wantErr:  false,
		},
		{
			name:      "pipeline-not-found",
			pipelines: []runtime.Object{},
			ref: &v1beta1.PipelineRef{
				Name: "simple",
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			tektonclient := fake.NewSimpleClientset(tc.pipelines...)

			lc := &resources.LocalPipelineRefResolver{
				Namespace:    "default",
				Tektonclient: tektonclient,
			}

			resolvedPipeline, resolvedRefSource, _, err := lc.GetPipeline(ctx, tc.ref.Name)
			if tc.wantErr && err == nil {
				t.Fatal("Expected error but found nil instead")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(resolvedPipeline, tc.expected); tc.expected != nil && d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			if resolvedRefSource != nil {
				t.Errorf("expected refSource is nil, but got %v", resolvedRefSource)
			}
		})
	}
}

func TestGetPipelineFunc(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	cfg.OnConfigChanged(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
		Data: map[string]string{
			"enable-tekton-oci-bundles": "true",
		},
	})
	ctx = cfg.ToContext(ctx)

	testcases := []struct {
		name            string
		localPipelines  []runtime.Object
		remotePipelines []runtime.Object
		ref             *v1beta1.PipelineRef
		expected        runtime.Object
	}{{
		name: "remote-pipeline",
		localPipelines: []runtime.Object{
			simplePipelineWithBaseSpec(),
			dummyPipeline,
		},
		remotePipelines: []runtime.Object{simplePipeline(), dummyPipeline},
		ref: &v1beta1.PipelineRef{
			Name:   "simple",
			Bundle: u.Host + "/remote-pipeline",
		},
		expected: simplePipeline(),
	}, {
		name: "local-pipeline",
		localPipelines: []runtime.Object{
			simplePipelineWithBaseSpec(),
			dummyPipeline,
		},
		remotePipelines: []runtime.Object{simplePipeline(), dummyPipeline},
		ref: &v1beta1.PipelineRef{
			Name: "simple",
		},
		expected: simplePipelineWithBaseSpec(),
	}, {
		name:           "remote-pipeline-without-defaults",
		localPipelines: []runtime.Object{simplePipeline()},
		remotePipelines: []runtime.Object{
			simplePipelineWithSpecAndParam(""),
			dummyPipeline},
		ref: &v1beta1.PipelineRef{
			Name:   "simple",
			Bundle: u.Host + "/remote-pipeline-without-defaults",
		},
		expected: simplePipelineWithSpecParamAndKind(),
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tektonclient := fake.NewSimpleClientset(tc.localPipelines...)
			kubeclient := fakek8s.NewSimpleClientset(&v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "default",
				},
			})

			_, err := test.CreateImage(u.Host+"/"+tc.name, tc.remotePipelines...)
			if err != nil {
				t.Fatalf("failed to upload test image: %s", err.Error())
			}

			fn := resources.GetPipelineFunc(ctx, kubeclient, tektonclient, nil, &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        tc.ref,
					ServiceAccountName: "default",
				},
			}, nil /*VerificationPolicies*/)
			if err != nil {
				t.Fatalf("failed to get pipeline fn: %s", err.Error())
			}

			pipeline, refSource, _, err := fn(ctx, tc.ref.Name)
			if err != nil {
				t.Fatalf("failed to call pipelinefn: %s", err.Error())
			}

			if diff := cmp.Diff(pipeline, tc.expected); tc.expected != nil && diff != "" {
				t.Error(diff)
			}

			if refSource != nil {
				t.Errorf("expected refSource is nil, but got %v", refSource)
			}
		})
	}
}

func TestGetPipelineFuncSpecAlreadyFetched(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	tektonclient := fake.NewSimpleClientset(simplePipeline(), dummyPipeline)
	kubeclient := fakek8s.NewSimpleClientset(&v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default",
		},
	})

	name := "anyname-really"
	pipelineSpec := v1beta1.PipelineSpec{
		Tasks: []v1beta1.PipelineTask{{
			Name:    "task1",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		}},
	}
	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				// Using simple here to show that, it won't fetch the simple pipelinespec,
				// which is different from the pipelineSpec above
				Name: "simple",
			},
			ServiceAccountName: "default",
		},
		Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			PipelineSpec: &pipelineSpec,
			Provenance: &v1beta1.Provenance{
				RefSource: sampleRefSource.DeepCopy(),
			},
		}},
	}
	expectedPipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: pipelineSpec,
	}

	fn := resources.GetPipelineFunc(ctx, kubeclient, tektonclient, nil, pipelineRun, nil /*VerificationPolicies*/)
	actualPipeline, actualRefSource, _, err := fn(ctx, name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if diff := cmp.Diff(actualPipeline, expectedPipeline); expectedPipeline != nil && diff != "" {
		t.Error(diff)
	}

	if d := cmp.Diff(sampleRefSource, actualRefSource); d != "" {
		t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
	}
}

func TestGetPipelineFunc_RemoteResolution(t *testing.T) {
	ctx := config.EnableStableAPIFields(context.Background())
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	pipelineRef := &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name         string
		pipelineYAML string
		wantPipeline *v1beta1.Pipeline
		wantErr      bool
	}{{
		name: "v1beta1 pipeline",
		pipelineYAML: strings.Join([]string{
			"kind: Pipeline",
			"apiVersion: tekton.dev/v1beta1",
			pipelineYAMLString,
		}, "\n"),
		wantPipeline: parse.MustParseV1beta1Pipeline(t, pipelineYAMLString),
	}, {
		name: "v1beta1 pipeline with beta features",
		pipelineYAML: strings.Join([]string{
			"kind: Pipeline",
			"apiVersion: tekton.dev/v1beta1",
			pipelineYAMLStringWithBetaFeatures,
		}, "\n"),
		wantPipeline: parse.MustParseV1beta1Pipeline(t, pipelineYAMLStringWithBetaFeatures),
	}, {
		name: "v1 pipeline",
		pipelineYAML: strings.Join([]string{
			"kind: Pipeline",
			"apiVersion: tekton.dev/v1",
			pipelineYAMLString,
		}, "\n"),
		wantPipeline: parse.MustParseV1beta1Pipeline(t, pipelineYAMLString),
	}, {
		name: "v1 pipeline with beta features",
		pipelineYAML: strings.Join([]string{
			"kind: Pipeline",
			"apiVersion: tekton.dev/v1",
			pipelineYAMLStringWithBetaFeatures,
		}, "\n"),
		wantPipeline: nil,
		wantErr:      true,
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			resolved := test.NewResolvedResource([]byte(tc.pipelineYAML), nil /* annotations */, sampleRefSource.DeepCopy(), nil /* data error */)
			requester := test.NewRequester(resolved, nil)
			fn := resources.GetPipelineFunc(ctx, nil, nil, requester, &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        pipelineRef,
					ServiceAccountName: "default",
				},
			}, nil /*VerificationPolicies*/)

			resolvedPipeline, resolvedRefSource, _, err := fn(ctx, pipelineRef.Name)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected an error when calling pipelinefunc but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("failed to call pipelinefn: %s", err.Error())
				}

				if diff := cmp.Diff(tc.wantPipeline, resolvedPipeline); diff != "" {
					t.Error(diff)
				}

				if d := cmp.Diff(sampleRefSource, resolvedRefSource); d != "" {
					t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestGetPipelineFunc_RemoteResolution_ReplacedParams(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	pipeline := parse.MustParseV1beta1Pipeline(t, pipelineYAMLString)
	pipelineRef := &v1beta1.PipelineRef{
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "git",
			Params: v1beta1.Params{{
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("$(params.resolver-param)"),
			}, {
				Name:  "bar",
				Value: *v1beta1.NewStructuredValues("$(context.pipelineRun.name)"),
			}},
		},
	}
	pipelineYAML := strings.Join([]string{
		"kind: Pipeline",
		"apiVersion: tekton.dev/v1beta1",
		pipelineYAMLString,
	}, "\n")

	resolved := test.NewResolvedResource([]byte(pipelineYAML), nil, sampleRefSource.DeepCopy(), nil)
	requester := &test.Requester{
		ResolvedResource: resolved,
		Params: v1beta1.Params{{
			Name:  "foo",
			Value: *v1beta1.NewStructuredValues("bar"),
		}, {
			Name:  "bar",
			Value: *v1beta1.NewStructuredValues("test-pipeline"),
		}},
	}
	fn := resources.GetPipelineFunc(ctx, nil, nil, requester, &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
			Params: v1beta1.Params{{
				Name:  "resolver-param",
				Value: *v1beta1.NewStructuredValues("bar"),
			}},
		},
	}, nil /*VerificationPolicies*/)

	resolvedPipeline, resolvedRefSource, _, err := fn(ctx, pipelineRef.Name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if diff := cmp.Diff(pipeline, resolvedPipeline); diff != "" {
		t.Error(diff)
	}

	if d := cmp.Diff(sampleRefSource, resolvedRefSource); d != "" {
		t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
	}

	pipelineRefNotMatching := &v1beta1.PipelineRef{
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "git",
			Params: v1beta1.Params{{
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("$(params.resolver-param)"),
			}, {
				Name:  "bar",
				Value: *v1beta1.NewStructuredValues("$(context.pipelineRun.name)"),
			}},
		},
	}

	fnNotMatching := resources.GetPipelineFunc(ctx, nil, nil, requester, &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pipeline",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRefNotMatching,
			ServiceAccountName: "default",
			Params: v1beta1.Params{{
				Name:  "resolver-param",
				Value: *v1beta1.NewStructuredValues("banana"),
			}},
		},
	}, nil /*VerificationPolicies*/)

	_, _, _, err = fnNotMatching(ctx, pipelineRefNotMatching.Name)
	if err == nil {
		t.Fatal("expected error for non-matching params, did not get one")
	}
	if !strings.Contains(err.Error(), `StringVal: "banana"`) {
		t.Fatalf("did not receive expected error, got '%s'", err.Error())
	}
}

func TestGetPipelineFunc_RemoteResolutionInvalidData(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	pipelineRef := &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}
	resolvesTo := []byte("INVALID YAML")
	resource := test.NewResolvedResource(resolvesTo, nil, nil, nil)
	requester := test.NewRequester(resource, nil)
	fn := resources.GetPipelineFunc(ctx, nil, nil, requester, &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
		},
	}, nil /*VerificationPolicies*/)
	if _, _, _, err := fn(ctx, pipelineRef.Name); err == nil {
		t.Fatalf("expected error due to invalid pipeline data but saw none")
	}
}

func TestGetPipelineFunc_V1beta1Pipeline_VerifyNoError(t *testing.T) {
	ctx := context.Background()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	tektonclient := fake.NewSimpleClientset()

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")
	unsignedPipelineBytes, err := json.Marshal(unsignedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	noMatchPolicyRefSource := &v1beta1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedUnmatched := test.NewResolvedResource(unsignedPipelineBytes, nil, noMatchPolicyRefSource, nil)
	requesterUnmatched := test.NewRequester(resolvedUnmatched, nil)

	signedPipeline, err := test.GetSignedV1beta1Pipeline(unsignedPipeline, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineBytes, err := json.Marshal(signedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	matchPolicyRefSource := &v1beta1.RefSource{
		URI: "	https://github.com/tektoncd/catalog.git",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedMatched := test.NewResolvedResource(signedPipelineBytes, nil, matchPolicyRefSource, nil)
	requesterMatched := test.NewRequester(resolvedMatched, nil)

	pipelineRef := &v1beta1.PipelineRef{
		Name: signedPipeline.Name,
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "git",
		},
	}

	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
		},
	}

	prWithStatus := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				PipelineSpec: &signedPipeline.Spec,
				Provenance: &v1beta1.Provenance{
					RefSource: &v1beta1.RefSource{
						URI:        "abc.com",
						Digest:     map[string]string{"sha1": "a123"},
						EntryPoint: "foo/bar",
					},
				},
			},
		},
	}

	warnPolicyRefSource := &v1beta1.RefSource{
		URI: "	warnVP",
	}
	resolvedUnsignedMatched := test.NewResolvedResource(unsignedPipelineBytes, nil, warnPolicyRefSource, nil)
	requesterUnsignedMatched := test.NewRequester(resolvedUnsignedMatched, nil)

	testcases := []struct {
		name                       string
		requester                  *test.Requester
		verificationNoMatchPolicy  string
		pipelinerun                v1beta1.PipelineRun
		policies                   []*v1alpha1.VerificationPolicy
		expected                   runtime.Object
		expectedRefSource          *v1beta1.RefSource
		expectedVerificationResult *trustedresources.VerificationResult
	}{{
		name:                       "signed pipeline with matching policy pass verification with enforce no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   signedPipeline,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "signed pipeline with matching policy pass verification with warn no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   signedPipeline,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "signed pipeline with matching policy pass verification with ignore no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   signedPipeline,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "warn unsigned pipeline without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   unsignedPipeline,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrNoMatchedPolicies},
	}, {
		name:                       "unsigned pipeline fails warn mode policies doesn't return error",
		requester:                  requesterUnsignedMatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   unsignedPipeline,
		expectedRefSource:          warnPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "ignore unsigned pipeline without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   unsignedPipeline,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationSkip},
	}, {
		name:                      "signed pipeline in status no need to verify",
		requester:                 requesterMatched,
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		pipelinerun:               prWithStatus,
		policies:                  vps,
		expected: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      signedPipeline.Name,
				Namespace: signedPipeline.Namespace,
			},
			Spec: signedPipeline.Spec,
		},
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: nil,
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			fn := resources.GetPipelineFunc(ctx, k8sclient, tektonclient, tc.requester, &tc.pipelinerun, tc.policies)

			gotResolvedPipeline, gotSource, gotVerificationResult, err := fn(ctx, pipelineRef.Name)
			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}
			if d := cmp.Diff(tc.expected, gotResolvedPipeline); d != "" {
				t.Errorf("resolvedPipeline did not match: %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expectedRefSource, gotSource); d != "" {
				t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
			}
			if tc.expectedVerificationResult == nil {
				if gotVerificationResult != nil {
					t.Errorf("VerificationResult did not match: want %v, got %v", tc.expectedVerificationResult, gotVerificationResult)
				}
				return
			}
			if d := cmp.Diff(gotVerificationResult, tc.expectedVerificationResult, verificationResultCmp); d != "" {
				t.Errorf("VerificationResult did not match:%s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetPipelineFunc_V1beta1Pipeline_VerifyError(t *testing.T) {
	ctx := context.Background()
	tektonclient := fake.NewSimpleClientset()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")
	unsignedPipelineBytes, err := json.Marshal(unsignedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	matchPolicyRefSource := &v1beta1.RefSource{
		URI: "https://github.com/tektoncd/catalog.git",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedPipelineBytes, nil, matchPolicyRefSource, nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)

	signedPipeline, err := test.GetSignedV1beta1Pipeline(unsignedPipeline, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineBytes, err := json.Marshal(signedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}

	noMatchPolicyRefSource := &v1beta1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedUnmatched := test.NewResolvedResource(signedPipelineBytes, nil, noMatchPolicyRefSource, nil)
	requesterUnmatched := test.NewRequester(resolvedUnmatched, nil)

	modifiedPipeline := signedPipeline.DeepCopy()
	modifiedPipeline.Annotations["random"] = "attack"
	modifiedPipelineBytes, err := json.Marshal(modifiedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	resolvedModified := test.NewResolvedResource(modifiedPipelineBytes, nil, matchPolicyRefSource, nil)
	requesterModified := test.NewRequester(resolvedModified, nil)

	pipelineRef := &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                       string
		requester                  *test.Requester
		verificationNoMatchPolicy  string
		expectedVerificationResult *trustedresources.VerificationResult
	}{
		{
			name:                       "unsigned pipeline fails verification with fail no match policy",
			requester:                  requesterUnsigned,
			verificationNoMatchPolicy:  config.FailNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "unsigned pipeline fails verification with warn no match policy",
			requester:                  requesterUnsigned,
			verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "unsigned pipeline fails verification with ignore no match policy",
			requester:                  requesterUnsigned,
			verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "modified pipeline fails verification with fail no match policy",
			requester:                  requesterModified,
			verificationNoMatchPolicy:  config.FailNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "modified pipeline fails verification with warn no match policy",
			requester:                  requesterModified,
			verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "modified pipeline fails verification with ignore no match policy",
			requester:                  requesterModified,
			verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "unmatched pipeline fails with fail no match policy",
			requester:                  requesterUnmatched,
			verificationNoMatchPolicy:  config.FailNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrNoMatchedPolicies},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        pipelineRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetPipelineFunc(ctx, k8sclient, tektonclient, tc.requester, pr, vps)

			_, _, gotVerificationResult, err := fn(ctx, pipelineRef.Name)
			if err != nil {
				t.Errorf("want err nil but got %v", err)
			}
			if d := cmp.Diff(gotVerificationResult, tc.expectedVerificationResult, verificationResultCmp); d != "" {
				t.Errorf("VerificationResult did not match:%s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetPipelineFunc_V1Pipeline_VerifyNoError(t *testing.T) {
	ctx := context.Background()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	tektonclient := fake.NewSimpleClientset()

	v1beta1UnsignedPipeline := &v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pipeline",
			APIVersion: "tekton.dev/v1beta1",
		},
	}
	if err := v1beta1UnsignedPipeline.ConvertFrom(ctx, unsignedV1Pipeline.DeepCopy()); err != nil {
		t.Error(err)
	}

	unsignedPipelineBytes, err := json.Marshal(unsignedV1Pipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	noMatchPolicyRefSource := &v1beta1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedUnmatched := test.NewResolvedResource(unsignedPipelineBytes, nil, noMatchPolicyRefSource, nil)
	requesterUnmatched := test.NewRequester(resolvedUnmatched, nil)

	signedPipeline, err := getSignedV1Pipeline(unsignedV1Pipeline, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}

	v1beta1SignedPipeline := &v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pipeline",
			APIVersion: "tekton.dev/v1beta1",
		},
	}
	if err := v1beta1SignedPipeline.ConvertFrom(ctx, signedPipeline.DeepCopy()); err != nil {
		t.Error(err)
	}

	signedPipelineBytes, err := json.Marshal(signedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	matchPolicyRefSource := &v1beta1.RefSource{
		URI: "	https://github.com/tektoncd/catalog.git",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedMatched := test.NewResolvedResource(signedPipelineBytes, nil, matchPolicyRefSource, nil)
	requesterMatched := test.NewRequester(resolvedMatched, nil)

	pipelineRef := &v1beta1.PipelineRef{
		Name: signedPipeline.Name,
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "git",
		},
	}

	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
		},
	}

	prWithStatus := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				PipelineSpec: &v1beta1SignedPipeline.Spec,
				Provenance: &v1beta1.Provenance{
					RefSource: &v1beta1.RefSource{
						URI:        "abc.com",
						Digest:     map[string]string{"sha1": "a123"},
						EntryPoint: "foo/bar",
					},
				},
			},
		},
	}

	warnPolicyRefSource := &v1beta1.RefSource{
		URI: "	warnVP",
	}
	resolvedUnsignedMatched := test.NewResolvedResource(unsignedPipelineBytes, nil, warnPolicyRefSource, nil)
	requesterUnsignedMatched := test.NewRequester(resolvedUnsignedMatched, nil)

	testcases := []struct {
		name                       string
		requester                  *test.Requester
		verificationNoMatchPolicy  string
		pipelinerun                v1beta1.PipelineRun
		policies                   []*v1alpha1.VerificationPolicy
		expected                   runtime.Object
		expectedRefSource          *v1beta1.RefSource
		expectedVerificationResult *trustedresources.VerificationResult
	}{{
		name:                       "signed pipeline with matching policy pass verification with enforce no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   v1beta1SignedPipeline,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "signed pipeline with matching policy pass verification with warn no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   v1beta1SignedPipeline,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "signed pipeline with matching policy pass verification with ignore no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   v1beta1SignedPipeline,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "warn unsigned pipeline without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   v1beta1UnsignedPipeline,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrNoMatchedPolicies},
	}, {
		name:                       "unsigned pipeline fails warn mode policies doesn't return error",
		requester:                  requesterUnsignedMatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   v1beta1UnsignedPipeline,
		expectedRefSource:          warnPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "ignore unsigned pipeline without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   v1beta1UnsignedPipeline,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationSkip},
	}, {
		name:                      "signed pipeline in status no need to verify",
		requester:                 requesterMatched,
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		pipelinerun:               prWithStatus,
		policies:                  vps,
		expected: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      signedPipeline.Name,
				Namespace: signedPipeline.Namespace,
			},
			Spec: v1beta1SignedPipeline.Spec,
		},
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: nil,
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			fn := resources.GetPipelineFunc(ctx, k8sclient, tektonclient, tc.requester, &tc.pipelinerun, tc.policies)

			gotResolvedPipeline, gotSource, gotVerificationResult, err := fn(ctx, pipelineRef.Name)
			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}
			if d := cmp.Diff(tc.expected, gotResolvedPipeline); d != "" {
				t.Errorf("resolvedPipeline did not match: %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expectedRefSource, gotSource); d != "" {
				t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
			}
			if tc.expectedVerificationResult == nil {
				if gotVerificationResult != nil {
					t.Errorf("VerificationResult did not match: want %v, got %v", tc.expectedVerificationResult, gotVerificationResult)
				}
				return
			}
			if d := cmp.Diff(gotVerificationResult, tc.expectedVerificationResult, verificationResultCmp); d != "" {
				t.Errorf("VerificationResult did not match:%s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetPipelineFunc_V1Pipeline_VerifyError(t *testing.T) {
	ctx := context.Background()
	tektonclient := fake.NewSimpleClientset()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedPipelineBytes, err := json.Marshal(unsignedV1Pipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	matchPolicyRefSource := &v1beta1.RefSource{
		URI: "https://github.com/tektoncd/catalog.git",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedPipelineBytes, nil, matchPolicyRefSource, nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)

	signedPipeline, err := getSignedV1Pipeline(unsignedV1Pipeline, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineBytes, err := json.Marshal(signedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}

	noMatchPolicyRefSource := &v1beta1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedUnmatched := test.NewResolvedResource(signedPipelineBytes, nil, noMatchPolicyRefSource, nil)
	requesterUnmatched := test.NewRequester(resolvedUnmatched, nil)

	modifiedPipeline := signedPipeline.DeepCopy()
	modifiedPipeline.Annotations["random"] = "attack"
	modifiedPipelineBytes, err := json.Marshal(modifiedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	resolvedModified := test.NewResolvedResource(modifiedPipelineBytes, nil, matchPolicyRefSource, nil)
	requesterModified := test.NewRequester(resolvedModified, nil)

	pipelineRef := &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                       string
		requester                  *test.Requester
		verificationNoMatchPolicy  string
		expectedVerificationResult *trustedresources.VerificationResult
	}{
		{
			name:                       "unsigned pipeline fails verification with fail no match policy",
			requester:                  requesterUnsigned,
			verificationNoMatchPolicy:  config.FailNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "unsigned pipeline fails verification with warn no match policy",
			requester:                  requesterUnsigned,
			verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "unsigned pipeline fails verification with ignore no match policy",
			requester:                  requesterUnsigned,
			verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "modified pipeline fails verification with fail no match policy",
			requester:                  requesterModified,
			verificationNoMatchPolicy:  config.FailNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "modified pipeline fails verification with warn no match policy",
			requester:                  requesterModified,
			verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "modified pipeline fails verification with ignore no match policy",
			requester:                  requesterModified,
			verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
		}, {
			name:                       "unmatched pipeline fails with fail no match policy",
			requester:                  requesterUnmatched,
			verificationNoMatchPolicy:  config.FailNoMatchPolicy,
			expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrNoMatchedPolicies},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        pipelineRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetPipelineFunc(ctx, k8sclient, tektonclient, tc.requester, pr, vps)

			_, _, gotVerificationResult, err := fn(ctx, pipelineRef.Name)
			if err != nil {
				t.Errorf("want err nil but got %v", err)
			}
			if d := cmp.Diff(gotVerificationResult, tc.expectedVerificationResult, verificationResultCmp); d != "" {
				t.Errorf("VerificationResult did not match:%s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetPipelineFunc_GetFuncError(t *testing.T) {
	ctx := context.Background()
	tektonclient := fake.NewSimpleClientset()
	_, k8sclient, vps := test.SetupMatchAllVerificationPolicies(t, "trusted-resources")

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")
	unsignedPipelineBytes, err := json.Marshal(unsignedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedPipelineBytes, nil, sampleRefSource.DeepCopy(), nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)
	resolvedUnsigned.DataErr = fmt.Errorf("resolution error")

	prBundleError := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name:   "pipelineName",
				Bundle: "bundle",
			},
			ServiceAccountName: "default",
		},
	}

	prResolutionError := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "pipelineName",
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "git",
				},
			},
			ServiceAccountName: "default",
		},
	}

	testcases := []struct {
		name        string
		requester   *test.Requester
		pipelinerun v1beta1.PipelineRun
		expectedErr error
	}{
		{
			name:        "get error when oci bundle return error",
			requester:   requesterUnsigned,
			pipelinerun: *prBundleError,
			expectedErr: fmt.Errorf(`failed to get keychain: serviceaccounts "default" not found`),
		},
		{
			name:        "get error when remote resolution return error",
			requester:   requesterUnsigned,
			pipelinerun: *prResolutionError,
			expectedErr: fmt.Errorf("error accessing data from remote resource: %w", resolvedUnsigned.DataErr),
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
			featureflags := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "tekton-pipelines",
					Name:      "feature-flags",
				},
				Data: map[string]string{
					"enable-tekton-oci-bundles": "true",
				},
			}
			store.OnConfigChanged(featureflags)
			ctx = store.ToContext(ctx)

			fn := resources.GetPipelineFunc(ctx, k8sclient, tektonclient, tc.requester, &tc.pipelinerun, vps)

			_, _, _, err = fn(ctx, tc.pipelinerun.Spec.PipelineRef.Name)
			if d := cmp.Diff(err.Error(), tc.expectedErr.Error()); d != "" {
				t.Errorf("GetPipelineFunc got %v, want %v", err, tc.expectedErr)
			}
		})
	}
}

func basePipeline(name string) *v1beta1.Pipeline {
	return &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pipeline",
			APIVersion: "tekton.dev/v1beta1",
		},
	}
}

func simplePipeline() *v1beta1.Pipeline {
	return basePipeline("simple")
}

func simplePipelineWithBaseSpec() *v1beta1.Pipeline {
	p := simplePipeline()
	p.Spec = v1beta1.PipelineSpec{
		Tasks: []v1beta1.PipelineTask{{
			Name: "something",
			TaskRef: &v1beta1.TaskRef{
				Name: "something",
			},
		}},
	}

	return p
}

func simplePipelineWithSpecAndParam(pt v1beta1.ParamType) *v1beta1.Pipeline {
	p := simplePipelineWithBaseSpec()
	p.Spec.Params = []v1beta1.ParamSpec{{
		Name: "foo",
		Type: pt,
	}}

	return p
}

func simplePipelineWithSpecParamAndKind() *v1beta1.Pipeline {
	p := simplePipelineWithBaseSpec()
	p.Spec.Params = []v1beta1.ParamSpec{{
		Name: "foo",
	}}

	return p
}

// This is missing the kind and apiVersion because those are added by
// the MustParse helpers from the test package.
var pipelineYAMLString = `
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
`

var pipelineYAMLStringWithBetaFeatures = `
metadata:
  name: foo
spec:
  tasks:
  - name: task1
    taskRef:
      resolver: git
      params:
      - name: name
        value: test-task
`

func getSignedV1Pipeline(unsigned *pipelinev1.Pipeline, signer signature.Signer, name string) (*pipelinev1.Pipeline, error) {
	signed := unsigned.DeepCopy()
	signed.Name = name
	if signed.Annotations == nil {
		signed.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signed)
	if err != nil {
		return nil, err
	}
	signed.Annotations[trustedresources.SignatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signed, nil
}

func signInterface(signer signature.Signer, i interface{}) ([]byte, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is nil")
	}
	b, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write(b)

	sig, err := signer.SignMessage(bytes.NewReader(h.Sum(nil)))
	if err != nil {
		return nil, err
	}

	return sig, nil
}
