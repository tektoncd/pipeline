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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/artifacts"
	"github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

var (
	outputResources map[string]v1alpha1.PipelineResourceInterface
)

func outputResourceSetup() {
	logger, _ = logging.NewLogger("", "")

	rs := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-git",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []v1alpha1.ResourceParam{{
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
			Params: []v1alpha1.ResourceParam{{
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

	outputResources = make(map[string]v1alpha1.PipelineResourceInterface)
	for _, r := range rs {
		ri, _ := v1alpha1.ResourceFromType(r, images)
		outputResources[r.Name] = ri
	}
}
func TestValidOutputResources(t *testing.T) {

	for _, c := range []struct {
		name        string
		desc        string
		task        *v1alpha1.Task
		taskRun     *v1alpha1.TaskRun
		wantSteps   []v1alpha1.Step
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-git",
							},
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-git",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-git",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-git",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-gcs",
							},
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-gcs",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name:       "source-workspace",
							Type:       "storage",
							TargetPath: "faraway-disk",
						}}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{
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
			}},
			{Container: corev1.Container{
				Name:  "upload-source-gcs-78c5n",
				Image: "google/cloud-sdk",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "volume-source-gcs-sname",
					MountPath: "/var/secret/sname",
				}},
				Command: []string{"gsutil"},
				Args:    []string{"rsync", "-d", "-r", "/workspace/output/source-workspace", "gs://some-bucket"},
				Env: []corev1.EnvVar{{
					Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json",
				}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-gcs",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{
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
			}},
			{Container: corev1.Container{
				Name:  "upload-source-gcs-78c5n",
				Image: "google/cloud-sdk",
				VolumeMounts: []corev1.VolumeMount{{
					Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
				}},
				Env: []corev1.EnvVar{{
					Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json",
				}},
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
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-gcs",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
			}},
			{Container: corev1.Container{
				Name:  "upload-source-gcs-mz4c7",
				Image: "google/cloud-sdk",
				VolumeMounts: []corev1.VolumeMount{{
					Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
				}},
				Env: []corev1.EnvVar{{
					Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json",
				}},
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
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-gcs",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{
			{Container: corev1.Container{
				Name:    "create-dir-source-workspace-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
			}},
			{Container: corev1.Container{
				Name:  "upload-source-gcs-mz4c7",
				Image: "google/cloud-sdk",
				VolumeMounts: []corev1.VolumeMount{{
					Name: "volume-source-gcs-sname", MountPath: "/var/secret/sname",
				}},
				Env: []corev1.EnvVar{{
					Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/sname/key.json",
				}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name:       "source-workspace",
							Type:       "image",
							TargetPath: "/workspace",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace"},
		}}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
	}, {
		desc: "multiple image output resource with no steps",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}, {
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace-1",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}, {
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace-1",
							Type: "image",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
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
			outputResourceSetup()
			fakekubeclient := fakek8s.NewSimpleClientset()
			got, err := AddOutputResources(fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, resolveOutputResources(c.taskRun), logger)
			if err != nil {
				t.Fatalf("Failed to declare output resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}

			if got != nil {
				if d := cmp.Diff(c.wantSteps, got.Steps); d != "" {
					t.Fatalf("post build steps mismatch (-want, +got): %s", d)
				}
				if d := cmp.Diff(c.wantVolumes, got.Volumes); d != "" {
					t.Fatalf("post build steps volumes mismatch (-want, +got): %s", d)
				}
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
		wantSteps []v1alpha1.Step
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-git",
							},
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-git",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}, {Container: corev1.Container{
			Name:    "artifact-copy-to-source-git-mz4c7",
			Image:   "google/cloud-sdk",
			Command: []string{"gsutil"},
			Args:    []string{"cp", "-P", "-r", "/workspace/output/source-workspace", "gs://fake-bucket/pipeline-task-name"},
		}}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-git",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}, {Container: corev1.Container{
			Name:    "artifact-copy-to-source-git-mz4c7",
			Image:   "google/cloud-sdk",
			Command: []string{"gsutil"},
			Args:    []string{"cp", "-P", "-r", "/workspace/output/source-workspace", "gs://fake-bucket/pipeline-task-name"},
		}}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-git",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-source-workspace-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace/output/source-workspace"},
		}}},
	}} {
		t.Run(c.name, func(t *testing.T) {
			outputResourceSetup()
			names.TestingSeed()
			fakekubeclient := fakek8s.NewSimpleClientset(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "tekton-pipelines",
						Name:      artifacts.GetBucketConfigName(),
					},
					Data: map[string]string{
						artifacts.BucketLocationKey: "gs://fake-bucket",
					},
				},
			)
			got, err := AddOutputResources(fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, resolveOutputResources(c.taskRun), logger)
			if err != nil {
				t.Fatalf("Failed to declare output resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}
			if got != nil {
				if d := cmp.Diff(c.wantSteps, got.Steps); d != "" {
					t.Fatalf("post build steps mismatch (-want, got): %s", d)
				}
			}
		})
	}
}

func TestInvalidOutputResources(t *testing.T) {
	for _, c := range []struct {
		desc    string
		task    *v1alpha1.Task
		taskRun *v1alpha1.TaskRun
		wantErr bool
	}{{
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "git",
						}}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
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
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
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
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "invalid-source-storage",
							},
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
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "storage",
						}}},
				},
			},
		},
		wantErr: true,
	}, {
		desc: "optional outputs declared",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{ResourceDeclaration: v1alpha1.ResourceDeclaration{
						Name:     "source-workspace",
						Type:     "git",
						Optional: true,
					}}},
				},
			},
		},
		taskRun: &v1alpha1.TaskRun{
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
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{ResourceDeclaration: v1alpha1.ResourceDeclaration{
						Name:     "source-workspace",
						Type:     "git",
						Optional: false,
					}}},
				},
			},
		},
		taskRun: &v1alpha1.TaskRun{
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
			outputResourceSetup()
			fakekubeclient := fakek8s.NewSimpleClientset()
			_, err := AddOutputResources(fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, resolveOutputResources(c.taskRun), logger)
			if (err != nil) != c.wantErr {
				t.Fatalf("Test AddOutputResourceSteps %v : error%v", c.desc, err)
			}
		})
	}
}

func resolveOutputResources(taskRun *v1alpha1.TaskRun) map[string]v1alpha1.PipelineResourceInterface {
	resolved := make(map[string]v1alpha1.PipelineResourceInterface)
	for _, r := range taskRun.Spec.Outputs.Resources {
		var i v1alpha1.PipelineResourceInterface
		if name := r.ResourceRef.Name; name != "" {
			i = outputResources[name]
			resolved[r.Name] = i
		} else if r.ResourceSpec != nil {
			i, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
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
