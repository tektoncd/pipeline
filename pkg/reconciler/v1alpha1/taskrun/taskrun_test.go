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

package taskrun_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/config"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"github.com/knative/build-pipeline/test"
)

var (
	groupVersionKind = schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "TaskRun",
	}
)

const (
	entrypointLocation = "/tools/entrypoint"
	toolsMountName     = "tools"
	pvcSizeBytes       = 5 * 1024 * 1024 * 1024 // 5 GBs
)

var ignoreLastTransitionTime = cmpopts.IgnoreTypes(duckv1alpha1.Condition{}.LastTransitionTime.Inner.Time)

var toolsMount = corev1.VolumeMount{
	Name:      toolsMountName,
	MountPath: "/tools",
}

var entrypointCopyStep = corev1.Container{
	Name:         "place-tools",
	Image:        config.DefaultEntrypointImage,
	Command:      []string{"/bin/cp"},
	Args:         []string{"/entrypoint", entrypointLocation},
	VolumeMounts: []corev1.VolumeMount{toolsMount},
}

func getExpectedPVC(tr *v1alpha1.TaskRun) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.Namespace,
			// This pvc is specific to this TaskRun, so we'll use the same name
			Name: tr.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(tr, groupVersionKind),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: *resource.NewQuantity(pvcSizeBytes, resource.BinarySI),
				},
			},
		},
	}
}

var simpleTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-task",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name:    "simple-step",
			Image:   "foo",
			Command: []string{"/mycmd"},
		}},
	},
}

var clustertask = &v1alpha1.ClusterTask{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-cluster-task",
	},
	Spec: v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name:    "simple-step",
			Image:   "foo",
			Command: []string{"/mycmd"},
		}},
	},
}

var outputTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-output-task",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name:    "simple-step",
			Image:   "foo",
			Command: []string{"/mycmd"},
		}},
		Inputs: &v1alpha1.Inputs{
			Resources: []v1alpha1.TaskResource{{
				Name: gitResource.Name,
				Type: v1alpha1.PipelineResourceTypeGit,
			}, {
				Name: anotherGitResource.Name,
				Type: v1alpha1.PipelineResourceTypeGit,
			}},
		},
		Outputs: &v1alpha1.Outputs{
			Resources: []v1alpha1.TaskResource{{
				Name: gitResource.Name,
				Type: v1alpha1.PipelineResourceTypeGit,
			}},
		},
	},
}

var saTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-with-sa",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name:    "sa-step",
			Image:   "foo",
			Command: []string{"/mycmd"},
		}},
	},
}

var templatedTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-task-with-templating",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		Inputs: &v1alpha1.Inputs{
			Resources: []v1alpha1.TaskResource{{
				Name: "workspace",
				Type: "git",
			}},
		},
		Outputs: &v1alpha1.Outputs{
			Resources: []v1alpha1.TaskResource{{
				Name: "myimage",
				Type: "image",
			}},
		},
		Steps: []corev1.Container{{
			Name:    "mycontainer",
			Image:   "myimage",
			Command: []string{"/mycmd"},
			Args: []string{
				"--my-arg=${inputs.params.myarg}",
				"--my-additional-arg=${outputs.resources.myimage.url}"},
		}, {
			Name:    "myothercontainer",
			Image:   "myotherimage",
			Command: []string{"/mycmd"},
			Args:    []string{"--my-other-arg=${inputs.resources.workspace.url}"},
		}},
	},
}

var defaultTemplatedTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-task-with-default-templating",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		Inputs: &v1alpha1.Inputs{
			Params: []v1alpha1.TaskParam{{
				Name:        "myarg",
				Description: "mydesc",
				Default:     "mydefault",
			}},
		},
		Steps: []corev1.Container{{
			Name:    "mycontainer",
			Image:   "myimage",
			Command: []string{"/mycmd"},
			Args:    []string{"--my-arg=${inputs.params.myarg}"},
		}, {
			Name:    "myothercontainer",
			Image:   "myotherimage",
			Command: []string{"/mycmd"},
			Args:    []string{"--my-other-arg=${inputs.resources.git-resource.url}"},
		}},
	},
}

var gitResource = &v1alpha1.PipelineResource{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "git-resource",
		Namespace: "foo",
	},
	Spec: v1alpha1.PipelineResourceSpec{
		Type: "git",
		Params: []v1alpha1.Param{{
			Name:  "URL",
			Value: "https://foo.git",
		}},
	},
}

var anotherGitResource = &v1alpha1.PipelineResource{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "another-git-resource",
		Namespace: "foo",
	},
	Spec: v1alpha1.PipelineResourceSpec{
		Type: "git",
		Params: []v1alpha1.Param{{
			Name:  "URL",
			Value: "https://foobar.git",
		}},
	},
}
var imageResource = &v1alpha1.PipelineResource{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "image-resource",
		Namespace: "foo",
	},
	Spec: v1alpha1.PipelineResourceSpec{
		Type: "image",
		Params: []v1alpha1.Param{{
			Name:  "URL",
			Value: "gcr.io/kristoff/sven",
		}},
	},
}

func getToolsVolume(claimName string) corev1.Volume {
	return corev1.Volume{
		Name: toolsMountName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
			},
		},
	}
}

func getRunName(tr *v1alpha1.TaskRun) string {
	return strings.Join([]string{tr.Namespace, tr.Name}, "/")
}

func TestReconcile(t *testing.T) {
	taskruns := []*v1alpha1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       simpleTask.Name,
				APIVersion: "a1",
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-with-sa-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			ServiceAccount: "test-sa",
			TaskRef: &v1alpha1.TaskRef{
				Name:       saTask.Name,
				APIVersion: "a1",
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-templating",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       templatedTask.Name,
				APIVersion: "a1",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{
					{
						Name:  "myarg",
						Value: "foo",
					},
				},
				Resources: []v1alpha1.TaskResourceBinding{
					{
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name:       gitResource.Name,
							APIVersion: "a1",
						},
						Name: "workspace",
					},
				},
			},
			Outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name:       "image-resource",
						APIVersion: "a1",
					},
					Name: "myimage",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-overrides-default-templating",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       defaultTemplatedTask.Name,
				APIVersion: "a1",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{
					{
						Name:  "myarg",
						Value: "foo",
					},
				},
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name:       gitResource.Name,
						APIVersion: "a1",
					},
					Name: gitResource.Name,
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-default-templating",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       defaultTemplatedTask.Name,
				APIVersion: "a1",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name:       gitResource.Name,
						APIVersion: "a1",
					},
					Name: gitResource.Name,
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-input-output",
			Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "PipelineRun",
				Name: "test",
			}},
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name: outputTask.Name,
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: gitResource.Name,
					},
					Name:  gitResource.Name,
					Paths: []string{"source-folder"},
				}, {
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: anotherGitResource.Name,
					},
					Name:  anotherGitResource.Name,
					Paths: []string{"source-folder"},
				}},
			},
			Outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: gitResource.Name,
					},
					Name:  gitResource.Name,
					Paths: []string{"output-folder"},
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-with-taskSpec",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{
					{
						Name:  "myarg",
						Value: "foo",
					},
				},
				Resources: []v1alpha1.TaskResourceBinding{
					{
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name:       gitResource.Name,
							APIVersion: "a1",
						},
						Name: "workspace",
					},
				},
			},
			TaskSpec: &v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Type: "git",
						Name: "workspace",
					}},
					Params: []v1alpha1.TaskParam{{
						Name:        "myarg",
						Description: "mydesc",
						Default:     "mydefault",
					}},
				},
				Steps: []corev1.Container{{
					Name:    "mycontainer",
					Image:   "myimage",
					Command: []string{"/mycmd"},
					Args:    []string{"--my-arg=${inputs.params.myarg}"},
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-with-task-sa-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       saTask.Name,
				APIVersion: "a1",
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-with-cluster-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       clustertask.Name,
				APIVersion: "a1",
				Kind:       "ClusterTask",
			},
		},
	}}

	d := test.Data{
		TaskRuns:          taskruns,
		Tasks:             []*v1alpha1.Task{simpleTask, saTask, templatedTask, defaultTemplatedTask, outputTask},
		ClusterTasks:      []*v1alpha1.ClusterTask{clustertask},
		PipelineResources: []*v1alpha1.PipelineResource{gitResource, anotherGitResource, imageResource},
	}
	testcases := []struct {
		name            string
		taskRun         *v1alpha1.TaskRun
		wantedBuildSpec buildv1alpha1.BuildSpec
	}{{
		name:    "success",
		taskRun: taskruns[0],
		wantedBuildSpec: buildv1alpha1.BuildSpec{
			Steps: []corev1.Container{
				entrypointCopyStep,
				{
					Name:    "simple-step",
					Image:   "foo",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				},
			},
			Volumes: []corev1.Volume{
				getToolsVolume(taskruns[0].Name),
			},
		},
	}, {
		name:    "serviceaccount",
		taskRun: taskruns[1],
		wantedBuildSpec: buildv1alpha1.BuildSpec{
			ServiceAccountName: "test-sa",
			Steps: []corev1.Container{
				entrypointCopyStep,
				{
					Name:    "sa-step",
					Image:   "foo",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				},
			},
			Volumes: []corev1.Volume{
				getToolsVolume(taskruns[1].Name),
			},
		},
	}, {
		name:    "params",
		taskRun: taskruns[2],
		wantedBuildSpec: buildv1alpha1.BuildSpec{
			Sources: []buildv1alpha1.SourceSpec{{
				Git: &buildv1alpha1.GitSourceSpec{
					Url:      "https://foo.git",
					Revision: "master",
				},
				Name: "git-resource",
			}},
			Steps: []corev1.Container{
				entrypointCopyStep,
				{
					Name:    "mycontainer",
					Image:   "myimage",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd","--my-arg=foo","--my-additional-arg=gcr.io/kristoff/sven"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				}, {
					Name:    "myothercontainer",
					Image:   "myotherimage",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd","--my-other-arg=https://foo.git"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				}},
			Volumes: []corev1.Volume{
				getToolsVolume(taskruns[2].Name),
			},
		},
	}, {
		name:    "input-overrides-default-params",
		taskRun: taskruns[3],
		wantedBuildSpec: buildv1alpha1.BuildSpec{
			Steps: []corev1.Container{
				entrypointCopyStep,
				{
					Name:    "mycontainer",
					Image:   "myimage",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd","--my-arg=foo"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				}, {
					Name:    "myothercontainer",
					Image:   "myotherimage",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd","--my-other-arg=https://foo.git"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				},
			},
			Volumes: []corev1.Volume{getToolsVolume(taskruns[3].Name)},
		},
	}, {
		name:    "default-params",
		taskRun: taskruns[4],
		wantedBuildSpec: buildv1alpha1.BuildSpec{
			Steps: []corev1.Container{
				entrypointCopyStep,
				{
					Name:    "mycontainer",
					Image:   "myimage",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd","--my-arg=mydefault"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				}, {
					Name:    "myothercontainer",
					Image:   "myotherimage",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd","--my-other-arg=https://foo.git"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				},
			},
			Volumes: []corev1.Volume{getToolsVolume(taskruns[4].Name)},
		},
	}, {
		name:    "wrap-steps",
		taskRun: taskruns[5],
		wantedBuildSpec: buildv1alpha1.BuildSpec{
			Sources: []buildv1alpha1.SourceSpec{{
				Git:  &buildv1alpha1.GitSourceSpec{Url: "https://foo.git", Revision: "master"},
				Name: "git-resource",
			}, {
				Git:  &buildv1alpha1.GitSourceSpec{Url: "https://foobar.git", Revision: "master"},
				Name: "another-git-resource",
			}},
			Steps: []corev1.Container{
				{
					Name:    "source-copy-another-git-resource-0",
					Image:   "busybox",
					Command: []string{"cp"}, Args: []string{"-r", "source-folder/.", "/workspace"},
					VolumeMounts: []corev1.VolumeMount{{Name: "test-pvc", MountPath: "/pvc"}},
				},
				{
					Name:    "source-copy-git-resource-0",
					Image:   "busybox",
					Command: []string{"cp"}, Args: []string{"-r", "source-folder/.", "/workspace"},
					VolumeMounts: []corev1.VolumeMount{{Name: "test-pvc", MountPath: "/pvc"}},
				},
				entrypointCopyStep, {
					Name:    "simple-step",
					Image:   "foo",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				}, {
					Name:         "source-mkdir-git-resource",
					Image:        "busybox",
					Command:      []string{"mkdir"},
					Args:         []string{"-p", "output-folder"},
					VolumeMounts: []corev1.VolumeMount{{Name: "test-pvc", MountPath: "/pvc"}},
				}, {
					Name:         "source-copy-git-resource",
					Image:        "busybox",
					Command:      []string{"cp"},
					Args:         []string{"-r", "/workspace/.", "output-folder"},
					VolumeMounts: []corev1.VolumeMount{{Name: "test-pvc", MountPath: "/pvc"}},
				}},
			Volumes: []corev1.Volume{
				getToolsVolume(taskruns[5].Name),
				resources.GetPVCVolume(taskruns[5].GetPipelineRunPVCName()),
			},
		},
	}, {
		name:    "taskrun-with-taskspec",
		taskRun: taskruns[6],
		wantedBuildSpec: buildv1alpha1.BuildSpec{
			Sources: []buildv1alpha1.SourceSpec{{
				Name: "git-resource",
				Git: &buildv1alpha1.GitSourceSpec{
					Url:      "https://foo.git",
					Revision: "master",
				},
			}},
			Steps: []corev1.Container{
				entrypointCopyStep,
				{
					Name:    "mycontainer",
					Image:   "myimage",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd","--my-arg=foo"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				},
			},
			Volumes: []corev1.Volume{
				getToolsVolume(taskruns[6].Name),
			},
		},
	}, {
		name:    "success-with-cluster-task",
		taskRun: taskruns[8],
		wantedBuildSpec: buildv1alpha1.BuildSpec{
			Steps: []corev1.Container{
				entrypointCopyStep,
				{
					Name:    "simple-step",
					Image:   "foo",
					Command: []string{entrypointLocation},
					Args:    []string{},
					Env: []corev1.EnvVar{
						{
							Name:  "ENTRYPOINT_OPTIONS",
							Value: `{"args":["/mycmd"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
						},
					},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				},
			},
			Volumes: []corev1.Volume{
				getToolsVolume(taskruns[8].Name),
			},
		},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testAssets := test.GetTaskRunController(d)
			c := testAssets.Controller
			clients := testAssets.Clients
			if err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.taskRun)); err != nil {
				t.Errorf("expected no error. Got error %v", err)
			}
			if len(clients.Build.Actions()) == 0 {
				t.Errorf("Expected actions to be logged in the buildclient, got none")
			}
			// check error
			build, err := clients.Build.BuildV1alpha1().Builds(tc.taskRun.Namespace).Get(tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to fetch build: %v", err)
			}
			if d := cmp.Diff(build.Spec, tc.wantedBuildSpec); d != "" {
				t.Errorf("buildspec doesn't match, diff: %s", d)
			}

			// This TaskRun is in progress now and the status should reflect that
			condition := tc.taskRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionUnknown {
				t.Errorf("Expected invalid TaskRun to have in progress status, but had %v", condition)
			}
			if condition != nil && condition.Reason != taskrun.ReasonRunning {
				t.Errorf("Expected reason %q but was %s", taskrun.ReasonRunning, condition.Reason)
			}

			namespace, name, err := cache.SplitMetaNamespaceKey(tc.taskRun.Name)
			if err != nil {
				t.Errorf("Invalid resource key: %v", err)
			}
			//Command, Args, Env, VolumeMounts
			if len(clients.Kube.Actions()) == 0 {
				t.Fatalf("Expected actions to be logged in the kubeclient, got none")
			}
			// 3. check that volume was created
			pvc, err := clients.Kube.CoreV1().PersistentVolumeClaims(namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to fetch build: %v", err)
			}

			// get related TaskRun to populate expected PVC
			tr, err := clients.Pipeline.PipelineV1alpha1().TaskRuns(namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to fetch build: %v", err)
			}
			expectedVolume := getExpectedPVC(tr)
			if d := cmp.Diff(pvc.Name, expectedVolume.Name); d != "" {
				t.Errorf("pvc doesn't match, diff: %s", d)
			}
			if d := cmp.Diff(pvc.OwnerReferences, expectedVolume.OwnerReferences); d != "" {
				t.Errorf("pvc doesn't match, diff: %s", d)
			}
			if d := cmp.Diff(pvc.Spec.AccessModes, expectedVolume.Spec.AccessModes); d != "" {
				t.Errorf("pvc doesn't match, diff: %s", d)
			}
			if pvc.Spec.Resources.Requests["storage"] != expectedVolume.Spec.Resources.Requests["storage"] {
				t.Errorf("pvc doesn't match, got: %v, expected: %v",
					pvc.Spec.Resources.Requests["storage"],
					expectedVolume.Spec.Resources.Requests["storage"])
			}
		})
	}
}

func TestReconcile_InvalidTaskRuns(t *testing.T) {
	taskRuns := []*v1alpha1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "notaskrun",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       "notask",
				APIVersion: "a1",
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrun-with-wrong-ref",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       "notask",
				APIVersion: "a1",
				Kind:       "ClusterTask",
			},
		},
	}}
	tasks := []*v1alpha1.Task{
		simpleTask,
	}

	d := test.Data{
		TaskRuns: taskRuns,
		Tasks:    tasks,
	}

	testcases := []struct {
		name    string
		taskRun *v1alpha1.TaskRun
		reason  string
	}{
		{
			name:    "task run with no task",
			taskRun: taskRuns[0],
			reason:  taskrun.ReasonFailedResolution,
		},
		{
			name:    "task run with no task",
			taskRun: taskRuns[1],
			reason:  taskrun.ReasonFailedResolution,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testAssets := test.GetTaskRunController(d)
			c := testAssets.Controller
			clients := testAssets.Clients
			err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.taskRun))
			// When a TaskRun is invalid and can't run, we don't want to return an error because
			// an error will tell the Reconciler to keep trying to reconcile; instead we want to stop
			// and forget about the Run.
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid TaskRun but saw %q", err)
			}
			if len(clients.Build.Actions()) != 0 {
				t.Errorf("expected no actions created by the reconciler, got %v", clients.Build.Actions())
			}
			// Since the TaskRun is invalid, the status should say it has failed
			condition := tc.taskRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionFalse {
				t.Errorf("Expected invalid TaskRun to have failed status, but had %v", condition)
			}
			if condition != nil && condition.Reason != tc.reason {
				t.Errorf("Expected failure to be because of reason %q but was %s", tc.reason, condition.Reason)
			}
		})
	}

}

func TestReconcileBuildFetchError(t *testing.T) {
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       "test-task",
				APIVersion: "a1",
			},
		},
	}
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{
			taskRun,
		},
		Tasks: []*v1alpha1.Task{simpleTask},
	}

	testAssets := test.GetTaskRunController(d)
	c := testAssets.Controller
	clients := testAssets.Clients

	reactor := func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() == "get" && action.GetResource().Resource == "builds" {
			// handled fetching builds
			return true, nil, fmt.Errorf("induce failure fetching builds")
		}
		return false, nil, nil
	}

	clients.Build.PrependReactor("*", "*", reactor)

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err == nil {
		t.Fatal("expected error when reconciling a Task for which we couldn't get the corresponding Build but got nil")
	}
}

func TestReconcileBuildUpdateStatus(t *testing.T) {
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       "test-task",
				APIVersion: "a1",
			},
		},
	}
	build := &buildv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskRun.Name,
			Namespace: taskRun.Namespace,
		},
		Spec: *simpleTask.Spec.GetBuildSpec(),
	}
	buildSt := &duckv1alpha1.Condition{
		Type: duckv1alpha1.ConditionSucceeded,
		// build is not completed
		Status:  corev1.ConditionUnknown,
		Message: "Running build",
	}
	build.Status.SetCondition(buildSt)
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{
			taskRun,
		},
		Tasks:  []*v1alpha1.Task{simpleTask},
		Builds: []*buildv1alpha1.Build{build},
	}

	testAssets := test.GetTaskRunController(d)
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when Reconcile() : %v", err)
	}
	newTr, err := clients.Pipeline.PipelineV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if d := cmp.Diff(newTr.Status.GetCondition(duckv1alpha1.ConditionSucceeded), buildSt, ignoreLastTransitionTime); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	// update build status and trigger reconcile : build is completed
	buildSt.Status = corev1.ConditionTrue
	buildSt.Message = "Build completed"
	build.Status.SetCondition(buildSt)

	if _, err = clients.Build.BuildV1alpha1().Builds(taskRun.Namespace).Update(build); err != nil {
		t.Errorf("Unexpected error while creating build: %v", err)
	}
	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when Reconcile(): %v", err)
	}

	newTr, err = clients.Pipeline.PipelineV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching taskrun: %v", err)
	}
	if d := cmp.Diff(newTr.Status.GetCondition(duckv1alpha1.ConditionSucceeded), buildSt, ignoreLastTransitionTime); d != "" {
		t.Errorf("Taskrun Status diff -want, +got: %v", d)
	}
}

func TestUpdateStatusFromBuildStatus(t *testing.T) {
	completed := corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "success"},
	}
	waiting := corev1.ContainerState{
		Waiting: &corev1.ContainerStateWaiting{
			Message: "foo",
			Reason:  "bar",
		},
	}
	failed := corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{ExitCode: 127, Reason: "oh-my-lord"},
	}
	startTime := metav1.NewTime(time.Date(2018, time.November, 10, 23, 0, 0, 0, time.UTC))
	completionTime := metav1.NewTime(time.Date(2018, time.November, 10, 23, 8, 0, 0, time.UTC))
	testCases := []struct {
		name           string
		buildStatus    buildv1alpha1.BuildStatus
		expectedStatus v1alpha1.TaskRunStatus
	}{
		{
			name:        "empty build status",
			buildStatus: buildv1alpha1.BuildStatus{},
			expectedStatus: v1alpha1.TaskRunStatus{
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Reason:  taskrun.ReasonRunning,
					Message: taskrun.ReasonRunning,
				}},
				Steps: []v1alpha1.StepState{},
			},
		},
		{
			name: "build validate failed",
			buildStatus: buildv1alpha1.BuildStatus{
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Reason:  "BuildValidationFailed",
					Message: `serviceaccounts "missing-sa" not-found`,
				}},
			},
			expectedStatus: v1alpha1.TaskRunStatus{
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Reason:  "BuildValidationFailed",
					Message: `serviceaccounts "missing-sa" not-found`,
				}},
				Steps: []v1alpha1.StepState{},
			},
		},
		{
			name: "running build status",
			buildStatus: buildv1alpha1.BuildStatus{
				StartTime: &startTime,
				StepStates: []corev1.ContainerState{
					waiting,
				},
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Reason:  "Running build",
					Message: "Running build",
				}},
				Cluster: &buildv1alpha1.ClusterSpec{
					Namespace: "default",
					PodName:   "im-am-the-pod",
				},
			},
			expectedStatus: v1alpha1.TaskRunStatus{
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Reason:  "Running build",
					Message: "Running build",
				}},
				Steps: []v1alpha1.StepState{
					{ContainerState: *waiting.DeepCopy()},
				},
				PodName:   "im-am-the-pod",
				StartTime: &startTime,
			},
		},
		{
			name: "completed build status (success)",
			buildStatus: buildv1alpha1.BuildStatus{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
				StepStates: []corev1.ContainerState{
					completed,
				},
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Reason:  "Build succeeded",
					Message: "Build succeeded",
				}},
				Cluster: &buildv1alpha1.ClusterSpec{
					Namespace: "default",
					PodName:   "im-am-the-pod",
				},
			},
			expectedStatus: v1alpha1.TaskRunStatus{
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Reason:  "Build succeeded",
					Message: "Build succeeded",
				}},
				Steps: []v1alpha1.StepState{
					{ContainerState: *completed.DeepCopy()},
				},
				PodName:        "im-am-the-pod",
				StartTime:      &startTime,
				CompletionTime: &completionTime,
			},
		},
		{
			name: "completed build status (failure)",
			buildStatus: buildv1alpha1.BuildStatus{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
				StepStates: []corev1.ContainerState{
					completed,
					failed,
				},
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Reason:  "Build failed",
					Message: "Build failed",
				}},
				Cluster: &buildv1alpha1.ClusterSpec{
					Namespace: "default",
					PodName:   "im-am-the-pod",
				},
			},
			expectedStatus: v1alpha1.TaskRunStatus{
				Conditions: []duckv1alpha1.Condition{{
					Type:    duckv1alpha1.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Reason:  "Build failed",
					Message: "Build failed",
				}},
				Steps: []v1alpha1.StepState{
					{ContainerState: *completed.DeepCopy()},
					{ContainerState: *failed.DeepCopy()},
				},
				PodName:        "im-am-the-pod",
				StartTime:      &startTime,
				CompletionTime: &completionTime,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskRun := &v1alpha1.TaskRun{}
			taskrun.UpdateStatusFromBuildStatus(taskRun, tc.buildStatus)
			if d := cmp.Diff(taskRun.Status, tc.expectedStatus, ignoreLastTransitionTime); d != "" {
				t.Errorf("-want, +got: %v", d)
			}
		})
	}
}

func TestCreateRedirectedBuild(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: &config.Entrypoint{
			Image: config.DefaultEntrypointImage,
		},
	}
	ctx := config.ToContext(context.Background(), cfg)

	tr := &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			ServiceAccount: "sa",
		},
	}
	tr.Name = "tr"
	tr.Namespace = "tr"

	bs := &buildv1alpha1.BuildSpec{
		Steps: []corev1.Container{
			{
				Command: []string{"abcd"},
				Args:    []string{"efgh"},
			},
			{
				Command: []string{"abcd"},
				Args:    []string{"efgh"},
			},
		},
		Volumes: []corev1.Volume{{Name: "v"}},
	}
	expectedSteps := len(bs.Steps) + 1
	expectedVolumes := len(bs.Volumes) + 1

	b, err := taskrun.CreateRedirectedBuild(ctx, bs, "pvc", tr)
	if err != nil {
		t.Errorf("expected CreateRedirectedBuild to pass: %v", err)
	}
	if b.Name != tr.Name {
		t.Errorf("names do not match: %s should be %s", b.Name, tr.Name)
	}
	if len(b.Spec.Steps) != expectedSteps {
		t.Errorf("step counts do not match: %d should be %d", len(b.Spec.Steps), expectedSteps)
	}
	if len(b.Spec.Volumes) != expectedVolumes {
		t.Errorf("volumes do not match: %d should be %d", len(b.Spec.Volumes), expectedVolumes)
	}
	if b.Spec.ServiceAccountName != tr.Spec.ServiceAccount {
		t.Errorf("services accounts do not match: %s should be %s", b.Spec.ServiceAccountName, tr.Spec.ServiceAccount)
	}
}

func TestReconcileOnCompletedTaskRun(t *testing.T) {
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       simpleTask.Name,
				APIVersion: "a1",
			},
		},
	}
	taskSt := &duckv1alpha1.Condition{
		Type:    duckv1alpha1.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  "Build succeeded",
		Message: "Build succeeded",
	}
	taskRun.Status.SetCondition(taskSt)
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{
			taskRun,
		},
		Tasks: []*v1alpha1.Task{simpleTask},
	}

	testAssets := test.GetTaskRunController(d)
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.PipelineV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if d := cmp.Diff(newTr.Status.GetCondition(duckv1alpha1.ConditionSucceeded), taskSt, ignoreLastTransitionTime); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}
