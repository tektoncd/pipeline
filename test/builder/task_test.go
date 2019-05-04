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
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	gitResource = tb.PipelineResource("git-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foo.git"),
	))
	anotherGitResource = tb.PipelineResource("another-git-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foobar.git"),
	))
)

func TestTask(t *testing.T) {
	task := tb.Task("test-task", "foo", tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("/foo/bar")),
			tb.InputsParam("param", tb.ParamDescription("mydesc"), tb.ParamDefault("default")),
		),
		tb.TaskOutputs(tb.OutputsResource("myotherimage", v1alpha1.PipelineResourceTypeImage)),
		tb.Step("mycontainer", "myimage", tb.Command("/mycmd"), tb.Args(
			"--my-other-arg=${inputs.resources.workspace.url}",
		)),
		tb.TaskVolume("foo", tb.VolumeSource(corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/foo/bar"},
		})),
		tb.TaskContainerTemplate(
			tb.EnvVar("FRUIT", "BANANA"),
		),
	))
	expectedTask := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "test-task", Namespace: "foo"},
		Spec: v1alpha1.TaskSpec{
			Steps: []corev1.Container{{
				Name:    "mycontainer",
				Image:   "myimage",
				Command: []string{"/mycmd"},
				Args:    []string{"--my-other-arg=${inputs.resources.workspace.url}"},
			}},
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name:       "workspace",
					Type:       v1alpha1.PipelineResourceTypeGit,
					TargetPath: "/foo/bar",
				}},
				Params: []v1alpha1.TaskParam{{Name: "param", Description: "mydesc", Default: "default"}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "myotherimage",
					Type: v1alpha1.PipelineResourceTypeImage,
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "foo",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/foo/bar"},
				},
			}},
			ContainerTemplate: &corev1.Container{
				Env: []corev1.EnvVar{{
					Name:  "FRUIT",
					Value: "BANANA",
				}},
			},
		},
	}
	if d := cmp.Diff(expectedTask, task); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}

func TestClusterTask(t *testing.T) {
	task := tb.ClusterTask("test-clustertask", tb.ClusterTaskSpec(
		tb.Step("mycontainer", "myimage", tb.Command("/mycmd"), tb.Args(
			"--my-other-arg=${inputs.resources.workspace.url}",
		)),
	))
	expectedTask := &v1alpha1.ClusterTask{
		ObjectMeta: metav1.ObjectMeta{Name: "test-clustertask"},
		Spec: v1alpha1.TaskSpec{
			Steps: []corev1.Container{{
				Name:    "mycontainer",
				Image:   "myimage",
				Command: []string{"/mycmd"},
				Args:    []string{"--my-other-arg=${inputs.resources.workspace.url}"},
			}},
		},
	}
	if d := cmp.Diff(expectedTask, task); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}

func TestTaskRunWithTaskRef(t *testing.T) {
	var trueB = true
	taskRun := tb.TaskRun("test-taskrun", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test",
			tb.OwnerReferenceAPIVersion("a1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("label", "label-value"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("task-output",
				tb.TaskRefKind(v1alpha1.ClusterTaskKind),
				tb.TaskRefAPIVersion("a1"),
			),
			tb.TaskRunInputs(
				tb.TaskRunInputsResource(gitResource.Name,
					tb.TaskResourceBindingRef("my-git"),
					tb.TaskResourceBindingPaths("source-folder"),
					tb.TaskResourceBindingRefAPIVersion("a1"),
				),
				tb.TaskRunInputsResource(anotherGitResource.Name,
					tb.TaskResourceBindingPaths("source-folder"),
					tb.TaskResourceBindingResourceSpec(&v1alpha1.PipelineResourceSpec{Type: v1alpha1.PipelineResourceTypeCluster}),
				),
				tb.TaskRunInputsParam("iparam", "ivalue"),
			),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource(gitResource.Name,
					tb.TaskResourceBindingPaths("output-folder"),
				),
			),
		),
		tb.TaskRunStatus(
			tb.PodName("my-pod-name"),
			tb.Condition(apis.Condition{Type: apis.ConditionSucceeded}),
			tb.StepState(tb.StateTerminated(127)),
		),
	)
	expectedTaskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-taskrun", Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{{
				Name:               "test",
				Kind:               "PipelineRun",
				APIVersion:         "a1",
				Controller:         &trueB,
				BlockOwnerDeletion: &trueB,
			}},
			Labels:      map[string]string{"label": "label-value"},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					Name: "git-resource",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name:       "my-git",
						APIVersion: "a1",
					},
					Paths: []string{"source-folder"},
				}, {
					Name:         "another-git-resource",
					ResourceSpec: &v1alpha1.PipelineResourceSpec{Type: v1alpha1.PipelineResourceType("cluster")},
					Paths:        []string{"source-folder"},
				}},
				Params: []v1alpha1.Param{{Name: "iparam", Value: "ivalue"}},
			},
			Outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					Name: "git-resource",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "git-resource",
					},
					Paths: []string{"output-folder"},
				}},
			},
			TaskRef: &v1alpha1.TaskRef{
				Name:       "task-output",
				Kind:       v1alpha1.ClusterTaskKind,
				APIVersion: "a1",
			},
		},
		Status: v1alpha1.TaskRunStatus{
			PodName: "my-pod-name",
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
			Steps: []v1alpha1.StepState{{ContainerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{ExitCode: 127},
			}}},
		},
	}
	if d := cmp.Diff(expectedTaskRun, taskRun); d != "" {
		t.Fatalf("TaskRun diff -want, +got: %v", d)
	}
}

func TestTaskRunWithTaskSpec(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskSpec(
			tb.Step("step", "image", tb.Command("/mycmd")),
		),
		tb.TaskRunServiceAccount("sa"),
		tb.TaskRunTimeout(2*time.Minute),
	))
	expectedTaskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-taskrun", Namespace: "foo",
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []corev1.Container{{
					Name:    "step",
					Image:   "image",
					Command: []string{"/mycmd"},
				}},
			},
			ServiceAccount: "sa",
			Timeout:        &metav1.Duration{Duration: 2 * time.Minute},
		},
	}
	if d := cmp.Diff(expectedTaskRun, taskRun); d != "" {
		t.Fatalf("TaskRun diff -want, +got: %v", d)
	}
}

func TestResolvedTaskResources(t *testing.T) {
	resolvedTaskResources := tb.ResolvedTaskResources(
		tb.ResolvedTaskResourcesTaskSpec(
			tb.Step("step", "image", tb.Command("/mycmd")),
		),
		tb.ResolvedTaskResourcesInputs("foo", tb.PipelineResource("bar", "baz")),
		tb.ResolvedTaskResourcesOutputs("qux", tb.PipelineResource("quux", "quuz")),
	)
	expectedResolvedTaskResources := &resources.ResolvedTaskResources{
		TaskSpec: &v1alpha1.TaskSpec{
			Steps: []corev1.Container{{
				Name:    "step",
				Image:   "image",
				Command: []string{"/mycmd"},
			}},
		},
		Inputs: map[string]*v1alpha1.PipelineResource{
			"foo": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: "baz",
				},
			},
		},
		Outputs: map[string]*v1alpha1.PipelineResource{
			"qux": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "quux",
					Namespace: "quuz",
				},
			},
		},
	}
	if d := cmp.Diff(expectedResolvedTaskResources, resolvedTaskResources); d != "" {
		t.Fatalf("ResolvedTaskResources diff -want, +got: %v", d)
	}
}
