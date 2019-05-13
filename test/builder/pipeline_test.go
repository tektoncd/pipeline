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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	duckv1beta1 "github.com/knative/pkg/apis/duck/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipeline(t *testing.T) {
	creationTime := time.Now()

	pipeline := tb.Pipeline("tomatoes", "foo", tb.PipelineSpec(
		tb.PipelineDeclaredResource("my-only-git-resource", "git"),
		tb.PipelineDeclaredResource("my-only-image-resource", "image"),
		tb.PipelineParam("first-param", tb.PipelineParamDefault("default-value"), tb.PipelineParamDescription("default description")),
		tb.PipelineTask("foo", "banana",
			tb.PipelineTaskParam("name", "value"),
		),
		tb.PipelineTask("bar", "chocolate",
			tb.PipelineTaskRefKind(v1alpha1.ClusterTaskKind),
			tb.PipelineTaskInputResource("some-repo", "my-only-git-resource", tb.From("foo")),
			tb.PipelineTaskOutputResource("some-image", "my-only-image-resource"),
		),
		tb.PipelineTask("never-gonna", "give-you-up",
			tb.RunAfter("foo"),
		),
	),
		tb.PipelineCreationTimestamp(creationTime),
	)
	expectedPipeline := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tomatoes", Namespace: "foo",
			CreationTimestamp: metav1.Time{Time: creationTime},
		},
		Spec: v1alpha1.PipelineSpec{
			Resources: []v1alpha1.PipelineDeclaredResource{{
				Name: "my-only-git-resource",
				Type: "git",
			}, {
				Name: "my-only-image-resource",
				Type: "image",
			}},
			Params: []v1alpha1.PipelineParam{{
				Name:        "first-param",
				Default:     "default-value",
				Description: "default description",
			}},
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "foo",
				TaskRef: v1alpha1.TaskRef{Name: "banana"},
				Params:  []v1alpha1.Param{{Name: "name", Value: "value"}},
			}, {
				Name:    "bar",
				TaskRef: v1alpha1.TaskRef{Name: "chocolate", Kind: v1alpha1.ClusterTaskKind},
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name:     "some-repo",
						Resource: "my-only-git-resource",
						From:     []string{"foo"},
					}},
					Outputs: []v1alpha1.PipelineTaskOutputResource{{
						Name:     "some-image",
						Resource: "my-only-image-resource",
					}},
				},
			}, {
				Name:     "never-gonna",
				TaskRef:  v1alpha1.TaskRef{Name: "give-you-up"},
				RunAfter: []string{"foo"},
			}},
		},
	}
	if d := cmp.Diff(expectedPipeline, pipeline); d != "" {
		t.Fatalf("Pipeline diff -want, +got: %v", d)
	}
}

func TestPipelineRun(t *testing.T) {
	startTime := time.Now()
	completedTime := startTime.Add(5 * time.Minute)

	pipelineRun := tb.PipelineRun("pear", "foo", tb.PipelineRunSpec(
		"tomatoes", tb.PipelineRunServiceAccount("sa"),
		tb.PipelineRunParam("first-param", "first-value"),
		tb.PipelineRunTimeout(&metav1.Duration{Duration: 1 * time.Hour}),
		tb.PipelineRunResourceBinding("some-resource", tb.PipelineResourceBindingRef("my-special-resource")),
	), tb.PipelineRunStatus(tb.PipelineRunStatusCondition(
		apis.Condition{Type: apis.ConditionSucceeded}),
		tb.PipelineRunStartTime(startTime),
		tb.PipelineRunCompletionTime(completedTime),
	), tb.PipelineRunLabel("label-key", "label-value"))
	expectedPipelineRun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pear",
			Namespace: "foo",
			Labels: map[string]string{
				"label-key": "label-value",
			},
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef:    v1alpha1.PipelineRef{Name: "tomatoes"},
			ServiceAccount: "sa",
			Params: []v1alpha1.Param{{
				Name:  "first-param",
				Value: "first-value",
			}},
			Timeout: &metav1.Duration{Duration: 1 * time.Hour},
			Resources: []v1alpha1.PipelineResourceBinding{{
				Name: "some-resource",
				ResourceRef: v1alpha1.PipelineResourceRef{
					Name: "my-special-resource",
				},
			}},
		},
		Status: v1alpha1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
			StartTime:      &metav1.Time{Time: startTime},
			CompletionTime: &metav1.Time{Time: completedTime},
		},
	}
	if d := cmp.Diff(expectedPipelineRun, pipelineRun); d != "" {
		t.Fatalf("PipelineRun diff -want, +got: %v", d)
	}
}

func TestPipelineResource(t *testing.T) {
	pipelineResource := tb.PipelineResource("git-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foo.git"),
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
