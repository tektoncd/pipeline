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
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var (
	gitResource = tb.PipelineResource("git-resource", tb.PipelineResourceNamespace("foo"), tb.PipelineResourceSpec(
		resource.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foo.git"),
	))
	anotherGitResource = tb.PipelineResource("another-git-resource", tb.PipelineResourceNamespace("foo"), tb.PipelineResourceSpec(
		resource.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foobar.git"),
	))
)

func TestTask(t *testing.T) {
	task := tb.Task("test-task", tb.TaskType(), tb.TaskSpec(
		tb.TaskParam("param", v1beta1.ParamTypeString, tb.ParamSpecDescription("mydesc"), tb.ParamSpecDefault("default")),
		tb.TaskParam("array-param", v1beta1.ParamTypeString, tb.ParamSpecDescription("desc"), tb.ParamSpecDefault("array", "values")),
		tb.TaskResources(
			tb.TaskResourcesInput("workspace", resource.PipelineResourceTypeGit, tb.ResourceTargetPath("/foo/bar")),
			tb.TaskResourcesInput("optional_workspace", resource.PipelineResourceTypeGit, tb.ResourceOptional(true)),
			tb.TaskResourcesOutput("myotherimage", resource.PipelineResourceTypeImage),
			tb.TaskResourcesOutput("myoptionalimage", resource.PipelineResourceTypeImage, tb.ResourceOptional(true)),
		),
		tb.TaskDescription("Test Task"),
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
	), tb.TaskNamespace("foo"))
	expectedTask := &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test-task", Namespace: "foo"},
		Spec: v1beta1.TaskSpec{
			Description: "Test Task",
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "mycontainer",
				Image:   "myimage",
				Command: []string{"/mycmd"},
				Args:    []string{"--my-other-arg=$(inputs.resources.workspace.url)"},
			}}, {Script: "echo foo", Container: corev1.Container{
				Image: "myimage2",
			}}},
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
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:        "bread",
				Description: "kind of bread",
				MountPath:   "/bread/path",
				ReadOnly:    false,
			}},
			Params: []v1beta1.ParamSpec{{
				Name:        "param",
				Type:        v1beta1.ParamTypeString,
				Description: "mydesc",
				Default:     v1beta1.NewArrayOrString("default"),
			}, {
				Name:        "array-param",
				Type:        v1beta1.ParamTypeString,
				Description: "desc",
				Default:     v1beta1.NewArrayOrString("array", "values"),
			}},
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:       "workspace",
						Type:       resource.PipelineResourceTypeGit,
						TargetPath: "/foo/bar",
					}}, {
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:       "optional_workspace",
						Type:       resource.PipelineResourceTypeGit,
						TargetPath: "",
						Optional:   true,
					}},
				},
				Outputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name: "myotherimage",
						Type: resource.PipelineResourceTypeImage,
					}}, {
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:       "myoptionalimage",
						Type:       resource.PipelineResourceTypeImage,
						TargetPath: "",
						Optional:   true,
					}},
				},
			},
		},
	}
	if d := cmp.Diff(expectedTask, task); d != "" {
		t.Fatalf("Task diff -want, +got: %v", d)
	}
}

func TestClusterTask(t *testing.T) {
	task := tb.ClusterTask("test-clustertask", tb.ClusterTaskType(), tb.ClusterTaskSpec(
		tb.Step("myimage", tb.StepCommand("/mycmd"), tb.StepArgs(
			"--my-other-arg=$(inputs.resources.workspace.url)",
		)),
	))
	expectedTask := &v1beta1.ClusterTask{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "ClusterTask",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test-clustertask"},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
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
	terminatedState := corev1.ContainerStateTerminated{Reason: "Completed"}

	taskRun := tb.TaskRun("test-taskrun",
		tb.TaskRunNamespace("foo"),
		tb.TaskRunOwnerReference("PipelineRun", "test",
			tb.OwnerReferenceAPIVersion("a1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabels(map[string]string{"label-2": "label-value-2", "label-3": "label-value-3"}),
		tb.TaskRunLabel("label", "label-value"),
		tb.TaskRunAnnotations(map[string]string{"annotation-1": "annotation-value-1", "annotation-2": "annotation-value-2"}),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("task-output",
				tb.TaskRefKind(v1beta1.ClusterTaskKind),
				tb.TaskRefAPIVersion("a1"),
			),
			tb.TaskRunParam("iparam", "ivalue"),
			tb.TaskRunParam("arrayparam", "array", "values"),
			tb.TaskRunResources(
				tb.TaskRunResourcesInput(gitResource.Name,
					tb.TaskResourceBindingRef("my-git"),
					tb.TaskResourceBindingPaths("source-folder"),
					tb.TaskResourceBindingRefAPIVersion("a1"),
				),
				tb.TaskRunResourcesInput(anotherGitResource.Name,
					tb.TaskResourceBindingPaths("source-folder"),
					tb.TaskResourceBindingResourceSpec(&resource.PipelineResourceSpec{Type: resource.PipelineResourceTypeCluster}),
				),
				tb.TaskRunResourcesOutput(gitResource.Name,
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
			tb.SidecarState(
				tb.SidecarStateName("sidecar"),
				tb.SidecarStateImageID("ImageID"),
				tb.SidecarStateContainerName("ContainerName"),
				tb.SetSidecarStateTerminated(terminatedState),
			),
		),
	)
	expectedTaskRun := &v1beta1.TaskRun{
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
			Annotations: map[string]string{
				"annotation-1": "annotation-value-1",
				"annotation-2": "annotation-value-2",
			},
		},
		Spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{{
				Name:  "iparam",
				Value: *v1beta1.NewArrayOrString("ivalue"),
			}, {
				Name:  "arrayparam",
				Value: *v1beta1.NewArrayOrString("array", "values"),
			}},
			Resources: &v1beta1.TaskRunResources{
				Inputs: []v1beta1.TaskResourceBinding{{
					PipelineResourceBinding: v1beta1.PipelineResourceBinding{
						Name: "git-resource",
						ResourceRef: &v1beta1.PipelineResourceRef{
							Name:       "my-git",
							APIVersion: "a1",
						},
					},
					Paths: []string{"source-folder"},
				}, {
					PipelineResourceBinding: v1beta1.PipelineResourceBinding{
						Name:         "another-git-resource",
						ResourceSpec: &resource.PipelineResourceSpec{Type: resource.PipelineResourceType("cluster")},
					},
					Paths: []string{"source-folder"},
				}},
				Outputs: []v1beta1.TaskResourceBinding{{
					PipelineResourceBinding: v1beta1.PipelineResourceBinding{
						Name: "git-resource",
						ResourceRef: &v1beta1.PipelineResourceRef{
							Name: "git-resource",
						},
					},
					Paths: []string{"output-folder"},
				}},
			},
			Timeout: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			TaskRef: &v1beta1.TaskRef{
				Name:       "task-output",
				Kind:       v1beta1.ClusterTaskKind,
				APIVersion: "a1",
			},
			Workspaces: []v1beta1.WorkspaceBinding{{
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
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				PodName: "my-pod-name",
				Sidecars: []v1beta1.SidecarState{{
					Name:          "sidecar",
					ImageID:       "ImageID",
					ContainerName: "ContainerName",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason: "Completed",
						},
					},
				}},
				Steps: []v1beta1.StepState{{ContainerState: corev1.ContainerState{
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
	taskRun := tb.TaskRun("test-taskrun",
		tb.TaskRunNamespace("foo"),
		tb.TaskRunSpec(
			tb.TaskRunTaskSpec(
				tb.Step("image", tb.StepCommand("/mycmd")),
				tb.TaskResources(
					tb.TaskResourcesInput("workspace", resource.PipelineResourceTypeGit, tb.ResourceOptional(true)),
				),
			),
			tb.TaskRunServiceAccountName("sa"),
			tb.TaskRunTimeout(2*time.Minute),
			tb.TaskRunSpecStatus(v1beta1.TaskRunSpecStatusCancelled),
		))
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-taskrun", Namespace: "foo",
			Annotations: map[string]string{},
		},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Image:   "image",
					Command: []string{"/mycmd"},
				}}},
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:     "workspace",
							Type:     resource.PipelineResourceTypeGit,
							Optional: true,
						}}},
				},
			},
			Resources:          &v1beta1.TaskRunResources{},
			ServiceAccountName: "sa",
			Status:             v1beta1.TaskRunSpecStatusCancelled,
			Timeout:            &metav1.Duration{Duration: 2 * time.Minute},
		},
	}
	if d := cmp.Diff(expectedTaskRun, taskRun); d != "" {
		t.Fatalf("TaskRun diff -want, +got: %v", d)
	}
}

func TestTaskRunWithPodTemplate(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun",
		tb.TaskRunNamespace("foo"),
		tb.TaskRunSpec(
			tb.TaskRunTaskSpec(
				tb.Step("image", tb.StepCommand("/mycmd")),
				tb.TaskResources(
					tb.TaskResourcesInput("workspace", resource.PipelineResourceTypeGit, tb.ResourceOptional(true)),
				),
			),
			tb.TaskRunServiceAccountName("sa"),
			tb.TaskRunTimeout(2*time.Minute),
			tb.TaskRunSpecStatus(v1beta1.TaskRunSpecStatusCancelled),
			tb.TaskRunNodeSelector(map[string]string{
				"label": "value",
			}),
		))
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-taskrun", Namespace: "foo",
			Annotations: map[string]string{},
		},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Image:   "image",
					Command: []string{"/mycmd"},
				}}},
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:     "workspace",
							Type:     resource.PipelineResourceTypeGit,
							Optional: true,
						}}},
				},
			},
			PodTemplate: &v1beta1.PodTemplate{
				NodeSelector: map[string]string{
					"label": "value",
				},
			},
			Resources:          &v1beta1.TaskRunResources{},
			ServiceAccountName: "sa",
			Status:             v1beta1.TaskRunSpecStatusCancelled,
			Timeout:            &metav1.Duration{Duration: 2 * time.Minute},
		},
	}
	if d := cmp.Diff(expectedTaskRun, taskRun); d != "" {
		t.Fatalf("TaskRun diff -want, +got: %v", d)
	}
}
