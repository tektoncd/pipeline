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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	simpleTask = v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Task",
			APIVersion: "tekton.dev/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{
					Name:    "step-1",
					Command: []string{"echo"},
					Args:    []string{"hello", "world"},
				},
			}},
		},
	}
)

func TestTaskRunResolutionRequest_ResolveTaskRef(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: simpleTask.Name,
			},
		},
	}

	assets, cancel := getClients(t, nil, test.Data{
		Tasks:    []*v1beta1.Task{simpleTask.DeepCopy()},
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	req := &TaskRunResolutionRequest{
		KubeClientSet:     assets.Clients.Kube,
		PipelineClientSet: assets.Clients.Pipeline,
		TaskRun:           tr,
	}

	if err := req.Resolve(assets.Ctx); err != nil {
		t.Fatalf("unexpected error resolving spec: %v", err)
	}

	if d := cmp.Diff(&simpleTask.Spec, req.ResolvedTaskSpec); d != "" {
		t.Errorf("%s", diff.PrintWantGot(d))
	}
}

func TestTaskRunResolutionRequest_ResolveInlineTaskSpec(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: simpleTask.Spec.DeepCopy(),
		},
	}

	assets, cancel := getClients(t, nil, test.Data{
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	req := &TaskRunResolutionRequest{
		KubeClientSet:     assets.Clients.Kube,
		PipelineClientSet: assets.Clients.Pipeline,
		TaskRun:           tr,
	}

	if err := req.Resolve(assets.Ctx); err != nil {
		t.Fatalf("unexpected error resolving spec: %v", err)
	}

	if d := cmp.Diff(&simpleTask.Spec, req.ResolvedTaskSpec); d != "" {
		t.Errorf("%s", diff.PrintWantGot(d))
	}
}

func TestTaskRunResolutionRequest_ResolveBundle(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Upload the simple task to the registry for our taskRunBundle TaskRun.
	ref, err := test.CreateImage(u.Host+"/"+simpleTask.Name, &simpleTask)
	if err != nil {
		t.Fatalf("failed to upload image with simple task: %s", err.Error())
	}

	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name:   simpleTask.Name,
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
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	req := &TaskRunResolutionRequest{
		KubeClientSet:     nil,
		PipelineClientSet: assets.Clients.Pipeline,
		TaskRun:           tr,
	}

	if err := req.Resolve(assets.Ctx); err != nil {
		t.Fatalf("unexpected error resolving spec: %v", err)
	}

	if d := cmp.Diff(&simpleTask.Spec, req.ResolvedTaskSpec); d != "" {
		t.Errorf("%s", diff.PrintWantGot(d))
	}
}

func TestTaskRunResolutionRequest_AlreadyResolved(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: simpleTask.Spec.DeepCopy(),
		},
		Status: v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskSpec: simpleTask.Spec.DeepCopy(),
			},
		},
	}

	req := &TaskRunResolutionRequest{
		TaskRun: tr,
	}

	err := req.Resolve(context.Background())
	if err != ErrorTaskRunAlreadyResolved {
		t.Fatalf("unexpected error resolving spec: %v", err)
	}
}
