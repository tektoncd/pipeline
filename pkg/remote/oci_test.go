/*
Copyright 2019 The Tekton Authors

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

package remote

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOCIResolver(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, _ := url.Parse(s.URL)

	testcases := []struct {
		name string
		task v1alpha1.TaskInterface
	}{
		{
			name: "simple-task",
			task: tb.Task("hello-world", "", tb.TaskType(), tb.TaskSpec(tb.Step("ubuntu", tb.StepCommand("echo 'Hello'")))),
		},
		{
			name: "cluster-task",
			task: tb.ClusterTask("hello-world", tb.ClusterTaskType(), tb.ClusterTaskSpec(tb.Step("ubuntu", tb.StepCommand("echo 'Hello'")))),
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var data string
			var key string

			raw, err := yaml.Marshal(tc.task)
			if err != nil {
				t.Errorf("failed to marshal task before uploading as image: %w", err)
			}
			data = string(raw)
			key = fmt.Sprintf("task/%s", tc.task.TaskMetadata().Name)

			imgRef, err := test.CreateImage(u.Host, key, data)
			if err != nil {
				t.Errorf("unexpected error pushing task image %w", err)
			}

			// Now we can call our resolver and see if the spec returned is the same.
			resolver := OCIResolver{
				imageReference: imgRef,
				keychain:       authn.DefaultKeychain,
			}

			ta, err := resolver.GetTask(tc.task.TaskMetadata().Name)
			if err != nil {
				t.Errorf("failed to fetch task %s: %w", tc.task.TaskMetadata().Name, err)
			}

			if diff := cmp.Diff(ta, tc.task); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestOCIResolver_BetaTasks(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, _ := url.Parse(s.URL)

	task := tb.Task("hello-world", "", tb.TaskType(), tb.TaskSpec(tb.Step("ubuntu", tb.StepCommand("echo 'Hello'"))))
	clusterTask := tb.ClusterTask("hello-world", tb.ClusterTaskType(), tb.ClusterTaskSpec(tb.Step("ubuntu", tb.StepCommand("echo 'Hello'"))))

	betaTask := v1beta1.Task{
		TypeMeta:   v1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "Task"},
		ObjectMeta: task.ObjectMeta,
		Spec:       task.Spec.TaskSpec,
	}
	betaClusterTask := v1beta1.ClusterTask{
		TypeMeta:   v1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "ClusterTask"},
		ObjectMeta: clusterTask.ObjectMeta,
		Spec:       clusterTask.Spec.TaskSpec,
	}

	testcases := []struct {
		name string
		task v1beta1.TaskInterface
	}{
		{
			name: "beta-task",
			task: &betaTask,
		},
		{
			name: "beta-cluster-task",
			task: &betaClusterTask,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var data string
			var key string

			raw, err := yaml.Marshal(tc.task)
			if err != nil {
				t.Errorf("failed to marshal task before uploading as image: %w", err)
			}
			data = string(raw)
			key = fmt.Sprintf("task/%s", tc.task.TaskMetadata().Name)

			imgRef, err := test.CreateImage(u.Host, key, data)
			if err != nil {
				t.Errorf("unexpected error pushing task image %w", err)
			}

			// Now we can call our resolver and see if the spec returned is the same.
			resolver := OCIResolver{
				imageReference: imgRef,
				keychain:       authn.DefaultKeychain,
			}

			ta, err := resolver.GetTask(tc.task.TaskMetadata().Name)
			if err != nil {
				t.Errorf("failed to fetch task %s: %w", tc.task.TaskMetadata().Name, err)
			}

			if diff := cmp.Diff(ta.TaskSpec(), v1alpha1.TaskSpec{
				TaskSpec: tc.task.TaskSpec(),
			}); diff != "" {
				t.Error(diff)
			}
		})
	}
}
