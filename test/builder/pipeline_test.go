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

package builder_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestPipeline(t *testing.T) {
	creationTime := time.Now()

	pipeline := tb.Pipeline("tomatoes", "foo", tb.PipelineSpec(
		tb.PipelineDeclaredResource("my-only-git-resource", "git"),
		tb.PipelineDeclaredResource("my-only-image-resource", "image"),
		tb.PipelineParamSpec("first-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("default-value"), tb.ParamSpecDescription("default description")),
		tb.PipelineTask("foo", "banana",
			tb.PipelineTaskParam("stringparam", "value"),
			tb.PipelineTaskParam("arrayparam", "array", "value"),
			tb.PipelineTaskCondition("some-condition-ref",
				tb.PipelineTaskConditionParam("param-name", "param-value"),
				tb.PipelineTaskConditionResource("some-resource", "my-only-git-resource", "bar", "never-gonna"),
			),
			tb.PipelineTaskWorkspaceBinding("task-workspace1", "workspace1"),
		),
		tb.PipelineTask("bar", "chocolate",
			tb.PipelineTaskRefKind(v1alpha1.ClusterTaskKind),
			tb.PipelineTaskInputResource("some-repo", "my-only-git-resource", tb.From("foo")),
			tb.PipelineTaskOutputResource("some-image", "my-only-image-resource"),
		),
		tb.PipelineTask("never-gonna", "give-you-up",
			tb.RunAfter("foo"),
		),
		tb.PipelineTask("foo", "", tb.PipelineTaskSpec(&v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "step",
				Image: "myimage",
			}}}},
		)),
		tb.PipelineWorkspaceDeclaration("workspace1"),
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
			Params: []v1alpha1.ParamSpec{{
				Name:        "first-param",
				Type:        v1alpha1.ParamTypeString,
				Default:     tb.ArrayOrString("default-value"),
				Description: "default description",
			}},
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "foo",
				TaskRef: &v1alpha1.TaskRef{Name: "banana"},
				Params: []v1alpha1.Param{{
					Name:  "stringparam",
					Value: *tb.ArrayOrString("value"),
				}, {
					Name:  "arrayparam",
					Value: *tb.ArrayOrString("array", "value"),
				}},
				Conditions: []v1alpha1.PipelineTaskCondition{{
					ConditionRef: "some-condition-ref",
					Params: []v1alpha1.Param{{
						Name: "param-name",
						Value: v1alpha1.ArrayOrString{
							Type:      "string",
							StringVal: "param-value",
						},
					}},
					Resources: []v1alpha1.PipelineTaskInputResource{{
						Name:     "some-resource",
						Resource: "my-only-git-resource",
						From:     []string{"bar", "never-gonna"},
					}},
				}},
				Workspaces: []v1alpha1.WorkspacePipelineTaskBinding{{
					Name:      "task-workspace1",
					Workspace: "workspace1",
				}},
			}, {
				Name:    "bar",
				TaskRef: &v1alpha1.TaskRef{Name: "chocolate", Kind: v1alpha1.ClusterTaskKind},
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
				TaskRef:  &v1alpha1.TaskRef{Name: "give-you-up"},
				RunAfter: []string{"foo"},
			}, {
				Name: "foo",
				TaskSpec: &v1alpha1.TaskSpec{
					Steps: []v1alpha1.Step{{Container: corev1.Container{
						Name:  "step",
						Image: "myimage",
					}},
					},
				},
			}},
			Workspaces: []v1alpha1.WorkspacePipelineDeclaration{{
				Name: "workspace1",
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
		"tomatoes", tb.PipelineRunServiceAccountName("sa"),
		tb.PipelineRunParam("first-param-string", "first-value"),
		tb.PipelineRunParam("second-param-array", "some", "array"),
		tb.PipelineRunTimeout(1*time.Hour),
		tb.PipelineRunResourceBinding("some-resource", tb.PipelineResourceBindingRef("my-special-resource")),
		tb.PipelineRunServiceAccountNameTask("foo", "sa-2"),
	), tb.PipelineRunStatus(tb.PipelineRunStatusCondition(
		apis.Condition{Type: apis.ConditionSucceeded}),
		tb.PipelineRunStartTime(startTime),
		tb.PipelineRunCompletionTime(completedTime),
		tb.PipelineRunTaskRunsStatus("trname", &v1alpha1.PipelineRunTaskRunStatus{
			PipelineTaskName: "task-1",
		}),
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
			PipelineRef:         &v1alpha1.PipelineRef{Name: "tomatoes"},
			ServiceAccountName:  "sa",
			ServiceAccountNames: []v1alpha1.PipelineRunSpecServiceAccountName{{TaskName: "foo", ServiceAccountName: "sa-2"}},
			Params: []v1alpha1.Param{{
				Name:  "first-param-string",
				Value: *tb.ArrayOrString("first-value"),
			}, {
				Name:  "second-param-array",
				Value: *tb.ArrayOrString("some", "array"),
			}},
			Timeout: &metav1.Duration{Duration: 1 * time.Hour},
			Resources: []v1alpha1.PipelineResourceBinding{{
				Name: "some-resource",
				ResourceRef: &v1alpha1.PipelineResourceRef{
					Name: "my-special-resource",
				},
			}},
		},
		Status: v1alpha1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
			PipelineRunStatusFields: v1alpha1.PipelineRunStatusFields{
				StartTime:      &metav1.Time{Time: startTime},
				CompletionTime: &metav1.Time{Time: completedTime},
				TaskRuns: map[string]*v1alpha1.PipelineRunTaskRunStatus{
					"trname": {PipelineTaskName: "task-1"},
				},
			},
		},
	}
	if d := cmp.Diff(expectedPipelineRun, pipelineRun); d != "" {
		t.Fatalf("PipelineRun diff -want, +got: %v", d)
	}
}

func TestPipelineRunWithPodTemplate(t *testing.T) {
	startTime := time.Now()
	completedTime := startTime.Add(5 * time.Minute)

	pipelineRun := tb.PipelineRun("pear", "foo", tb.PipelineRunSpec(
		"tomatoes", tb.PipelineRunServiceAccountName("sa"),
		tb.PipelineRunParam("first-param-string", "first-value"),
		tb.PipelineRunParam("second-param-array", "some", "array"),
		tb.PipelineRunTimeout(1*time.Hour),
		tb.PipelineRunResourceBinding("some-resource", tb.PipelineResourceBindingRef("my-special-resource")),
		tb.PipelineRunServiceAccountNameTask("foo", "sa-2"),
		tb.PipelineRunNodeSelector(map[string]string{
			"label": "value",
		}),
	), tb.PipelineRunStatus(tb.PipelineRunStatusCondition(
		apis.Condition{Type: apis.ConditionSucceeded}),
		tb.PipelineRunStartTime(startTime),
		tb.PipelineRunCompletionTime(completedTime),
		tb.PipelineRunTaskRunsStatus("trname", &v1alpha1.PipelineRunTaskRunStatus{
			PipelineTaskName: "task-1",
		}),
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
			PipelineRef:         &v1alpha1.PipelineRef{Name: "tomatoes"},
			ServiceAccountName:  "sa",
			ServiceAccountNames: []v1alpha1.PipelineRunSpecServiceAccountName{{TaskName: "foo", ServiceAccountName: "sa-2"}},
			Params: []v1alpha1.Param{{
				Name:  "first-param-string",
				Value: *tb.ArrayOrString("first-value"),
			}, {
				Name:  "second-param-array",
				Value: *tb.ArrayOrString("some", "array"),
			}},
			Timeout: &metav1.Duration{Duration: 1 * time.Hour},
			Resources: []v1alpha1.PipelineResourceBinding{{
				Name: "some-resource",
				ResourceRef: &v1alpha1.PipelineResourceRef{
					Name: "my-special-resource",
				},
			}},
			PodTemplate: &v1alpha1.PodTemplate{
				NodeSelector: map[string]string{
					"label": "value",
				},
			},
		},
		Status: v1alpha1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
			PipelineRunStatusFields: v1alpha1.PipelineRunStatusFields{
				StartTime:      &metav1.Time{Time: startTime},
				CompletionTime: &metav1.Time{Time: completedTime},
				TaskRuns: map[string]*v1alpha1.PipelineRunTaskRunStatus{
					"trname": {PipelineTaskName: "task-1"},
				},
			},
		},
	}
	if d := cmp.Diff(expectedPipelineRun, pipelineRun); d != "" {
		t.Fatalf("PipelineRun diff -want, +got: %v", d)
	}
}

func TestPipelineRunWithResourceSpec(t *testing.T) {
	startTime := time.Now()
	completedTime := startTime.Add(5 * time.Minute)

	pipelineRun := tb.PipelineRun("pear", "foo", tb.PipelineRunSpec(
		"tomatoes", tb.PipelineRunServiceAccountName("sa"),
		tb.PipelineRunParam("first-param-string", "first-value"),
		tb.PipelineRunParam("second-param-array", "some", "array"),
		tb.PipelineRunTimeout(1*time.Hour),
		tb.PipelineRunResourceBinding("some-resource",
			tb.PipelineResourceBindingResourceSpec(&v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeGit,
				Params: []v1alpha1.ResourceParam{{
					Name:  "url",
					Value: "git",
				}}})),
		tb.PipelineRunServiceAccountNameTask("foo", "sa-2"),
	), tb.PipelineRunStatus(tb.PipelineRunStatusCondition(
		apis.Condition{Type: apis.ConditionSucceeded}),
		tb.PipelineRunStartTime(startTime),
		tb.PipelineRunCompletionTime(completedTime),
		tb.PipelineRunTaskRunsStatus("trname", &v1alpha1.PipelineRunTaskRunStatus{
			PipelineTaskName: "task-1",
		}),
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
			PipelineRef:         &v1alpha1.PipelineRef{Name: "tomatoes"},
			ServiceAccountName:  "sa",
			ServiceAccountNames: []v1alpha1.PipelineRunSpecServiceAccountName{{TaskName: "foo", ServiceAccountName: "sa-2"}},
			Params: []v1alpha1.Param{{
				Name:  "first-param-string",
				Value: *tb.ArrayOrString("first-value"),
			}, {
				Name:  "second-param-array",
				Value: *tb.ArrayOrString("some", "array"),
			}},
			Timeout: &metav1.Duration{Duration: 1 * time.Hour},
			Resources: []v1alpha1.PipelineResourceBinding{{
				Name: "some-resource",
				ResourceSpec: &v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceType("git"),
					Params: []v1alpha1.ResourceParam{{
						Name:  "url",
						Value: "git",
					}},
					SecretParams: nil,
				},
			}},
		},
		Status: v1alpha1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
			PipelineRunStatusFields: v1alpha1.PipelineRunStatusFields{
				StartTime:      &metav1.Time{Time: startTime},
				CompletionTime: &metav1.Time{Time: completedTime},
				TaskRuns: map[string]*v1alpha1.PipelineRunTaskRunStatus{
					"trname": {PipelineTaskName: "task-1"},
				},
			},
		},
	}
	if d := cmp.Diff(expectedPipelineRun, pipelineRun); d != "" {
		t.Fatalf("PipelineRun diff -want, +got: %v", d)
	}
}

func TestPipelineRunWithPipelineSpec(t *testing.T) {
	pipelineRun := tb.PipelineRun("pear", "foo", tb.PipelineRunSpec("", tb.PipelineRunPipelineSpec(
		tb.PipelineTask("a-task", "some-task")),
		tb.PipelineRunServiceAccountName("sa"),
	))

	expectedPipelineRun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pear",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: nil,
			PipelineSpec: &v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "a-task",
					TaskRef: &v1alpha1.TaskRef{Name: "some-task"},
				}},
			},
			ServiceAccountName: "sa",
			Timeout:            &metav1.Duration{Duration: 1 * time.Hour},
		},
	}

	if diff := cmp.Diff(expectedPipelineRun, pipelineRun); diff != "" {
		t.Fatalf("PipelineRun diff -want, +got: %s", diff)
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
			Params: []v1alpha1.ResourceParam{{
				Name: "URL", Value: "https://foo.git",
			}},
		},
	}
	if d := cmp.Diff(expectedPipelineResource, pipelineResource); d != "" {
		t.Fatalf("PipelineResource diff -want, +got: %v", d)
	}
}
