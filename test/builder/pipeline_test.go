/*
Copyright 2018 The Knative Authors
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
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/knative/build-pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipeline(t *testing.T) {
	pipeline := tb.Pipeline("tomatoes", "foo",
		tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
	)
	expectedPipeline := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "tomatoes", Namespace: "foo"},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "foo",
				TaskRef: v1alpha1.TaskRef{Name: "banana"},
			}},
		},
	}
	if d := cmp.Diff(expectedPipeline, pipeline); d != "" {
		t.Fatalf("Pipeline diff -want, +got: %v", d)
	}
}

func TestPipelineRun(t *testing.T) {
	pipelineRun := tb.PipelineRun("pear", "foo",
		tb.PipelineRunSpec("tomatoes", tb.PipelineRunServiceAccount("sa")),
	)
	expectedPipelineRun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pear", Namespace: "foo"},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef:        v1alpha1.PipelineRef{Name: "tomatoes"},
			PipelineTriggerRef: v1alpha1.PipelineTriggerRef{Type: v1alpha1.PipelineTriggerTypeManual},
			ServiceAccount:     "sa",
		},
	}
	if d := cmp.Diff(expectedPipelineRun, pipelineRun); d != "" {
		t.Fatalf("PipelineRun diff -want, +got: %v", d)
	}
}

func TestPipelineResource(t *testing.T) {
	pipelineResource := tb.PipelineResource("git-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit, tb.PipelineSpecParam("URL", "https://foo.git"),
	))
	expectedPipelineResource := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{Name: "git-resource", Namespace: "foo"},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.Param{{
				Name: "URL", Value: "https://foo.git",
			}},
		},
	}
	if d := cmp.Diff(expectedPipelineResource, pipelineResource); d != "" {
		t.Fatalf("PipelineResource diff -want, +got: %v", d)
	}
}
