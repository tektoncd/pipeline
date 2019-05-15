/*
Copyright 2018 The Knative Authors.

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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	fakeclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

var (
	outputpipelineResourceLister listers.PipelineResourceLister
)

func outputResourceSetup(t *testing.T) {
	logger, _ = logging.NewLogger("", "")
	fakeClient := fakeclientset.NewSimpleClientset()
	sharedInfomer := informers.NewSharedInformerFactory(fakeClient, 0)
	pipelineResourceInformer := sharedInfomer.Tekton().V1alpha1().PipelineResources()
	outputpipelineResourceLister = pipelineResourceInformer.Lister()

	rs := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-git",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []v1alpha1.Param{{
				Name:  "Url",
				Value: "https://github.com/grafeas/kritis",
			}, {
				Name:  "Revision",
				Value: "master",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-source-storage",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "storage",
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-gcs",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []v1alpha1.Param{{
				Name:  "Location",
				Value: "gs://some-bucket",
			}, {
				Name:  "type",
				Value: "gcs",
			}, {
				Name:  "dir",
				Value: "true",
			}},
			SecretParams: []v1alpha1.SecretParam{{
				SecretKey:  "key.json",
				SecretName: "sname",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-image",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "image",
		},
	}}

	for _, r := range rs {
		if err := pipelineResourceInformer.Informer().GetIndexer().Add(r); err != nil {
			t.Fatal(err)
		}
	}
}
func TestValidOutputResources(t *testing.T) {

	for _, c := range []struct {
		name        string
		desc        string
		task        *v1alpha1.Task
		taskRun     *v1alpha1.TaskRun
		wantSteps   []corev1.Container
		wantVolumes []corev1.Volume
	}{{
		name: "git resource in input and output",
		desc: "git resource declared as both input and output with pipelinerun owner reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:    "source-mkdir-source-git-9l9zj",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p pipeline-task-name"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "pipelinerun-pvc",
				MountPath: "/pvc",
			}},
		}, {
			Name:    "source-copy-source-git-mz4c7",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "cp -r /workspace/source-workspace/. pipeline-task-name"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "pipelinerun-pvc",
				MountPath: "/pvc",
			}},
		}},
	}, {
		name: "git resource in output only",
		desc: "git resource declared as output with pipelinerun owner reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:    "source-mkdir-source-git-9l9zj",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p pipeline-task-name"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "pipelinerun-pvc",
				MountPath: "/pvc",
			}},
		}, {
			Name:    "source-copy-source-git-mz4c7",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "cp -r /workspace/output/source-workspace/. pipeline-task-name"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "pipelinerun-pvc",
				MountPath: "/pvc",
			}},
		}},
	}, {
		name: "image resource in output with pipelinerun with owner",
		desc: "image resource declared as output with pipelinerun owner reference should not generate any steps",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image",
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "image",
					}},
				},
			},
		},
		wantSteps:   nil,
		wantVolumes: nil,
	}, {
		name: "git resource in output",
		desc: "git resource declared in output without pipelinerun owner reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
	}, {
		name: "storage resource as both input and output",
		desc: "storage resource defined in both input and output with parents pipelinerun reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun-parent",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-gcs",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-gcs",
						},
						Paths: []string{"pipeline-task-path"},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name:       "source-workspace",
						Type:       "storage",
						TargetPath: "faraway-disk",
					}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "storage",
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:  "upload-source-gcs-9l9zj",
			Image: "override-with-gsutil-image:latest",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "volume-source-gcs-sname",
				MountPath: "/var/secret/sname",
			}},
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "rsync -d -r /workspace/faraway-disk gs://some-bucket"},
			Env: []corev1.EnvVar{{
				Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json",
			}},
		}, {
			Name:         "source-mkdir-source-gcs-mz4c7",
			Image:        "override-with-bash-noop:latest",
			Command:      []string{"/ko-app/bash"},
			Args:         []string{"-args", "mkdir -p pipeline-task-path"},
			VolumeMounts: []corev1.VolumeMount{{Name: "pipelinerun-parent-pvc", MountPath: "/pvc"}},
		}, {
			Name:         "source-copy-source-gcs-mssqb",
			Image:        "override-with-bash-noop:latest",
			Command:      []string{"/ko-app/bash"},
			Args:         []string{"-args", "cp -r /workspace/faraway-disk/. pipeline-task-path"},
			VolumeMounts: []corev1.VolumeMount{{Name: "pipelinerun-parent-pvc", MountPath: "/pvc"}},
		}},
		wantVolumes: []corev1.Volume{{
			Name: "volume-source-gcs-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}, {
		name: "storage resource as output",
		desc: "storage resource defined only in output with pipeline ownder reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-gcs",
						},
						Paths: []string{"pipeline-task-path"},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "storage",
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:  "upload-source-gcs-9l9zj",
			Image: "override-with-gsutil-image:latest",
			VolumeMounts: []corev1.VolumeMount{{
				Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
			}},
			Env: []corev1.EnvVar{{
				Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json",
			}},
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "rsync -d -r /workspace/output/source-workspace gs://some-bucket"},
		}, {
			Name:         "source-mkdir-source-gcs-mz4c7",
			Image:        "override-with-bash-noop:latest",
			Command:      []string{"/ko-app/bash"},
			Args:         []string{"-args", "mkdir -p pipeline-task-path"},
			VolumeMounts: []corev1.VolumeMount{{Name: "pipelinerun-pvc", MountPath: "/pvc"}},
		}, {
			Name:         "source-copy-source-gcs-mssqb",
			Image:        "override-with-bash-noop:latest",
			Command:      []string{"/ko-app/bash"},
			Args:         []string{"-args", "cp -r /workspace/output/source-workspace/. pipeline-task-path"},
			VolumeMounts: []corev1.VolumeMount{{Name: "pipelinerun-pvc", MountPath: "/pvc"}},
		}},
		wantVolumes: []corev1.Volume{{
			Name: "volume-source-gcs-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}, {
		name: "storage resource as output with no owner",
		desc: "storage resource defined only in output without pipelinerun reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-gcs",
						},
						Paths: []string{"pipeline-task-path"},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "storage",
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:  "upload-source-gcs-9l9zj",
			Image: "override-with-gsutil-image:latest",
			VolumeMounts: []corev1.VolumeMount{{
				Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
			}},
			Env: []corev1.EnvVar{{
				Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json",
			}},
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "rsync -d -r /workspace/output/source-workspace gs://some-bucket"},
		}},
		wantVolumes: []corev1.Volume{{
			Name: "volume-source-gcs-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}, {
		name: "storage resource as output with matching build volumes",
		desc: "storage resource defined only in output without pipelinerun reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-gcs",
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "storage",
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:  "upload-source-gcs-9l9zj",
			Image: "override-with-gsutil-image:latest",
			VolumeMounts: []corev1.VolumeMount{{
				Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
			}},
			Env: []corev1.EnvVar{{
				Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json",
			}},
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "rsync -d -r /workspace/output/source-workspace gs://some-bucket"},
		}},
		wantVolumes: []corev1.Volume{{
			Name: "volume-source-gcs-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}, {
		name: "image resource as output",
		desc: "image resource defined only in output",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image",
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "image",
					}},
				},
			},
		},
		wantSteps: nil,
	}, {
		name: "Resource with TargetPath as output",
		desc: "Resource with TargetPath defined only in output",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image",
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name:       "source-workspace",
						Type:       "image",
						TargetPath: "/workspace",
					}},
				},
			},
		},
		wantSteps: nil,
	}, {
		desc: "image output resource with no steps",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image",
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "image",
					}},
				},
			},
		},
		wantSteps: nil,
	}} {
		t.Run(c.name, func(t *testing.T) {
			names.TestingSeed()
			outputResourceSetup(t)
			fakekubeclient := fakek8s.NewSimpleClientset()
			err := AddOutputResources(fakekubeclient, c.task.Name, &c.task.Spec, c.taskRun, outputpipelineResourceLister, logger)
			if err != nil {
				t.Fatalf("Failed to declare output resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}

			if d := cmp.Diff(c.task.Spec.Steps, c.wantSteps); d != "" {
				t.Fatalf("post build steps mismatch: %s", d)
			}
			if c.taskRun.GetPipelineRunPVCName() != "" {
				c.wantVolumes = append(
					c.wantVolumes,
					corev1.Volume{
						Name: c.taskRun.GetPipelineRunPVCName(),
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: c.taskRun.GetPipelineRunPVCName(),
							},
						},
					},
				)
			}

			if d := cmp.Diff(c.task.Spec.Volumes, c.wantVolumes); d != "" {
				t.Fatalf("post build steps volumes mismatch: %s", d)
			}
		})
	}
}

func TestValidOutputResourcesWithBucketStorage(t *testing.T) {
	for _, c := range []struct {
		name      string
		desc      string
		task      *v1alpha1.Task
		taskRun   *v1alpha1.TaskRun
		wantSteps []corev1.Container
	}{{
		name: "git resource in input and output with bucket storage",
		desc: "git resource declared as both input and output with pipelinerun owner reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:    "artifact-copy-to-source-git-9l9zj",
			Image:   "override-with-gsutil-image:latest",
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "cp -r /workspace/source-workspace gs://fake-bucket/pipeline-task-name"},
		}},
	}, {
		name: "git resource in output only with bucket storage",
		desc: "git resource declared as output with pipelinerun owner reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:    "artifact-copy-to-source-git-9l9zj",
			Image:   "override-with-gsutil-image:latest",
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "cp -r /workspace/output/source-workspace gs://fake-bucket/pipeline-task-name"},
		}},
	}, {
		name: "git resource in output",
		desc: "git resource declared in output without pipelinerun owner reference",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
	}} {
		t.Run(c.name, func(t *testing.T) {
			outputResourceSetup(t)
			names.TestingSeed()
			fakekubeclient := fakek8s.NewSimpleClientset(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "tekton-pipelines",
						Name:      v1alpha1.BucketConfigName,
					},
					Data: map[string]string{
						v1alpha1.BucketLocationKey: "gs://fake-bucket",
					},
				},
			)
			err := AddOutputResources(fakekubeclient, c.task.Name, &c.task.Spec, c.taskRun, outputpipelineResourceLister, logger)
			if err != nil {
				t.Fatalf("Failed to declare output resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}

			if d := cmp.Diff(c.task.Spec.Steps, c.wantSteps); d != "" {
				t.Fatalf("post build steps mismatch: %s", d)
			}
		})
	}
}

func TestInValidOutputResources(t *testing.T) {
	for _, c := range []struct {
		desc    string
		task    *v1alpha1.Task
		taskRun *v1alpha1.TaskRun
		wantErr bool
	}{{
		desc: "git declared in both resource spec and resource ref",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
						ResourceSpec: &v1alpha1.PipelineResourceSpec{
							Type: v1alpha1.PipelineResourceTypeGit,
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
		wantErr: true,
	}, {
		desc: "git declared in output both in resource ref and spec",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git",
						},
						ResourceSpec: &v1alpha1.PipelineResourceSpec{
							Type: v1alpha1.PipelineResourceTypeGit,
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
		wantErr: true,
	}, {
		desc: "git declared in neither resource ref and spec",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
		wantErr: true,
	}, {
		desc: "no outputs defined",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
		},
		wantErr: false,
	}, {
		desc: "no outputs defined in task but defined in taskrun",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "git",
					}},
				},
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-gcs",
						},
						Paths: []string{"test-path"},
					}},
				},
			},
		},
		wantErr: false,
	}, {
		desc: "no outputs defined in tasktun but defined in task",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "storage",
					}},
				},
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "foo",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
		},
		wantErr: true,
	}, {
		desc: "invalid storage resource",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "invalid-source-storage",
						},
					}},
				},
			},
		},
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-workspace",
						Type: "storage",
					}},
				},
			},
		},
		wantErr: true,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			outputResourceSetup(t)
			fakekubeclient := fakek8s.NewSimpleClientset()
			err := AddOutputResources(fakekubeclient, c.task.Name, &c.task.Spec, c.taskRun, outputpipelineResourceLister, logger)
			if (err != nil) != c.wantErr {
				t.Fatalf("Test AddOutputResourceSteps %v : error%v", c.desc, err)
			}
		})
	}
}
