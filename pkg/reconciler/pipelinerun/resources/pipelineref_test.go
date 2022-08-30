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
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/tektoncd/pipeline/pkg/apis/config"
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

	sampleConfigSource = &v1beta1.ConfigSource{
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

			resolvedPipeline, resolvedConfigSource, err := lc.GetPipeline(ctx, tc.ref.Name)
			if tc.wantErr && err == nil {
				t.Fatal("Expected error but found nil instead")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(resolvedPipeline, tc.expected); tc.expected != nil && d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			if resolvedConfigSource != nil {
				t.Errorf("expected configsource is nil, but got %v", resolvedConfigSource)
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
		expected: simplePipelineWithSpecParamAndKind(v1beta1.ParamTypeString, v1beta1.NamespacedTaskKind),
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

			fn, err := resources.GetPipelineFunc(ctx, kubeclient, tektonclient, nil, &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        tc.ref,
					ServiceAccountName: "default",
				},
			})
			if err != nil {
				t.Fatalf("failed to get pipeline fn: %s", err.Error())
			}

			pipeline, configSource, err := fn(ctx, tc.ref.Name)
			if err != nil {
				t.Fatalf("failed to call pipelinefn: %s", err.Error())
			}

			if diff := cmp.Diff(pipeline, tc.expected); tc.expected != nil && diff != "" {
				t.Error(diff)
			}

			if configSource != nil {
				t.Errorf("expected configsource is nil, but got %v", configSource)
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
				ConfigSource: sampleConfigSource.DeepCopy(),
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

	fn, err := resources.GetPipelineFunc(ctx, kubeclient, tektonclient, nil, pipelineRun)
	if err != nil {
		t.Fatalf("failed to get pipeline fn: %s", err.Error())
	}
	actualPipeline, actualConfigSource, err := fn(ctx, name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if diff := cmp.Diff(actualPipeline, expectedPipeline); expectedPipeline != nil && diff != "" {
		t.Error(diff)
	}

	if d := cmp.Diff(sampleConfigSource, actualConfigSource); d != "" {
		t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
	}
}

func TestGetPipelineFunc_RemoteResolution(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	pipeline := parse.MustParseV1beta1Pipeline(t, pipelineYAMLString)
	pipelineRef := &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}
	pipelineYAML := strings.Join([]string{
		"kind: Pipeline",
		"apiVersion: tekton.dev/v1beta1",
		pipelineYAMLString,
	}, "\n")

	resolved := test.NewResolvedResource([]byte(pipelineYAML), nil, sampleConfigSource.DeepCopy(), nil)
	requester := test.NewRequester(resolved, nil)
	fn, err := resources.GetPipelineFunc(ctx, nil, nil, requester, &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
		},
	})
	if err != nil {
		t.Fatalf("failed to get pipeline fn: %s", err.Error())
	}

	resolvedPipeline, resolvedConfigSource, err := fn(ctx, pipelineRef.Name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if diff := cmp.Diff(pipeline, resolvedPipeline); diff != "" {
		t.Error(diff)
	}

	if d := cmp.Diff(sampleConfigSource, resolvedConfigSource); d != "" {
		t.Errorf("configsource did not match: %s", diff.PrintWantGot(d))
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
			Params: []v1beta1.Param{{
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

	resolved := test.NewResolvedResource([]byte(pipelineYAML), nil, sampleConfigSource.DeepCopy(), nil)
	requester := &test.Requester{
		ResolvedResource: resolved,
		Params: []v1beta1.Param{{
			Name:  "foo",
			Value: *v1beta1.NewStructuredValues("bar"),
		}, {
			Name:  "bar",
			Value: *v1beta1.NewStructuredValues("test-pipeline"),
		}},
	}
	fn, err := resources.GetPipelineFunc(ctx, nil, nil, requester, &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
			Params: []v1beta1.Param{{
				Name:  "resolver-param",
				Value: *v1beta1.NewStructuredValues("bar"),
			}},
		},
	})
	if err != nil {
		t.Fatalf("failed to get pipeline fn: %s", err.Error())
	}

	resolvedPipeline, resolvedConfigSource, err := fn(ctx, pipelineRef.Name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if diff := cmp.Diff(pipeline, resolvedPipeline); diff != "" {
		t.Error(diff)
	}

	if d := cmp.Diff(sampleConfigSource, resolvedConfigSource); d != "" {
		t.Errorf("configsource did not match: %s", diff.PrintWantGot(d))
	}

	pipelineRefNotMatching := &v1beta1.PipelineRef{
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "git",
			Params: []v1beta1.Param{{
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("$(params.resolver-param)"),
			}, {
				Name:  "bar",
				Value: *v1beta1.NewStructuredValues("$(context.pipelineRun.name)"),
			}},
		},
	}

	fnNotMatching, err := resources.GetPipelineFunc(ctx, nil, nil, requester, &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pipeline",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRefNotMatching,
			ServiceAccountName: "default",
			Params: []v1beta1.Param{{
				Name:  "resolver-param",
				Value: *v1beta1.NewStructuredValues("banana"),
			}},
		},
	})
	if err != nil {
		t.Fatalf("failed to get pipeline fn: %s", err.Error())
	}

	_, _, err = fnNotMatching(ctx, pipelineRefNotMatching.Name)
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
	fn, err := resources.GetPipelineFunc(ctx, nil, nil, requester, &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        pipelineRef,
			ServiceAccountName: "default",
		},
	})
	if err != nil {
		t.Fatalf("failed to get pipeline fn: %s", err.Error())
	}
	if _, _, err := fn(ctx, pipelineRef.Name); err == nil {
		t.Fatalf("expected error due to invalid pipeline data but saw none")
	}
}

func TestLocalPipelineRef_TrustedResourceVerification_Success(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	signer, secretpath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")
	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, signer, "test-signed")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}

	// attack another signed pipeline
	signedPipeline2, err := test.GetSignedPipeline(test.GetUnsignedPipeline("test-pipeline2"), signer, "test-signed2")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	tamperedPipeline := signedPipeline2.DeepCopy()
	if tamperedPipeline.Annotations == nil {
		tamperedPipeline.Annotations = make(map[string]string)
	}
	tamperedPipeline.Annotations["random"] = "attack"

	tektonclient := fake.NewSimpleClientset(signedPipeline, unsignedPipeline, tamperedPipeline)

	testcases := []struct {
		name                     string
		ref                      *v1beta1.PipelineRef
		resourceVerificationMode string
		expected                 runtime.Object
	}{
		{
			name: "local signed pipeline with enforce policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-signed",
			},
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 signedPipeline,
		}, {
			name: "local unsigned pipeline with warn policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-pipeline",
			},
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 unsignedPipeline,
		},
		{
			name: "local signed pipeline with warn policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-signed",
			},
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 signedPipeline,
		}, {
			name: "local tampered pipeline with warn policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-signed2",
			},
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 tamperedPipeline,
		}, {
			name: "local unsigned pipeline with skip policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-pipeline",
			},
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 unsignedPipeline,
		},
		{
			name: "local signed pipeline with skip policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-signed",
			},
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 signedPipeline,
		}, {
			name: "local tampered pipeline with skip policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-signed2",
			},
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 tamperedPipeline,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, secretpath, tc.resourceVerificationMode)
			lc := &resources.LocalPipelineRefResolver{
				Namespace:    "trusted-resources",
				Tektonclient: tektonclient,
			}

			pipeline, source, err := lc.GetPipeline(ctx, tc.ref.Name)
			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}
			if d := cmp.Diff(pipeline, tc.expected); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if source != nil {
				t.Errorf("expected source is nil, but got %v", source)
			}
		})
	}
}

func TestLocalPipelineRef_TrustedResourceVerification_Error(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	signer, secretpath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")
	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, signer, "test-signed")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}

	// attack another signed pipeline
	signedPipeline2, err := test.GetSignedPipeline(test.GetUnsignedPipeline("test-pipeline2"), signer, "test-signed2")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	tamperedPipeline := signedPipeline2.DeepCopy()
	if tamperedPipeline.Annotations == nil {
		tamperedPipeline.Annotations = make(map[string]string)
	}
	tamperedPipeline.Annotations["random"] = "attack"

	tektonclient := fake.NewSimpleClientset(signedPipeline, unsignedPipeline, tamperedPipeline)

	testcases := []struct {
		name                     string
		ref                      *v1beta1.PipelineRef
		resourceVerificationMode string
		expected                 runtime.Object
		expectedErr              error
	}{
		{
			name: "local unsigned pipeline with enforce policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-pipeline",
			},
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 nil,
			expectedErr:              trustedresources.ErrorResourceVerificationFailed,
		},
		{
			name: "local tampered pipeline with enforce policy",
			ref: &v1beta1.PipelineRef{
				Name: "test-signed2",
			},
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 nil,
			expectedErr:              trustedresources.ErrorResourceVerificationFailed,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, secretpath, tc.resourceVerificationMode)
			lc := &resources.LocalPipelineRefResolver{
				Namespace:    "trusted-resources",
				Tektonclient: tektonclient,
			}

			pipeline, source, err := lc.GetPipeline(ctx, tc.ref.Name)
			if err == nil || !errors.Is(err, tc.expectedErr) {
				t.Fatalf("Expected error %v but found %v instead", tc.expectedErr, err)
			}
			if d := cmp.Diff(pipeline, tc.expected); d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			if source != nil {
				t.Errorf("expected source is nil, but got %v", source)
			}
		})
	}
}

func TestGetPipelineFunc_RemoteResolution_TrustedResourceVerification_Success(t *testing.T) {
	ctx := context.Background()
	signer, secretpath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")
	unsignedPipelineBytes, err := json.Marshal(unsignedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedPipelineBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineBytes, err := json.Marshal(signedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}

	resolvedSigned := test.NewResolvedResource(signedPipelineBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterSigned := test.NewRequester(resolvedSigned, nil)

	tamperedPipeline := signedPipeline.DeepCopy()
	tamperedPipeline.Annotations["random"] = "attack"
	tamperedPipelineBytes, err := json.Marshal(tamperedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	resolvedTampered := test.NewResolvedResource(tamperedPipelineBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterTampered := test.NewRequester(resolvedTampered, nil)

	pipelineRef := &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                     string
		requester                *test.Requester
		resourceVerificationMode string
		expected                 runtime.Object
	}{
		{
			name:                     "signed pipeline with enforce policy",
			requester:                requesterSigned,
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 signedPipeline,
		}, {
			name:                     "unsigned pipeline with warn policy",
			requester:                requesterUnsigned,
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 unsignedPipeline,
		}, {
			name:                     "signed pipeline with warn policy",
			requester:                requesterSigned,
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 signedPipeline,
		}, {
			name:                     "tampered pipeline with warn policy",
			requester:                requesterTampered,
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 tamperedPipeline,
		}, {
			name:                     "unsigned pipeline with skip policy",
			requester:                requesterUnsigned,
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 unsignedPipeline,
		}, {
			name:                     "signed pipeline with skip policy",
			requester:                requesterSigned,
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 signedPipeline,
		}, {
			name:                     "tampered pipeline with skip policy",
			requester:                requesterTampered,
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 tamperedPipeline,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, secretpath, tc.resourceVerificationMode)
			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        pipelineRef,
					ServiceAccountName: "default",
				},
			}
			fn, err := resources.GetPipelineFunc(ctx, nil, nil, tc.requester, pr)
			if err != nil {
				t.Fatalf("failed to get pipeline fn: %s", err.Error())
			}

			resolvedPipeline, source, err := fn(ctx, pipelineRef.Name)
			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}
			if d := cmp.Diff(tc.expected, resolvedPipeline); d != "" {
				t.Error(d)
			}
			if d := cmp.Diff(sampleConfigSource, source); d != "" {
				t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetPipelineFunc_RemoteResolution_TrustedResourceVerification_Error(t *testing.T) {
	ctx := context.Background()
	signer, secretpath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")
	unsignedPipelineBytes, err := json.Marshal(unsignedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedPipelineBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}

	tamperedPipeline := signedPipeline.DeepCopy()
	tamperedPipeline.Annotations["random"] = "attack"
	tamperedPipelineBytes, err := json.Marshal(tamperedPipeline)
	if err != nil {
		t.Fatal("fail to marshal pipeline", err)
	}
	resolvedTampered := test.NewResolvedResource(tamperedPipelineBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterTampered := test.NewRequester(resolvedTampered, nil)

	pipelineRef := &v1beta1.PipelineRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                     string
		requester                *test.Requester
		resourceVerificationMode string
		expected                 runtime.Object
		expectedErr              error
	}{
		{
			name:                     "unsigned pipeline with enforce policy",
			requester:                requesterUnsigned,
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 nil,
			expectedErr:              trustedresources.ErrorResourceVerificationFailed,
		}, {
			name:                     "tampered pipeline with enforce policy",
			requester:                requesterTampered,
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 nil,
			expectedErr:              trustedresources.ErrorResourceVerificationFailed,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, secretpath, tc.resourceVerificationMode)
			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        pipelineRef,
					ServiceAccountName: "default",
				},
			}
			fn, err := resources.GetPipelineFunc(ctx, nil, nil, tc.requester, pr)
			if err != nil {
				t.Fatalf("failed to get pipeline fn: %s", err.Error())
			}

			resolvedPipeline, source, err := fn(ctx, pipelineRef.Name)
			if err == nil || !errors.Is(err, tc.expectedErr) {
				t.Fatalf("Expected error %v but found %v instead", tc.expectedErr, err)
			}
			if d := cmp.Diff(tc.expected, resolvedPipeline); d != "" {
				t.Error(d)
			}
			if source != nil {
				t.Errorf("expected source is nil, but got %v", source)
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

func simplePipelineWithSpecParamAndKind(pt v1beta1.ParamType, tk v1beta1.TaskKind) *v1beta1.Pipeline {
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
