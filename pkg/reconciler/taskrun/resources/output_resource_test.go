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

package resources

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

var (
	outputTestResources map[string]v1beta1.PipelineResourceInterface
)

func outputTestResourceSetup() {

	rs := []*resourcev1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-git",
			Namespace: "marshmallow",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []resourcev1alpha1.ResourceParam{{
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
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: "storage",
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-gcs",
			Namespace: "marshmallow",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []resourcev1alpha1.ResourceParam{{
				Name:  "Location",
				Value: "gs://some-bucket",
			}, {
				Name:  "type",
				Value: "gcs",
			}, {
				Name:  "dir",
				Value: "true",
			}},
			SecretParams: []resourcev1alpha1.SecretParam{{
				SecretKey:  "key.json",
				SecretName: "sname",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-gcs-bucket",
			Namespace: "marshmallow",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []resourcev1alpha1.ResourceParam{{
				Name:  "Location",
				Value: "gs://some-bucket",
			}, {
				Name:  "Type",
				Value: "gcs",
			}, {
				Name:  "dir",
				Value: "true",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-gcs-bucket-2",
			Namespace: "marshmallow",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []resourcev1alpha1.ResourceParam{{
				Name:  "Location",
				Value: "gs://some-bucket-2",
			}, {
				Name:  "Type",
				Value: "gcs",
			}, {
				Name:  "dir",
				Value: "true",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-gcs-bucket-3",
			Namespace: "marshmallow",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []resourcev1alpha1.ResourceParam{{
				Name:  "Location",
				Value: "gs://some-bucket-3",
			}, {
				Name:  "Type",
				Value: "gcs",
			}, {
				Name:  "dir",
				Value: "true",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-image",
			Namespace: "marshmallow",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: "image",
		},
	}}

	outputTestResources = make(map[string]v1beta1.PipelineResourceInterface)
	for _, r := range rs {
		ri, _ := resource.FromType(r.Name, r, images)
		outputTestResources[r.Name] = ri
	}
}

func TestValidOutputResources(t *testing.T) {

	for _, c := range []struct {
		name        string
		desc        string
		task        *v1beta1.Task
		taskRun     *v1beta1.TaskRun
		wantSteps   []v1beta1.Step
		wantVolumes []corev1.Volume
	}{{
		name: "git resource in input and output",
		desc: "git resource declared as both input and output with pipelinerun owner reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-git",
							},
						},
					}},
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-git",
							},
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}, {Container: corev1.Container{
			Name:    "source-mkdir-source-git-mz4c7",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "pipeline-task-name"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "pipelinerun-pvc",
				MountPath: "/pvc",
			}},
		}}, {Container: corev1.Container{
			Name:    "source-copy-source-git-mssqb",
			Image:   "busybox",
			Command: []string{"cp", "-r", "/workspace/output/source-workspace/.", "pipeline-task-name"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "pipelinerun-pvc",
				MountPath: "/pvc",
			}},
			Env: []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "source-git"}},
		}}},
		wantVolumes: []corev1.Volume{
			{
				Name: "pipelinerun-pvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pipelinerun-pvc",
						ReadOnly:  false,
					},
				},
			},
		},
	}, {
		name: "git resource in output only",
		desc: "git resource declared as output with pipelinerun owner reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-git",
							},
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}, {Container: corev1.Container{
			Name:    "source-mkdir-source-git-mz4c7",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "pipeline-task-name"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "pipelinerun-pvc",
				MountPath: "/pvc",
			}},
		}}, {Container: corev1.Container{
			Name:    "source-copy-source-git-mssqb",
			Image:   "busybox",
			Command: []string{"cp", "-r", "/workspace/output/source-workspace/.", "pipeline-task-name"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "pipelinerun-pvc",
				MountPath: "/pvc",
			}},
			Env: []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "source-git"}},
		}}},
		wantVolumes: []corev1.Volume{
			{
				Name: "pipelinerun-pvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pipelinerun-pvc",
						ReadOnly:  false,
					},
				},
			},
		},
	}, {
		name: "image resource in output with pipelinerun with owner",
		desc: "image resource declared as output with pipelinerun owner reference should not generate any steps",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
		wantVolumes: nil,
	}, {
		name: "git resource in output",
		desc: "git resource declared in output without pipelinerun owner reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-git",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
	}, {
		name: "storage resource as both input and output",
		desc: "storage resource defined in both input and output with parents pipelinerun reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun-parent",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs",
							},
						},
					}},
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs",
							},
						},
						Paths: []string{"pipeline-task-path"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:       "source-workspace",
							Type:       "storage",
							TargetPath: "faraway-disk",
						}}},
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
			}},
			{Container: corev1.Container{
				Name:         "source-mkdir-source-gcs-mz4c7",
				Image:        "busybox",
				Command:      []string{"mkdir", "-p", "pipeline-task-path"},
				VolumeMounts: []corev1.VolumeMount{{Name: "pipelinerun-parent-pvc", MountPath: "/pvc"}},
			}},
			{Container: corev1.Container{
				Name:         "source-copy-source-gcs-mssqb",
				Image:        "busybox",
				Command:      []string{"cp", "-r", "/workspace/output/source-workspace/.", "pipeline-task-path"},
				VolumeMounts: []corev1.VolumeMount{{Name: "pipelinerun-parent-pvc", MountPath: "/pvc"}},
				Env: []corev1.EnvVar{
					{Name: "TEKTON_RESOURCE_NAME", Value: "source-gcs"},
				},
			}},
			{Container: corev1.Container{
				Name:  "upload-source-gcs-78c5n",
				Image: "gcr.io/google.com/cloudsdktool/cloud-sdk",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "volume-source-gcs-sname",
					MountPath: "/var/secret/sname",
				}},
				Command: []string{"gsutil"},
				Args:    []string{"rsync", "-d", "-r", "/workspace/output/source-workspace", "gs://some-bucket"},
				Env: []corev1.EnvVar{
					{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json"},
					{Name: "HOME", Value: pipeline.HomeDir},
				},
			}},
		},

		wantVolumes: []corev1.Volume{{
			Name: "volume-source-gcs-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		},
			{
				Name: "pipelinerun-parent-pvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pipelinerun-parent-pvc",
						ReadOnly:  false,
					},
				},
			},
		},
	}, {
		name: "storage resource as output",
		desc: "storage resource defined only in output with pipeline ownder reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs",
							},
						},
						Paths: []string{"pipeline-task-path"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
			}},
			{Container: corev1.Container{
				Name:         "source-mkdir-source-gcs-mz4c7",
				Image:        "busybox",
				Command:      []string{"mkdir", "-p", "pipeline-task-path"},
				VolumeMounts: []corev1.VolumeMount{{Name: "pipelinerun-pvc", MountPath: "/pvc"}},
			}},
			{Container: corev1.Container{
				Name:         "source-copy-source-gcs-mssqb",
				Image:        "busybox",
				Command:      []string{"cp", "-r", "/workspace/output/source-workspace/.", "pipeline-task-path"},
				VolumeMounts: []corev1.VolumeMount{{Name: "pipelinerun-pvc", MountPath: "/pvc"}},
				Env: []corev1.EnvVar{
					{Name: "TEKTON_RESOURCE_NAME", Value: "source-gcs"},
				},
			}},
			{Container: corev1.Container{
				Name:  "upload-source-gcs-78c5n",
				Image: "gcr.io/google.com/cloudsdktool/cloud-sdk",
				VolumeMounts: []corev1.VolumeMount{{
					Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
				}},
				Env: []corev1.EnvVar{
					{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json"},
					{Name: "HOME", Value: pipeline.HomeDir},
				},
				Command: []string{"gsutil"},
				Args:    []string{"rsync", "-d", "-r", "/workspace/output/source-workspace", "gs://some-bucket"},
			}},
		},
		wantVolumes: []corev1.Volume{{
			Name: "volume-source-gcs-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		},
			{
				Name: "pipelinerun-pvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pipelinerun-pvc",
					},
				},
			},
		},
	}, {
		name: "storage resource as output with no owner",
		desc: "storage resource defined only in output without pipelinerun reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs",
							},
						},
						Paths: []string{"pipeline-task-path"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
			}},
			{Container: corev1.Container{
				Name:  "upload-source-gcs-mz4c7",
				Image: "gcr.io/google.com/cloudsdktool/cloud-sdk",
				VolumeMounts: []corev1.VolumeMount{{
					Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
				}},
				Env: []corev1.EnvVar{
					{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json"},
					{Name: "HOME", Value: pipeline.HomeDir},
				},
				Command: []string{"gsutil"},
				Args:    []string{"rsync", "-d", "-r", "/workspace/output/source-workspace", "gs://some-bucket"},
			}},
		},
		wantVolumes: []corev1.Volume{{
			Name: "volume-source-gcs-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}, {
		name: "storage resource as output with matching build volumes",
		desc: "storage resource defined only in output without pipelinerun reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
			}},
			{Container: corev1.Container{
				Name:  "upload-source-gcs-mz4c7",
				Image: "gcr.io/google.com/cloudsdktool/cloud-sdk",
				VolumeMounts: []corev1.VolumeMount{{
					Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
				}},
				Env: []corev1.EnvVar{
					{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json"},
					{Name: "HOME", Value: pipeline.HomeDir},
				},
				Command: []string{"gsutil"},
				Args:    []string{"rsync", "-d", "-r", "/workspace/output/source-workspace", "gs://some-bucket"},
			}},
		},
		wantVolumes: []corev1.Volume{{
			Name: "volume-source-gcs-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}, {
		name: "image resource as output",
		desc: "image resource defined only in output",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
	}, {
		name: "Resource with TargetPath as output",
		desc: "Resource with TargetPath defined only in output",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:       "source-workspace",
							Type:       "image",
							TargetPath: "/workspace",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace"},
		}}},
	}, {
		desc: "image output resource with no steps",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
	}, {
		desc: "multiple image output resource with no steps",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}, {
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace-1",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}, {
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace-1",
							Type: "image",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-1-mz4c7",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace-1"},
		}}, {Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
	}} {
		t.Run(c.name, func(t *testing.T) {
			names.TestingSeed()
			outputTestResourceSetup()
			fakekubeclient := fakek8s.NewSimpleClientset()
			got, err := AddOutputResources(context.Background(), fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, resolveOutputResources(c.taskRun))
			if err != nil {
				t.Fatalf("Failed to declare output resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}

			if got != nil {
				if d := cmp.Diff(c.wantSteps, got.Steps); d != "" {
					t.Fatalf("post build steps mismatch %s", diff.PrintWantGot(d))
				}
				if d := cmp.Diff(c.wantVolumes, got.Volumes); d != "" {
					t.Fatalf("post build steps volumes mismatch %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestValidOutputResourcesWithBucketStorage(t *testing.T) {
	for _, c := range []struct {
		name      string
		desc      string
		task      *v1beta1.Task
		taskRun   *v1beta1.TaskRun
		wantSteps []v1beta1.Step
	}{{
		name: "git resource in input and output with bucket storage",
		desc: "git resource declared as both input and output with pipelinerun owner reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-git",
							},
						},
					}},
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-git",
							},
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}, {Container: corev1.Container{
			Name:    "artifact-copy-to-source-git-mz4c7",
			Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
			Command: []string{"gsutil"},
			Args:    []string{"cp", "-P", "-r", "/workspace/output/source-workspace", "gs://fake-bucket/pipeline-task-name"},
		}}},
	}, {
		name: "git resource in output only with bucket storage",
		desc: "git resource declared as output with pipelinerun owner reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-git",
							},
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}, {Container: corev1.Container{
			Name:    "artifact-copy-to-source-git-mz4c7",
			Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
			Command: []string{"gsutil"},
			Args:    []string{"cp", "-P", "-r", "/workspace/output/source-workspace", "gs://fake-bucket/pipeline-task-name"},
		}}},
	}, {
		name: "git resource in output",
		desc: "git resource declared in output without pipelinerun owner reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-git",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
	}} {
		t.Run(c.name, func(t *testing.T) {
			outputTestResourceSetup()
			names.TestingSeed()
			fakekubeclient := fakek8s.NewSimpleClientset()
			bucketConfig, err := config.NewArtifactBucketFromMap(map[string]string{
				config.BucketLocationKey: "gs://fake-bucket",
			})
			if err != nil {
				t.Fatalf("Test: %q; Error setting up bucket config = %v", c.desc, err)
			}
			configs := config.Config{
				ArtifactBucket: bucketConfig,
			}
			ctx := config.ToContext(context.Background(), &configs)
			got, err := AddOutputResources(ctx, fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, resolveOutputResources(c.taskRun))
			if err != nil {
				t.Fatalf("Failed to declare output resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}
			if got != nil {
				if d := cmp.Diff(c.wantSteps, got.Steps); d != "" {
					t.Fatalf("post build steps mismatch %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestInvalidOutputResources(t *testing.T) {
	for _, c := range []struct {
		desc    string
		task    *v1beta1.Task
		taskRun *v1beta1.TaskRun
		wantErr bool
	}{{
		desc: "no outputs defined",
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{},
		},
		taskRun: &v1beta1.TaskRun{
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
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs",
							},
						},
						Paths: []string{"test-path"},
					}},
				},
			},
		},
		wantErr: false,
	}, {
		desc: "no outputs defined in taskrun but defined in task",
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		taskRun: &v1beta1.TaskRun{
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
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "invalid-source-storage",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantErr: true,
	}, {
		desc: "optional outputs declared",
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:     "source-workspace",
						Type:     "git",
						Optional: true,
					}}},
				},
			},
		},
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-optional-output",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
		},
		wantErr: false,
	}, {
		desc: "required outputs declared",
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:     "source-workspace",
						Type:     "git",
						Optional: false,
					}}},
				},
			},
		},
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-required-output",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
		},
		wantErr: true,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			outputTestResourceSetup()
			fakekubeclient := fakek8s.NewSimpleClientset()
			_, err := AddOutputResources(context.Background(), fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, resolveOutputResources(c.taskRun))
			if (err != nil) != c.wantErr {
				t.Fatalf("Test AddOutputResourceSteps %v : error%v", c.desc, err)
			}
		})
	}
}

func resolveInputResources(taskRun *v1beta1.TaskRun) map[string]v1beta1.PipelineResourceInterface {
	resolved := make(map[string]v1beta1.PipelineResourceInterface)
	if taskRun.Spec.Resources == nil {
		return resolved
	}
	for _, r := range taskRun.Spec.Resources.Inputs {
		var i v1beta1.PipelineResourceInterface
		if name := r.ResourceRef.Name; name != "" {
			i = outputTestResources[name]
			resolved[r.Name] = i
		} else if r.ResourceSpec != nil {
			i, _ = resource.FromType(name, &resourcev1alpha1.PipelineResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: r.Name,
				},
				Spec: *r.ResourceSpec,
			}, images)
			resolved[r.Name] = i
		}
	}
	return resolved
}

func resolveOutputResources(taskRun *v1beta1.TaskRun) map[string]v1beta1.PipelineResourceInterface {
	resolved := make(map[string]v1beta1.PipelineResourceInterface)
	if taskRun.Spec.Resources == nil {
		return resolved
	}
	for _, r := range taskRun.Spec.Resources.Outputs {
		var i v1beta1.PipelineResourceInterface
		if name := r.ResourceRef.Name; name != "" {
			i = outputTestResources[name]
			resolved[r.Name] = i
		} else if r.ResourceSpec != nil {
			i, _ = resource.FromType(r.Name, &resourcev1alpha1.PipelineResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: r.Name,
				},
				Spec: *r.ResourceSpec,
			}, images)
			resolved[r.Name] = i
		}
	}
	return resolved
}

// TestInputOutputBucketResources checks that gcs storage resources can be used as both inputs and
// outputs in the same tasks if a artifact bucket configmap exists.
func TestInputOutputBucketResources(t *testing.T) {
	for _, c := range []struct {
		name        string
		desc        string
		task        *v1beta1.Task
		taskRun     *v1beta1.TaskRun
		wantSteps   []v1beta1.Step
		wantVolumes []corev1.Volume
	}{{
		name: "storage resource as both input and output",
		desc: "storage resource defined in both input and output with parents pipelinerun reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun-parent",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs-bucket",
							},
						},
						Paths: []string{"pipeline-task-path"},
					}},
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs-bucket",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:       "source-workspace",
							Type:       "storage",
							TargetPath: "faraway-disk",
						}}},
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-mssqb",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
			}},
			{Container: corev1.Container{
				Name:         "artifact-dest-mkdir-source-workspace-9l9zj",
				Image:        "busybox",
				Command:      []string{"mkdir", "-p", "/workspace/faraway-disk"},
				VolumeMounts: nil,
			}},
			{Container: corev1.Container{
				Name:    "artifact-copy-from-source-workspace-mz4c7",
				Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
				Command: []string{"gsutil"},
				Args: []string{
					"cp",
					"-P",
					"-r",
					"gs://fake-bucket/pipeline-task-path/*",
					"/workspace/faraway-disk",
				},
				Env: []corev1.EnvVar{{
					Name:  "GOOGLE_APPLICATION_CREDENTIALS",
					Value: "/var/bucketsecret/sname/key.json",
				}},
				VolumeMounts: []corev1.VolumeMount{{Name: "volume-bucket-sname", MountPath: "/var/bucketsecret/sname"}},
			}},
			{Container: corev1.Container{
				Name:         "upload-source-gcs-bucket-78c5n",
				Image:        "gcr.io/google.com/cloudsdktool/cloud-sdk",
				VolumeMounts: nil,
				Command:      []string{"gsutil"},
				Args:         []string{"rsync", "-d", "-r", "/workspace/output/source-workspace", "gs://some-bucket"},
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: pipeline.HomeDir,
				}},
			}},
		},
		wantVolumes: []corev1.Volume{{
			Name: "volume-bucket-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}, {
		name: "two storage resource inputs and one output",
		desc: "two storage resources defined in input and one in output with parents pipelinerun reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun-parent",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs-bucket",
							},
						},
						Paths: []string{"pipeline-task-path"},
					}, {
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace-2",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs-bucket-2",
							},
						},
						Paths: []string{"pipeline-task-path-2"},
					}},
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace-3",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs-bucket-3",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:       "source-workspace",
							Type:       "storage",
							TargetPath: "faraway-disk",
						}}, {
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:       "source-workspace-2",
							Type:       "storage",
							TargetPath: "faraway-disk-2",
						}}},
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace-3",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1beta1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-3-6nl7g",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace-3"},
			}},
			{Container: corev1.Container{
				Name:         "artifact-dest-mkdir-source-workspace-mssqb",
				Image:        "busybox",
				Command:      []string{"mkdir", "-p", "/workspace/faraway-disk"},
				VolumeMounts: nil,
			}},
			{Container: corev1.Container{
				Name:    "artifact-copy-from-source-workspace-78c5n",
				Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
				Command: []string{"gsutil"},
				Args: []string{
					"cp",
					"-P",
					"-r",
					"gs://fake-bucket/pipeline-task-path/*",
					"/workspace/faraway-disk",
				},
				Env: []corev1.EnvVar{
					{
						Name:  "GOOGLE_APPLICATION_CREDENTIALS",
						Value: "/var/bucketsecret/sname/key.json",
					},
				},
				VolumeMounts: []corev1.VolumeMount{{Name: "volume-bucket-sname", MountPath: "/var/bucketsecret/sname"}},
			}},
			{Container: corev1.Container{
				Name:         "artifact-dest-mkdir-source-workspace-2-9l9zj",
				Image:        "busybox",
				VolumeMounts: nil,
				Command:      []string{"mkdir", "-p", "/workspace/faraway-disk-2"},
				Env:          nil,
			}},
			{Container: corev1.Container{
				Name:    "artifact-copy-from-source-workspace-2-mz4c7",
				Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
				Command: []string{"gsutil"},
				Args:    []string{"cp", "-P", "-r", "gs://fake-bucket/pipeline-task-path-2/*", "/workspace/faraway-disk-2"},
				Env: []corev1.EnvVar{
					{
						Name:  "GOOGLE_APPLICATION_CREDENTIALS",
						Value: "/var/bucketsecret/sname/key.json",
					},
				},
				VolumeMounts: []corev1.VolumeMount{{Name: "volume-bucket-sname", MountPath: "/var/bucketsecret/sname"}},
			}}, {Container: corev1.Container{
				Name:    "upload-source-gcs-bucket-3-j2tds",
				Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
				Command: []string{"gsutil"},
				Args:    []string{"rsync", "-d", "-r", "/workspace/output/source-workspace-3", "gs://some-bucket-3"},
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: pipeline.HomeDir,
				}},
			}},
		},
		wantVolumes: []corev1.Volume{{
			Name: "volume-bucket-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}, {
		name: "two storage resource outputs",
		desc: "two storage resources defined in output with parents pipelinerun reference",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun-parent",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs-bucket",
							},
						},
					}, {
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace-2",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-gcs-bucket-2",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}, {
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace-2",
							Type: "storage",
						}},
					},
				},
			},
		},
		wantSteps: []v1beta1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-2-mssqb",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace-2"},
			}},
			{Container: corev1.Container{
				Name:         "create-dir-source-workspace-9l9zj",
				Image:        "busybox",
				Command:      []string{"mkdir", "-p", "/workspace/output/source-workspace"},
				VolumeMounts: nil,
			}},
			{Container: corev1.Container{
				Name:    "upload-source-gcs-bucket-mz4c7",
				Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
				Command: []string{"gsutil"},
				Args: []string{
					"rsync",
					"-d",
					"-r",
					"/workspace/output/source-workspace",
					"gs://some-bucket",
				},
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: pipeline.HomeDir,
				}},
			}},
			{Container: corev1.Container{
				Name:         "upload-source-gcs-bucket-2-78c5n",
				Image:        "gcr.io/google.com/cloudsdktool/cloud-sdk",
				VolumeMounts: nil,
				Command:      []string{"gsutil"},
				Args:         []string{"rsync", "-d", "-r", "/workspace/output/source-workspace-2", "gs://some-bucket-2"},
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: pipeline.HomeDir,
				}},
			}},
		},
		wantVolumes: []corev1.Volume{{
			Name: "volume-bucket-sname",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
			},
		}},
	}} {
		t.Run(c.name, func(t *testing.T) {
			names.TestingSeed()
			outputTestResourceSetup()
			fakekubeclient := fakek8s.NewSimpleClientset()
			bucketConfig, err := config.NewArtifactBucketFromMap(map[string]string{
				config.BucketLocationKey:                 "gs://fake-bucket",
				config.BucketServiceAccountSecretNameKey: "sname",
				config.BucketServiceAccountSecretKeyKey:  "key.json",
			})
			if err != nil {
				t.Fatalf("Test: %q; Error setting up bucket config = %v", c.desc, err)
			}
			configs := config.Config{
				ArtifactBucket: bucketConfig,
			}
			ctx := config.ToContext(context.Background(), &configs)
			inputs := resolveInputResources(c.taskRun)
			ts, err := AddInputResource(ctx, fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, inputs)
			if err != nil {
				t.Fatalf("Failed to declare input resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}

			got, err := AddOutputResources(ctx, fakekubeclient, images, c.task.Name, ts, c.taskRun, resolveOutputResources(c.taskRun))
			if err != nil {
				t.Fatalf("Failed to declare output resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}

			if got != nil {
				if d := cmp.Diff(c.wantSteps, got.Steps); d != "" {
					t.Fatalf("post build steps mismatch %s", diff.PrintWantGot(d))
				}
				if d := cmp.Diff(c.wantVolumes, got.Volumes); d != "" {
					t.Fatalf("post build steps volumes mismatch %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}
