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

package builder_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineResource(t *testing.T) {
	pipelineResource := tb.PipelineResource("git-resource", tb.PipelineResourceNamespace("foo"), tb.PipelineResourceSpec(
		v1beta1.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foo.git"), tb.PipelineResourceDescription("test description"),
	))
	expectedPipelineResource := &resource.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{Name: "git-resource", Namespace: "foo"},
		Spec: resource.PipelineResourceSpec{
			Description: "test description",
			Type:        v1beta1.PipelineResourceTypeGit,
			Params: []resource.ResourceParam{{
				Name: "URL", Value: "https://foo.git",
			}},
		},
	}
	if d := cmp.Diff(expectedPipelineResource, pipelineResource); d != "" {
		t.Fatalf("PipelineResource diff -want, +got: %v", d)
	}
}
