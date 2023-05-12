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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/tektoncd/pipeline/pkg/apis/config"
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
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	pipeline := parse.MustParseV1beta1Pipeline(t, pipelineYAMLString)
	pipelineRef := &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name         string
		pipelineYAML string
	}{{
		name: "v1beta1 pipeline",
		pipelineYAML: strings.Join([]string{
			"kind: Pipeline",
			"apiVersion: tekton.dev/v1beta1",
			pipelineYAMLString,
		}, "\n"),
	}, {
		name: "v1 pipeline",
		pipelineYAML: strings.Join([]string{
			"kind: Pipeline",
			"apiVersion: tekton.dev/v1",
			pipelineYAMLString,
		}, "\n"),
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
			if err != nil {
				t.Fatalf("failed to call pipelinefn: %s", err.Error())
			}

			if diff := cmp.Diff(pipeline, resolvedPipeline); diff != "" {
				t.Error(diff)
			}

			if d := cmp.Diff(sampleRefSource, resolvedRefSource); d != "" {
				t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
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

func TestGetPipelineFunc_VerifySuccess(t *testing.T) {
	// This test case tests the success cases of trusted-resources-verification-no-match-policy when it is set to
	// fail: passed matching policy verification
	// warn and ignore: no matching policies.
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

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, signer, "signed")
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
		name:                       "ignore unsigned pipeline without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   vps,
		expected:                   unsignedPipeline,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationSkip},
	}, {
		name:                       "warn no policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   []*v1alpha1.VerificationPolicy{},
		expected:                   unsignedPipeline,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrNoMatchedPolicies},
	}, {
		name:                       "ignore no policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		pipelinerun:                pr,
		policies:                   []*v1alpha1.VerificationPolicy{},
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

			resolvedPipeline, source, vr, err := fn(ctx, pipelineRef.Name)
			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}
			if d := cmp.Diff(tc.expected, resolvedPipeline); d != "" {
				t.Errorf("resolvedPipeline did not match: %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expectedRefSource, source); d != "" {
				t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expectedVerificationResult, vr, cmpopts.EquateErrors()); d != "" {
				t.Errorf("VerificationResult did not match: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetPipelineFunc_VerifyError(t *testing.T) {
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

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, signer, "signed")
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
		name                           string
		requester                      *test.Requester
		verificationNoMatchPolicy      string
		expectedErr                    error
		expectedVerificationResultType trustedresources.VerificationResultType
	}{
		{
			name:                           "unsigned pipeline fails verification with fail no match policy",
			requester:                      requesterUnsigned,
			verificationNoMatchPolicy:      config.FailNoMatchPolicy,
			expectedErr:                    trustedresources.ErrResourceVerificationFailed,
			expectedVerificationResultType: trustedresources.VerificationError,
		}, {
			name:                           "unsigned pipeline fails verification with warn no match policy",
			requester:                      requesterUnsigned,
			verificationNoMatchPolicy:      config.WarnNoMatchPolicy,
			expectedErr:                    trustedresources.ErrResourceVerificationFailed,
			expectedVerificationResultType: trustedresources.VerificationError,
		}, {
			name:                           "unsigned pipeline fails verification with ignore no match policy",
			requester:                      requesterUnsigned,
			verificationNoMatchPolicy:      config.IgnoreNoMatchPolicy,
			expectedErr:                    trustedresources.ErrResourceVerificationFailed,
			expectedVerificationResultType: trustedresources.VerificationError,
		}, {
			name:                           "modified pipeline fails verification with fail no match policy",
			requester:                      requesterModified,
			verificationNoMatchPolicy:      config.FailNoMatchPolicy,
			expectedErr:                    trustedresources.ErrResourceVerificationFailed,
			expectedVerificationResultType: trustedresources.VerificationError,
		}, {
			name:                           "modified pipeline fails verification with warn no match policy",
			requester:                      requesterModified,
			verificationNoMatchPolicy:      config.WarnNoMatchPolicy,
			expectedErr:                    trustedresources.ErrResourceVerificationFailed,
			expectedVerificationResultType: trustedresources.VerificationError,
		}, {
			name:                           "modified pipeline fails verification with ignore no match policy",
			requester:                      requesterModified,
			verificationNoMatchPolicy:      config.IgnoreNoMatchPolicy,
			expectedErr:                    trustedresources.ErrResourceVerificationFailed,
			expectedVerificationResultType: trustedresources.VerificationError,
		}, {
			name:                           "unmatched pipeline fails with fail no match policy",
			requester:                      requesterUnmatched,
			verificationNoMatchPolicy:      config.FailNoMatchPolicy,
			expectedErr:                    trustedresources.ErrNoMatchedPolicies,
			expectedVerificationResultType: trustedresources.VerificationError,
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

			_, _, vr, _ := fn(ctx, pipelineRef.Name)
			if !errors.Is(vr.Err, tc.expectedErr) {
				t.Errorf("GetPipelineFunc got %v, want %v", err, tc.expectedErr)
			}
			if tc.expectedVerificationResultType != vr.VerificationResultType {
				t.Errorf("VerificationResultType mismatch, want %d got %d", tc.expectedVerificationResultType, vr.VerificationResultType)
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
