/*
Copyright 2021 The Tekton Authors

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

package resolution

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	simplePipeline = v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pipeline",
			APIVersion: "tekton.dev/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "test",
				TaskRef: &v1beta1.TaskRef{
					Name: "test-task",
					Kind: "Task",
				},
			}},
		},
	}
)

func TestPipelineRunResolutionRequest_ResolvePipelineRef(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
	}

	assets, cancel := getClients(t, nil, test.Data{
		Pipelines:    []*v1beta1.Pipeline{simplePipeline.DeepCopy()},
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	req := &PipelineRunResolutionRequest{
		KubeClientSet:     assets.Clients.Kube,
		PipelineClientSet: assets.Clients.Pipeline,
		PipelineRun:       pr,
	}

	if err := req.Resolve(assets.Ctx); err != nil {
		t.Fatalf("unexpected error resolving spec: %v", err)
	}

	if d := cmp.Diff(&simplePipeline.Spec, req.ResolvedPipelineSpec); d != "" {
		t.Errorf("%s", diff.PrintWantGot(d))
	}
}

func TestPipelineRunResolutionRequest_ResolveInlinePipelineSpec(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: simplePipeline.Spec.DeepCopy(),
		},
	}

	assets, cancel := getClients(t, nil, test.Data{
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	req := &PipelineRunResolutionRequest{
		KubeClientSet:     assets.Clients.Kube,
		PipelineClientSet: assets.Clients.Pipeline,
		PipelineRun:       pr,
	}

	if err := req.Resolve(assets.Ctx); err != nil {
		t.Fatalf("unexpected error resolving spec: %v", err)
	}

	if d := cmp.Diff(&simplePipeline.Spec, req.ResolvedPipelineSpec); d != "" {
		t.Errorf("%s", diff.PrintWantGot(d))
	}
}

func TestPipelineRunResolutionRequest_ResolveBundle(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Upload the simple pipeline to the registry for our pipelineRunBundle PipelineRun.
	ref, err := test.CreateImage(u.Host+"/"+simplePipeline.Name, &simplePipeline)
	if err != nil {
		t.Fatalf("failed to upload image with simple pipeline: %s", err.Error())
	}

	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name:   simplePipeline.Name,
				Bundle: ref,
			},
		},
	}

	config := config.Config{
		FeatureFlags: &config.FeatureFlags{
			EnableTektonOCIBundles: true,
		},
	}
	assets, cancel := getClients(t, &config, test.Data{
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	req := &PipelineRunResolutionRequest{
		KubeClientSet:     nil,
		PipelineClientSet: assets.Clients.Pipeline,
		PipelineRun:       pr,
	}

	if err := req.Resolve(assets.Ctx); err != nil {
		t.Fatalf("unexpected error resolving spec: %v", err)
	}

	if d := cmp.Diff(&simplePipeline.Spec, req.ResolvedPipelineSpec); d != "" {
		t.Errorf("%s", diff.PrintWantGot(d))
	}
}

func TestPipelineRunResolutionRequest_AlreadyResolved(t *testing.T) {
	tr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: simplePipeline.Spec.DeepCopy(),
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				PipelineSpec: simplePipeline.Spec.DeepCopy(),
			},
		},
	}

	req := &PipelineRunResolutionRequest{
		PipelineRun: tr,
	}

	err := req.Resolve(context.Background())
	if err != ErrorPipelineRunAlreadyResolved {
		t.Fatalf("unexpected error resolving spec: %v", err)
	}
}
