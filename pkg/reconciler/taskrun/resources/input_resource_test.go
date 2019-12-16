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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/artifacts"
	"github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/test/names"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

var (
	images = pipeline.Images{
		EntrypointImage:          "override-with-entrypoint:latest",
		NopImage:                 "tianon/true",
		GitImage:                 "override-with-git:latest",
		CredsImage:               "override-with-creds:latest",
		KubeconfigWriterImage:    "override-with-kubeconfig-writer:latest",
		ShellImage:               "busybox",
		GsutilImage:              "google/cloud-sdk",
		BuildGCSFetcherImage:     "gcr.io/cloud-builders/gcs-fetcher:latest",
		PRImage:                  "override-with-pr:latest",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}
	inputResourceInterfaces map[string]v1alpha1.PipelineResourceInterface
	logger                  *zap.SugaredLogger

	gitInputs = &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{{
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name: "gitspace",
				Type: "git",
			}}},
	}
	multipleGitInputs = &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{{
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name: "gitspace",
				Type: "git",
			}}, {
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name: "git-duplicate-space",
				Type: "git",
			}},
		},
	}
	gcsInputs = &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{{
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name:       "workspace",
				Type:       "gcs",
				TargetPath: "gcs-dir",
			}}},
	}
	multipleGcsInputs = &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{
			{
				ResourceDeclaration: v1alpha1.ResourceDeclaration{
					Name:       "workspace",
					Type:       "gcs",
					TargetPath: "gcs-dir",
				},
			},
			{
				ResourceDeclaration: v1alpha1.ResourceDeclaration{
					Name:       "workspace2",
					Type:       "gcs",
					TargetPath: "gcs-dir",
				},
			},
		},
	}
	clusterInputs = &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{{
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name: "target-cluster",
				Type: "cluster",
			}}},
	}
	optionalGitInputs = &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{{
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name:     "gitspace",
				Type:     "git",
				Optional: false,
			}}, {
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name:     "git-optional-space",
				Type:     "git",
				Optional: true,
			}},
		},
	}
)

func setUp() {
	logger, _ = logging.NewLogger("", "")

	rs := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "the-git",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []v1alpha1.ResourceParam{{
				Name:  "Url",
				Value: "https://github.com/grafeas/kritis",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "the-git-with-branch",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []v1alpha1.ResourceParam{{
				Name:  "Url",
				Value: "https://github.com/grafeas/kritis",
			}, {
				Name:  "Revision",
				Value: "branch",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "the-git-with-sslVerify-false",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []v1alpha1.ResourceParam{{
				Name:  "Url",
				Value: "https://github.com/grafeas/kritis",
			}, {
				Name:  "Revision",
				Value: "branch",
			}, {
				Name:  "SSLVerify",
				Value: "false",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster2",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "cluster",
			Params: []v1alpha1.ResourceParam{{
				Name:  "Name",
				Value: "cluster2",
			}, {
				Name:  "Url",
				Value: "http://10.10.10.10",
			}},
			SecretParams: []v1alpha1.SecretParam{{
				FieldName:  "cadata",
				SecretKey:  "cadatakey",
				SecretName: "secret1",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster3",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "cluster",
			Params: []v1alpha1.ResourceParam{{
				Name:  "name",
				Value: "cluster3",
			}, {
				Name:  "Url",
				Value: "http://10.10.10.10",
			}, {
				Name:  "Namespace",
				Value: "namespace1",
			}, {
				Name: "CAdata",
				// echo "my-ca-cert" | base64
				Value: "bXktY2EtY2VydAo=",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage1",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []v1alpha1.ResourceParam{{
				Name:  "Location",
				Value: "gs://fake-bucket/rules.zip",
			}, {
				Name:  "Type",
				Value: "gcs",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage2",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []v1alpha1.ResourceParam{{
				Name:  "Location",
				Value: "gs://fake-bucket/other.zip",
			}, {
				Name:  "Type",
				Value: "gcs",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-gcs-keys",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []v1alpha1.ResourceParam{{
				Name:  "Location",
				Value: "gs://fake-bucket/rules.zip",
			}, {
				Name:  "Type",
				Value: "gcs",
			}, {
				Name:  "Dir",
				Value: "true",
			}},
			SecretParams: []v1alpha1.SecretParam{{
				SecretKey:  "key.json",
				SecretName: "secret-name",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			}, {
				SecretKey:  "token",
				SecretName: "secret-name2",
				FieldName:  "GOOGLE_TOKEN",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-gcs-invalid",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "storage",
			Params: []v1alpha1.ResourceParam{{
				Name:  "Location",
				Value: "gs://fake-bucket/rules",
			}, {
				Name:  "Type",
				Value: "non-existent",
			}},
		},
	}}
	inputResourceInterfaces = make(map[string]v1alpha1.PipelineResourceInterface)
	for _, r := range rs {
		ri, _ := v1alpha1.ResourceFromType(r, images)
		inputResourceInterfaces[r.Name] = ri
	}
}

func TestAddResourceToTask(t *testing.T) {

	task := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-from-repo",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: gitInputs,
		},
	}
	taskWithMultipleGitSources := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-from-repo",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: multipleGitInputs,
		},
	}
	taskWithTargetPath := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task-with-targetpath",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: gcsInputs,
		},
	}
	taskWithOptionalGitSources := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-from-repo-with-optional-source",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: optionalGitInputs,
		},
	}

	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-from-repo-run",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name: "simpleTask",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
						ResourceRef: &v1alpha1.PipelineResourceRef{
							Name: "the-git",
						},
						Name: "gitspace",
					},
				}},
			},
		},
	}

	for _, c := range []struct {
		desc    string
		task    *v1alpha1.Task
		taskRun *v1alpha1.TaskRun
		wantErr bool
		want    *v1alpha1.TaskSpec
	}{{
		desc:    "simple with default revision",
		task:    task,
		taskRun: taskRun,
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "git-source-the-git-9l9zj",
				Image:      "override-with-git:latest",
				Command:    []string{"/ko-app/git-init"},
				Args:       []string{"-url", "https://github.com/grafeas/kritis", "-revision", "master", "-path", "/workspace/gitspace"},
				WorkingDir: "/workspace",
				Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "the-git"}},
			}}},
		},
	}, {
		desc: "simple with branch",
		task: task,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "simpleTask",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git-with-branch",
							},
							Name: "gitspace",
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "git-source-the-git-with-branch-9l9zj",
				Image:      "override-with-git:latest",
				Command:    []string{"/ko-app/git-init"},
				Args:       []string{"-url", "https://github.com/grafeas/kritis", "-revision", "branch", "-path", "/workspace/gitspace"},
				WorkingDir: "/workspace",
				Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "the-git-with-branch"}},
			}}},
		},
	}, {
		desc: "reuse git input resource and verify order",
		task: taskWithMultipleGitSources,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "simpleTask",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git-with-branch",
							},
							Name: "gitspace",
						},
					}, {
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git-with-branch",
							},
							Name: "git-duplicate-space",
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: multipleGitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "git-source-the-git-with-branch-mz4c7",
				Image:      "override-with-git:latest",
				Command:    []string{"/ko-app/git-init"},
				Args:       []string{"-url", "https://github.com/grafeas/kritis", "-revision", "branch", "-path", "/workspace/gitspace"},
				WorkingDir: "/workspace",
				Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "the-git-with-branch"}},
			}}, {Container: corev1.Container{
				Name:       "git-source-the-git-with-branch-9l9zj",
				Image:      "override-with-git:latest",
				Command:    []string{"/ko-app/git-init"},
				Args:       []string{"-url", "https://github.com/grafeas/kritis", "-revision", "branch", "-path", "/workspace/git-duplicate-space"},
				WorkingDir: "/workspace",
				Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "the-git-with-branch"}},
			}}},
		},
	}, {
		desc: "set revision to default value 1",
		task: task,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "simpleTask",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git",
							},
							Name: "gitspace",
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "git-source-the-git-9l9zj",
				Image:      "override-with-git:latest",
				Command:    []string{"/ko-app/git-init"},
				Args:       []string{"-url", "https://github.com/grafeas/kritis", "-revision", "master", "-path", "/workspace/gitspace"},
				WorkingDir: "/workspace",
				Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "the-git"}},
			}}},
		},
	}, {
		desc: "set revision to provdided branch",
		task: task,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "simpleTask",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git-with-branch",
							},
							Name: "gitspace",
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "git-source-the-git-with-branch-9l9zj",
				Image:      "override-with-git:latest",
				Command:    []string{"/ko-app/git-init"},
				Args:       []string{"-url", "https://github.com/grafeas/kritis", "-revision", "branch", "-path", "/workspace/gitspace"},
				WorkingDir: "/workspace",
				Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "the-git-with-branch"}},
			}}},
		},
	}, {
		desc: "git resource as input from previous task",
		task: task,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-from-git",
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
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git",
							},
							Name: "gitspace",
						},
						Paths: []string{"prev-task-path"},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "create-dir-gitspace-mz4c7",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gitspace"},
			}}, {Container: corev1.Container{
				Name:         "source-copy-gitspace-9l9zj",
				Image:        "busybox",
				Command:      []string{"cp", "-r", "prev-task-path/.", "/workspace/gitspace"},
				VolumeMounts: []corev1.VolumeMount{{MountPath: "/pvc", Name: "pipelinerun-pvc"}},
			}}},
			Volumes: []corev1.Volume{{
				Name: "pipelinerun-pvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pipelinerun-pvc"},
				},
			}},
		},
	}, {
		desc: "simple with sslVerify false",
		task: task,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "simpleTask",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git-with-sslVerify-false",
							},
							Name: "gitspace",
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "git-source-the-git-with-sslVerify-false-9l9zj",
				Image:      "override-with-git:latest",
				Command:    []string{"/ko-app/git-init"},
				Args:       []string{"-url", "https://github.com/grafeas/kritis", "-revision", "branch", "-path", "/workspace/gitspace", "-sslVerify=false"},
				WorkingDir: "/workspace",
				Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "the-git-with-sslVerify-false"}},
			}}},
		},
	}, {
		desc: "storage resource as input with target path",
		task: taskWithTargetPath,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-from-gcs",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "storage1",
							},
							Name: "workspace",
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gcsInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "create-dir-storage1-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gcs-dir"},
			}}, {
				Script: `#!/usr/bin/env bash
if [[ "${GOOGLE_APPLICATION_CREDENTIALS}" != "" ]]; then
  echo GOOGLE_APPLICATION_CREDENTIALS is set, activating Service Account...
  gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS}
fi
gsutil cp gs://fake-bucket/rules.zip /workspace/gcs-dir
`,
				Container: corev1.Container{
					Name:  "fetch-storage1-mz4c7",
					Image: "google/cloud-sdk",
				},
			}},
		},
	}, {
		desc: "storage resource as input from previous task",
		task: taskWithTargetPath,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-from-gcs",
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
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "storage1",
							},
							Name: "workspace",
						},
						Paths: []string{"prev-task-path"},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gcsInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "create-dir-workspace-mz4c7",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gcs-dir"},
			}}, {Container: corev1.Container{
				Name:         "source-copy-workspace-9l9zj",
				Image:        "busybox",
				Command:      []string{"cp", "-r", "prev-task-path/.", "/workspace/gcs-dir"},
				VolumeMounts: []corev1.VolumeMount{{MountPath: "/pvc", Name: "pipelinerun-pvc"}},
			}}},
			Volumes: []corev1.Volume{{
				Name: "pipelinerun-pvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pipelinerun-pvc"},
				},
			}},
		},
	}, {
		desc: "invalid gcs resource type name",
		task: task,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-from-invalid-gcs",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "storage-gcs-invalid",
							},
							Name: "workspace",
						},
					}},
				},
			},
		},
		wantErr: true,
	}, {
		desc: "invalid gcs resource type name",
		task: task,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-from-invalid-gcs",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "storage-gcs-invalid",
							},
							Name: "workspace",
						},
					}},
				},
			},
		},
		wantErr: true,
	}, {
		desc: "invalid resource name",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "workspace-invalid",
							Type: "git",
						}}},
				},
			},
		},
		taskRun: taskRun,
		wantErr: true,
	}, {
		desc: "cluster resource with plain text",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: clusterInputs,
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "build-from-repo",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "target-cluster",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "cluster3",
							},
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: clusterInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "kubeconfig-9l9zj",
				Image:   "override-with-kubeconfig-writer:latest",
				Command: []string{"/ko-app/kubeconfigwriter"},
				Args: []string{
					"-clusterConfig", `{"name":"cluster3","type":"cluster","url":"http://10.10.10.10","revision":"","username":"","password":"","namespace":"namespace1","token":"","Insecure":false,"cadata":"bXktY2EtY2VydAo=","secrets":null}`,
				},
			}}},
		},
	}, {
		desc: "cluster resource with secrets",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: clusterInputs,
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "build-from-repo",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "target-cluster",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "cluster2",
							},
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: clusterInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "kubeconfig-9l9zj",
				Image:   "override-with-kubeconfig-writer:latest",
				Command: []string{"/ko-app/kubeconfigwriter"},
				Args: []string{
					"-clusterConfig", `{"name":"cluster2","type":"cluster","url":"http://10.10.10.10","revision":"","username":"","password":"","namespace":"","token":"","Insecure":false,"cadata":null,"secrets":[{"fieldName":"cadata","secretKey":"cadatakey","secretName":"secret1"}]}`,
				},
				Env: []corev1.EnvVar{{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret1",
							},
							Key: "cadatakey",
						},
					},
					Name: "CADATA",
				}},
			}}},
		},
	}, {
		desc: "optional git input resource",
		task: taskWithOptionalGitSources,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo-with-optional-git",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "simpleTask",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git-with-branch",
							},
							Name: "gitspace",
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: optionalGitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "git-source-the-git-with-branch-9l9zj",
				Image:      "override-with-git:latest",
				Command:    []string{"/ko-app/git-init"},
				Args:       []string{"-url", "https://github.com/grafeas/kritis", "-revision", "branch", "-path", "/workspace/gitspace"},
				WorkingDir: "/workspace",
				Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "the-git-with-branch"}},
			}}},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			setUp()
			names.TestingSeed()
			fakekubeclient := fakek8s.NewSimpleClientset()
			got, err := AddInputResource(fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, mockResolveTaskResources(c.taskRun), logger)
			if (err != nil) != c.wantErr {
				t.Errorf("Test: %q; AddInputResource() error = %v, WantErr %v", c.desc, err, c.wantErr)
			}
			if got != nil {
				if d := cmp.Diff(got, c.want); d != "" {
					t.Errorf("Diff:\n%s", d)
				}
			}
		})
	}
}

func TestStorageInputResource(t *testing.T) {
	gcsStorageInputs := &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{{
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name: "gcs-input-resource",
				Type: "storage",
			}}},
	}
	optionalStorageInputs := &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{{
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name:     "gcs-input-resource",
				Type:     "storage",
				Optional: true,
			}}},
	}

	for _, c := range []struct {
		desc    string
		task    *v1alpha1.Task
		taskRun *v1alpha1.TaskRun
		wantErr bool
		want    *v1alpha1.TaskSpec
	}{{
		desc: "inputs with no resource spec and resource ref",
		task: &v1alpha1.Task{
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "gcs-input-resource",
							Type: "storage",
						}}},
				},
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-storage-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "gcs-input-resource",
						},
					}},
				},
			},
		},
		wantErr: true,
	}, {
		desc: "inputs with resource spec and no resource ref",
		task: &v1alpha1.Task{
			Spec: v1alpha1.TaskSpec{
				Inputs: gcsStorageInputs,
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-storage-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "gcs-input-resource",
							ResourceSpec: &v1alpha1.PipelineResourceSpec{
								Type: v1alpha1.PipelineResourceTypeStorage,
								Params: []v1alpha1.ResourceParam{{
									Name:  "Location",
									Value: "gs://fake-bucket/rules.zip",
								}, {
									Name:  "Type",
									Value: "gcs",
								}},
							},
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gcsStorageInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "create-dir-gcs-input-resource-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gcs-input-resource"},
			}}, {
				Script: `#!/usr/bin/env bash
if [[ "${GOOGLE_APPLICATION_CREDENTIALS}" != "" ]]; then
  echo GOOGLE_APPLICATION_CREDENTIALS is set, activating Service Account...
  gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS}
fi
gsutil cp gs://fake-bucket/rules.zip /workspace/gcs-input-resource
`,
				Container: corev1.Container{
					Name:  "fetch-gcs-input-resource-mz4c7",
					Image: "google/cloud-sdk",
				},
			}},
		},
	}, {
		desc: "no inputs",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-storage",
				Namespace: "marshmallow",
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-storage-run",
				Namespace: "marshmallow",
			},
		},
		wantErr: false,
		want:    &v1alpha1.TaskSpec{},
	}, {
		desc: "storage resource as input",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-storage",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: gcsStorageInputs,
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-storage-run",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "gcs-input-resource",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "storage-gcs-keys",
							},
						},
					}},
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: gcsStorageInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "create-dir-storage-gcs-keys-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gcs-input-resource"},
			}}, {
				Script: `#!/usr/bin/env bash
if [[ "${GOOGLE_APPLICATION_CREDENTIALS}" != "" ]]; then
  echo GOOGLE_APPLICATION_CREDENTIALS is set, activating Service Account...
  gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS}
fi
gsutil rsync -d -r gs://fake-bucket/rules.zip /workspace/gcs-input-resource
`,
				Container: corev1.Container{
					Name:  "fetch-storage-gcs-keys-mz4c7",
					Image: "google/cloud-sdk",
					VolumeMounts: []corev1.VolumeMount{
						{Name: "volume-storage-gcs-keys-secret-name", MountPath: "/var/secret/secret-name"},
					},
					Env: []corev1.EnvVar{
						{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/secret-name/key.json"},
					},
				},
			}},
			Volumes: []corev1.Volume{{
				Name:         "volume-storage-gcs-keys-secret-name",
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret-name"}},
			}, {
				Name:         "volume-storage-gcs-keys-secret-name2",
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret-name2"}},
			}},
		},
	}, {
		desc: "optional inputs with no resource spec and no resource ref",
		task: &v1alpha1.Task{
			Spec: v1alpha1.TaskSpec{
				Inputs: optionalStorageInputs,
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-storage-run-with-optional-inputs",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: nil,
					Params:    nil,
				},
			},
		},
		wantErr: false,
		want: &v1alpha1.TaskSpec{
			Inputs: optionalStorageInputs,
			Steps:  nil,
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			setUp()
			fakekubeclient := fakek8s.NewSimpleClientset()
			got, err := AddInputResource(fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, mockResolveTaskResources(c.taskRun), logger)
			if (err != nil) != c.wantErr {
				t.Errorf("Test: %q; AddInputResource() error = %v, WantErr %v", c.desc, err, c.wantErr)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Errorf("Didn't get expected Task spec (-want, +got): %s", d)
			}
		})
	}
}

func TestAddStepsToTaskWithBucketFromConfigMap(t *testing.T) {
	task := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-from-repo",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: gitInputs,
		},
	}
	taskWithTargetPath := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task-with-targetpath",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: gcsInputs,
		},
	}
	taskWithMultipleGcsInputs := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task-with-multiple-gcs-inputs",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: multipleGcsInputs,
		},
	}

	gcsVolumes := []corev1.Volume{
		{
			Name: "volume-bucket-gcs-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "gcs-config",
				},
			},
		},
	}
	gcsVolumeMounts := []corev1.VolumeMount{{Name: "volume-bucket-gcs-config", MountPath: "/var/bucketsecret/gcs-config"}}
	gcsEnv := []corev1.EnvVar{
		{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: "/var/bucketsecret/gcs-config/my-key",
		},
	}

	for _, c := range []struct {
		desc    string
		task    *v1alpha1.Task
		taskRun *v1alpha1.TaskRun
		want    *v1alpha1.TaskSpec
	}{{
		desc: "git resource as input from previous task - copy to bucket",
		task: task,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-from-git",
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
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "the-git",
							},
							Name: "gitspace",
						},
						Paths: []string{"prev-task-path"},
					}},
				},
			},
		},
		want: &v1alpha1.TaskSpec{
			Inputs: gitInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "artifact-dest-mkdir-gitspace-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gitspace"},
			}}, {Container: corev1.Container{
				Name:         "artifact-copy-from-gitspace-mz4c7",
				Image:        "google/cloud-sdk",
				Command:      []string{"gsutil"},
				Args:         []string{"cp", "-P", "-r", "gs://fake-bucket/prev-task-path/*", "/workspace/gitspace"},
				Env:          gcsEnv,
				VolumeMounts: gcsVolumeMounts,
			}}},
			Volumes: gcsVolumes,
		},
	}, {
		desc: "storage resource as input from previous task - copy from bucket",
		task: taskWithTargetPath,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-from-gcs",
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
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "storage1",
							},
							Name: "workspace",
						},
						Paths: []string{"prev-task-path"},
					}},
				},
			},
		},
		want: &v1alpha1.TaskSpec{
			Inputs: gcsInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "artifact-dest-mkdir-workspace-mssqb",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gcs-dir"},
			}}, {Container: corev1.Container{
				Name:         "artifact-copy-from-workspace-78c5n",
				Image:        "google/cloud-sdk",
				Command:      []string{"gsutil"},
				Args:         []string{"cp", "-P", "-r", "gs://fake-bucket/prev-task-path/*", "/workspace/gcs-dir"},
				Env:          gcsEnv,
				VolumeMounts: gcsVolumeMounts,
			}}},
			Volumes: gcsVolumes,
		},
	}, {
		desc: "storage resource with multiple inputs from previous task - copy from bucket",
		task: taskWithMultipleGcsInputs,
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "get-from-gcs",
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
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "storage1",
							},
							Name: "workspace",
						},
						Paths: []string{"prev-task-path"},
					}, {
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "storage2",
							},
							Name: "workspace2",
						},
						Paths: []string{"prev-task-path2"},
					}},
				},
			},
		},
		want: &v1alpha1.TaskSpec{
			Inputs: multipleGcsInputs,
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "artifact-dest-mkdir-workspace-vr6ds",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gcs-dir"},
			}}, {Container: corev1.Container{
				Name:         "artifact-copy-from-workspace-l22wn",
				Image:        "google/cloud-sdk",
				Command:      []string{"gsutil"},
				Args:         []string{"cp", "-P", "-r", "gs://fake-bucket/prev-task-path/*", "/workspace/gcs-dir"},
				Env:          gcsEnv,
				VolumeMounts: gcsVolumeMounts,
			}}, {Container: corev1.Container{
				Name:    "artifact-dest-mkdir-workspace2-6nl7g",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace/gcs-dir"},
			}}, {Container: corev1.Container{
				Name:         "artifact-copy-from-workspace2-j2tds",
				Image:        "google/cloud-sdk",
				Command:      []string{"gsutil"},
				Args:         []string{"cp", "-P", "-r", "gs://fake-bucket/prev-task-path2/*", "/workspace/gcs-dir"},
				Env:          gcsEnv,
				VolumeMounts: gcsVolumeMounts,
			}}},
			Volumes: gcsVolumes,
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			setUp()
			fakekubeclient := fakek8s.NewSimpleClientset(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "tekton-pipelines",
						Name:      artifacts.GetBucketConfigName(),
					},
					Data: map[string]string{
						artifacts.BucketLocationKey:              "gs://fake-bucket",
						artifacts.BucketServiceAccountSecretName: "gcs-config",
						artifacts.BucketServiceAccountSecretKey:  "my-key",
					},
				},
			)
			got, err := AddInputResource(fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, mockResolveTaskResources(c.taskRun), logger)
			if err != nil {
				t.Errorf("Test: %q; AddInputResource() error = %v", c.desc, err)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Errorf("Didn't get expected TaskSpec (-want, +got): %s", d)
			}
		})
	}
}

func mockResolveTaskResources(taskRun *v1alpha1.TaskRun) map[string]v1alpha1.PipelineResourceInterface {
	resolved := make(map[string]v1alpha1.PipelineResourceInterface)
	for _, r := range taskRun.Spec.Inputs.Resources {
		var i v1alpha1.PipelineResourceInterface
		switch {
		case r.ResourceRef != nil && r.ResourceRef.Name != "":
			i = inputResourceInterfaces[r.ResourceRef.Name]
			resolved[r.Name] = i
		case r.ResourceSpec != nil:
			i, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: r.Name,
				},
				Spec: *r.ResourceSpec,
			}, images)
			resolved[r.Name] = i
		default:
			resolved[r.Name] = nil
		}
	}
	return resolved
}
