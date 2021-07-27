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
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/registry"
	ta "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	logtesting "knative.dev/pkg/logging/testing"
)

var (
	simplePipeline = tb.Pipeline("simple", tb.PipelineType, tb.PipelineNamespace("default"))
	dummyPipeline  = tb.Pipeline("dummy", tb.PipelineType, tb.PipelineNamespace("default"))
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
			pipelines: []runtime.Object{simplePipeline, dummyPipeline},
			ref: &v1beta1.PipelineRef{
				Name: "simple",
			},
			expected: simplePipeline,
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

			task, err := lc.GetPipeline(ctx, tc.ref.Name)
			if tc.wantErr && err == nil {
				t.Fatal("Expected error but found nil instead")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(task, tc.expected); tc.expected != nil && d != "" {
				t.Error(diff.PrintWantGot(d))
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
			tb.Pipeline("simple", tb.PipelineType, tb.PipelineNamespace("default"), tb.PipelineSpec(tb.PipelineTask("something", "something"))),
			dummyPipeline,
		},
		remotePipelines: []runtime.Object{simplePipeline, dummyPipeline},
		ref: &v1beta1.PipelineRef{
			Name:   "simple",
			Bundle: u.Host + "/remote-pipeline",
		},
		expected: simplePipeline,
	}, {
		name: "local-pipeline",
		localPipelines: []runtime.Object{
			tb.Pipeline("simple", tb.PipelineType, tb.PipelineNamespace("default"), tb.PipelineSpec(tb.PipelineTask("something", "something"))),
			dummyPipeline,
		},
		remotePipelines: []runtime.Object{simplePipeline, dummyPipeline},
		ref: &v1beta1.PipelineRef{
			Name: "simple",
		},
		expected: tb.Pipeline("simple", tb.PipelineType, tb.PipelineNamespace("default"), tb.PipelineSpec(tb.PipelineTask("something", "something"))),
	}, {
		name:           "remote-pipeline-without-defaults",
		localPipelines: []runtime.Object{simplePipeline},
		remotePipelines: []runtime.Object{
			tb.Pipeline("simple", tb.PipelineType, tb.PipelineNamespace("default"),
				tb.PipelineSpec(tb.PipelineTask("something", "something"), tb.PipelineParamSpec("foo", ""))),
			dummyPipeline},
		ref: &v1beta1.PipelineRef{
			Name:   "simple",
			Bundle: u.Host + "/remote-pipeline-without-defaults",
		},
		expected: tb.Pipeline("simple", tb.PipelineType, tb.PipelineNamespace("default"),
			tb.PipelineSpec(tb.PipelineTask("something", "something", tb.PipelineTaskRefKind(v1beta1.NamespacedTaskKind)), tb.PipelineParamSpec("foo", v1beta1.ParamTypeString))),
	}, {
		name:           "remote-v1alpha1-pipeline-without-defaults",
		localPipelines: []runtime.Object{simplePipeline},
		remotePipelines: []runtime.Object{
			ta.Pipeline("simple", ta.PipelineType, ta.PipelineNamespace("default"),
				ta.PipelineSpec(ta.PipelineTask("something", "something"), ta.PipelineParamSpec("foo", "")))},
		ref: &v1alpha1.PipelineRef{
			Name:   "simple",
			Bundle: u.Host + "/remote-v1alpha1-pipeline-without-defaults",
		},
		expected: tb.Pipeline("simple", tb.PipelineNamespace("default"),
			tb.PipelineSpec(tb.PipelineTask("something", "something", tb.PipelineTaskRefKind(v1beta1.NamespacedTaskKind)), tb.PipelineParamSpec("foo", v1beta1.ParamTypeString))),
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

			fn, err := resources.GetPipelineFunc(ctx, kubeclient, tektonclient, &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        tc.ref,
					ServiceAccountName: "default",
				},
			})
			if err != nil {
				t.Fatalf("failed to get pipeline fn: %s", err.Error())
			}

			pipeline, err := fn(ctx, tc.ref.Name)
			if err != nil {
				t.Fatalf("failed to call pipelinefn: %s", err.Error())
			}

			if diff := cmp.Diff(pipeline, tc.expected); tc.expected != nil && diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestGetPipelineFuncSpecAlreadyFetched(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	tektonclient := fake.NewSimpleClientset(simplePipeline, dummyPipeline)
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
		}},
	}
	expectedPipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: pipelineSpec,
	}

	fn, err := resources.GetPipelineFunc(ctx, kubeclient, tektonclient, pipelineRun)
	if err != nil {
		t.Fatalf("failed to get pipeline fn: %s", err.Error())
	}
	actualPipeline, err := fn(ctx, name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if diff := cmp.Diff(actualPipeline, expectedPipeline); expectedPipeline != nil && diff != "" {
		t.Error(diff)
	}
}
