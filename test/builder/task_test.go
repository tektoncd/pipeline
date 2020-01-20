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
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
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
			tb.InputsResource("optional_workspace", v1alpha1.PipelineResourceTypeGit, tb.ResourceOptional(true)),
			tb.InputsParamSpec("param", v1alpha1.ParamTypeString, tb.ParamSpecDescription("mydesc"), tb.ParamSpecDefault("default")),
			tb.InputsParamSpec("array-param", v1alpha1.ParamTypeString, tb.ParamSpecDescription("desc"), tb.ParamSpecDefault("array", "values")),
		),
		tb.TaskOutputs(
			tb.OutputsResource("myotherimage", v1alpha1.PipelineResourceTypeImage),
			tb.OutputsResource("myoptionalimage", v1alpha1.PipelineResourceTypeImage, tb.ResourceOptional(true)),
		),
		tb.Step("myimage", tb.StepName("mycontainer"), tb.StepCommand("/mycmd"), tb.StepArgs(
			"--my-other-arg=$(inputs.resources.workspace.url)",
		)),
		tb.Step("myimage2", tb.StepScript("echo foo")),
		tb.TaskVolume("foo", tb.VolumeSource(corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/foo/bar"},
		})),
		tb.TaskStepTemplate(
			tb.EnvVar("FRUIT", "BANANA"),
		),
		tb.TaskWorkspace("bread", "kind of bread", "/bread/path", false),
	))
	expectedTask := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "test-task", Namespace: "foo"},
		Spec: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "mycontainer",
				Image:   "myimage",
				Command: []string{"/mycmd"},
				Args:    []string{"--my-other-arg=$(inputs.resources.workspace.url)"},
			}}, {Script: "echo foo", Container: corev1.Container{
				Image: "myimage2",
			}}},
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					ResourceDeclaration: v1alpha1.ResourceDeclaration{
						Name:       "workspace",
						Type:       v1alpha1.PipelineResourceTypeGit,
						TargetPath: "/foo/bar",
					}}, {
					ResourceDeclaration: v1alpha1.ResourceDeclaration{
						Name:       "optional_workspace",
						Type:       v1alpha1.PipelineResourceTypeGit,
						TargetPath: "",
						Optional:   true,
					}}},
				Params: []v1alpha1.ParamSpec{{
					Name:        "param",
					Type:        v1alpha1.ParamTypeString,
					Description: "mydesc",
					Default:     tb.ArrayOrString("default"),
				}, {
					Name:        "array-param",
					Type:        v1alpha1.ParamTypeString,
					Description: "desc",
					Default:     tb.ArrayOrString("array", "values"),
				}}},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					ResourceDeclaration: v1alpha1.ResourceDeclaration{
						Name: "myotherimage",
						Type: v1alpha1.PipelineResourceTypeImage,
					}}, {
					ResourceDeclaration: v1alpha1.ResourceDeclaration{
						Name:       "myoptionalimage",
						Type:       v1alpha1.PipelineResourceTypeImage,
						TargetPath: "",
						Optional:   true,
					}}},
			},
			Volumes: []corev1.Volume{{
				Name: "foo",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/foo/bar"},
				},
			}},
			StepTemplate: &corev1.Container{
				Env: []corev1.EnvVar{{
					Name:  "FRUIT",
					Value: "BANANA",
				}},
			},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:        "bread",
				Description: "kind of bread",
				MountPath:   "/bread/path",
				ReadOnly:    false,
			}},
		},
	}
	if d := cmp.Diff(expectedTask, task); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}

func TestClusterTask(t *testing.T) {
	task := tb.ClusterTask("test-clustertask", tb.ClusterTaskSpec(
		tb.Step("myimage", tb.StepCommand("/mycmd"), tb.StepArgs(
			"--my-other-arg=$(inputs.resources.workspace.url)",
		)),
	))
	expectedTask := &v1alpha1.ClusterTask{
		ObjectMeta: metav1.ObjectMeta{Name: "test-clustertask"},
		Spec: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Image:   "myimage",
				Command: []string{"/mycmd"},
				Args:    []string{"--my-other-arg=$(inputs.resources.workspace.url)"},
			}}},
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
		tb.TaskRunLabels(map[string]string{"label-2": "label-value-2", "label-3": "label-value-3"}),
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
				tb.TaskRunInputsParam("arrayparam", "array", "values"),
			),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource(gitResource.Name,
					tb.TaskResourceBindingRef(gitResource.Name),
					tb.TaskResourceBindingPaths("output-folder"),
				),
			),
			tb.TaskRunWorkspaceEmptyDir("bread", "path"),
			tb.TaskRunWorkspacePVC("pizza", "", "pool-party"),
		),
		tb.TaskRunStatus(
			tb.PodName("my-pod-name"),
			tb.StatusCondition(apis.Condition{Type: apis.ConditionSucceeded}),
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
			Labels: map[string]string{
				"label":   "label-value",
				"label-2": "label-value-2",
				"label-3": "label-value-3",
			},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
						Name: "git-resource",
						ResourceRef: &v1alpha1.PipelineResourceRef{
							Name:       "my-git",
							APIVersion: "a1",
						},
					},
					Paths: []string{"source-folder"},
				}, {
					PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
						Name:         "another-git-resource",
						ResourceSpec: &v1alpha1.PipelineResourceSpec{Type: v1alpha1.PipelineResourceType("cluster")},
					},
					Paths: []string{"source-folder"},
				}},
				Params: []v1alpha1.Param{{
					Name:  "iparam",
					Value: *tb.ArrayOrString("ivalue"),
				}, {
					Name:  "arrayparam",
					Value: *tb.ArrayOrString("array", "values"),
				}},
			},
			Outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
						Name: "git-resource",
						ResourceRef: &v1alpha1.PipelineResourceRef{
							Name: "git-resource",
						},
					},
					Paths: []string{"output-folder"},
				}},
			},
			Timeout: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			TaskRef: &v1alpha1.TaskRef{
				Name:       "task-output",
				Kind:       v1alpha1.ClusterTaskKind,
				APIVersion: "a1",
			},
			Workspaces: []v1alpha1.WorkspaceBinding{{
				Name:     "bread",
				SubPath:  "path",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}, {
				Name:    "pizza",
				SubPath: "",
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "pool-party",
				},
			}},
		},
		Status: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
			TaskRunStatusFields: v1alpha1.TaskRunStatusFields{
				PodName: "my-pod-name",
				Steps: []v1alpha1.StepState{{ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 127},
				}}},
			},
		},
	}
	if d := cmp.Diff(expectedTaskRun, taskRun); d != "" {
		t.Fatalf("TaskRun diff -want, +got: %v", d)
	}
}

func TestTaskRunWithTaskSpec(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskSpec(
			tb.Step("image", tb.StepCommand("/mycmd")),
			tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit, tb.ResourceOptional(true))),
		),
		tb.TaskRunServiceAccountName("sa"),
		tb.TaskRunTimeout(2*time.Minute),
		tb.TaskRunSpecStatus(v1alpha1.TaskRunSpecStatusCancelled),
	))
	expectedTaskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-taskrun", Namespace: "foo",
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []v1alpha1.Step{{Container: corev1.Container{
					Image:   "image",
					Command: []string{"/mycmd"},
				}}},
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name:     "workspace",
							Type:     v1alpha1.PipelineResourceTypeGit,
							Optional: true,
						}}},
					Params: nil,
				},
			},
			ServiceAccountName: "sa",
			Status:             v1alpha1.TaskRunSpecStatusCancelled,
			Timeout:            &metav1.Duration{Duration: 2 * time.Minute},
		},
	}
	if d := cmp.Diff(expectedTaskRun, taskRun); d != "" {
		t.Fatalf("TaskRun diff -want, +got: %v", d)
	}
}

func TestTaskRunWithPodTemplate(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskSpec(
			tb.Step("image", tb.StepCommand("/mycmd")),
			tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit, tb.ResourceOptional(true))),
		),
		tb.TaskRunServiceAccountName("sa"),
		tb.TaskRunTimeout(2*time.Minute),
		tb.TaskRunSpecStatus(v1alpha1.TaskRunSpecStatusCancelled),
		tb.TaskRunNodeSelector(map[string]string{
			"label": "value",
		}),
	))
	expectedTaskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-taskrun", Namespace: "foo",
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []v1alpha1.Step{{Container: corev1.Container{
					Image:   "image",
					Command: []string{"/mycmd"},
				}}},
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name:     "workspace",
							Type:     v1alpha1.PipelineResourceTypeGit,
							Optional: true,
						}}},
					Params: nil,
				},
			},
			PodTemplate: &v1alpha1.PodTemplate{
				NodeSelector: map[string]string{
					"label": "value",
				},
			},
			ServiceAccountName: "sa",
			Status:             v1alpha1.TaskRunSpecStatusCancelled,
			Timeout:            &metav1.Duration{Duration: 2 * time.Minute},
		},
	}
	if d := cmp.Diff(expectedTaskRun, taskRun); d != "" {
		t.Fatalf("TaskRun diff -want, +got: %v", d)
	}
}

func TestResolvedTaskResources(t *testing.T) {
	resolvedTaskResources := tb.ResolvedTaskResources(
		tb.ResolvedTaskResourcesTaskSpec(
			tb.Step("image", tb.StepCommand("/mycmd")),
		),
		tb.ResolvedTaskResourcesInputs("foo", tb.PipelineResource("bar", "baz")),
		tb.ResolvedTaskResourcesOutputs("qux", tb.PipelineResource("quux", "quuz")),
	)
	expectedResolvedTaskResources := &resources.ResolvedTaskResources{
		TaskSpec: &v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Image:   "image",
				Command: []string{"/mycmd"},
			}}},
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
