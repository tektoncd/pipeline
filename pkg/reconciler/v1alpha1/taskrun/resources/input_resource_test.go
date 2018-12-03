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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	fakeclientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/logging"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	pipelineResourceLister listers.PipelineResourceLister
	logger                 *zap.SugaredLogger
)

func setUp() {
	logger, _ = logging.NewLogger("", "")
	fakeClient := fakeclientset.NewSimpleClientset()
	sharedInfomer := informers.NewSharedInformerFactory(fakeClient, 0)
	pipelineResourceInformer := sharedInfomer.Pipeline().V1alpha1().PipelineResources()
	pipelineResourceLister = pipelineResourceInformer.Lister()

	rs := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "the-git",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []v1alpha1.Param{{
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
			Params: []v1alpha1.Param{{
				Name:  "Url",
				Value: "https://github.com/grafeas/kritis",
			}, {
				Name:  "Revision",
				Value: "branch",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster2",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "cluster",
			Params: []v1alpha1.Param{{
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
			Params: []v1alpha1.Param{{
				Name:  "Url",
				Value: "http://10.10.10.10",
			}, {
				Name: "CAdata",
				// echo "my-ca-cert" | base64
				Value: "bXktY2EtY2VydAo=",
			}},
		},
	}}
	for _, r := range rs {
		pipelineResourceInformer.Informer().GetIndexer().Add(r)
	}
}

func build() *buildv1alpha1.Build {
	boolTrue := true
	return &buildv1alpha1.Build{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Build",
			APIVersion: "build.knative.dev/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-from-repo",
			Namespace: "marshmallow",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "pipeline.knative.dev/v1alpha1",
				Kind:               "TaskRun",
				Name:               "build-from-repo-run",
				Controller:         &boolTrue,
				BlockOwnerDeletion: &boolTrue,
			}},
		},
		Spec: buildv1alpha1.BuildSpec{},
	}
}
func TestAddResourceToBuild(t *testing.T) {
	boolTrue := true
	task := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-from-repo",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: "git",
				}},
			},
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
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "the-git",
					},
					Name: "workspace",
				}},
			},
		},
	}

	for _, c := range []struct {
		desc    string
		task    *v1alpha1.Task
		taskRun *v1alpha1.TaskRun
		build   *buildv1alpha1.Build
		wantErr bool
		want    *buildv1alpha1.Build
	}{{
		desc:    "simple with default revision",
		task:    task,
		taskRun: taskRun,
		build:   build(),
		wantErr: false,
		want: &buildv1alpha1.Build{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Build",
				APIVersion: "build.knative.dev/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "pipeline.knative.dev/v1alpha1",
					Kind:               "TaskRun",
					Name:               "build-from-repo-run",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: buildv1alpha1.BuildSpec{
				Sources: []buildv1alpha1.SourceSpec{{
					Git: &buildv1alpha1.GitSourceSpec{
						Url:      "https://github.com/grafeas/kritis",
						Revision: "master",
					},
					Name: "the-git",
				}},
			},
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
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "the-git-with-branch",
						},
						Name: "workspace",
					}},
				},
			},
		},
		build:   build(),
		wantErr: false,
		want: &buildv1alpha1.Build{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Build",
				APIVersion: "build.knative.dev/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "pipeline.knative.dev/v1alpha1",
						Kind:               "TaskRun",
						Name:               "build-from-repo-run",
						Controller:         &boolTrue,
						BlockOwnerDeletion: &boolTrue,
					},
				},
			},
			Spec: buildv1alpha1.BuildSpec{
				Sources: []buildv1alpha1.SourceSpec{{
					Name: "the-git-with-branch",
					Git: &buildv1alpha1.GitSourceSpec{
						Url:      "https://github.com/grafeas/kritis",
						Revision: "branch",
					},
				}},
			},
		},
	}, {
		desc: "set revision to default value",
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
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "the-git",
						},
						Name: "workspace",
					}},
				},
			},
		},
		build:   build(),
		wantErr: false,
		want: &buildv1alpha1.Build{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Build",
				APIVersion: "build.knative.dev/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "pipeline.knative.dev/v1alpha1",
					Kind:               "TaskRun",
					Name:               "build-from-repo-run",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: buildv1alpha1.BuildSpec{
				Sources: []buildv1alpha1.SourceSpec{{
					Git: &buildv1alpha1.GitSourceSpec{
						Url:      "https://github.com/grafeas/kritis",
						Revision: "master",
					},
					Name: "the-git",
				}},
			},
		},
	}, {
		desc: "set revision to default value",
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
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "the-git-with-branch",
						},
						Name: "workspace",
					}},
				},
			},
		},
		build:   build(),
		wantErr: false,
		want: &buildv1alpha1.Build{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Build",
				APIVersion: "build.knative.dev/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "pipeline.knative.dev/v1alpha1",
						Kind:               "TaskRun",
						Name:               "build-from-repo-run",
						Controller:         &boolTrue,
						BlockOwnerDeletion: &boolTrue,
					},
				},
			},
			Spec: buildv1alpha1.BuildSpec{
				Sources: []buildv1alpha1.SourceSpec{{
					Git: &buildv1alpha1.GitSourceSpec{
						Url:      "https://github.com/grafeas/kritis",
						Revision: "branch",
					},
					Name: "the-git-with-branch",
				}},
			},
		},
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
						Name: "workspace-invalid",
						Type: "git",
					}},
				},
			},
		},
		taskRun: taskRun,
		build:   build(),
		wantErr: true,
		want:    nil,
	}, {
		desc: "cluster resource with plain text",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "target-cluster",
						Type: "cluster",
					}},
				},
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
						Name: "target-cluster",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "cluster3",
						},
					}},
				},
			},
		},
		build:   build(),
		wantErr: false,
		want: &buildv1alpha1.Build{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Build",
				APIVersion: "build.knative.dev/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "pipeline.knative.dev/v1alpha1",
					Kind:               "TaskRun",
					Name:               "build-from-repo-run",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:  "kubeconfig",
					Image: "override-with-kubeconfig-writer:latest",
					Args: []string{
						"-clusterConfig", `{"name":"cluster3","type":"cluster","url":"http://10.10.10.10","revision":"","username":"","password":"","token":"","Insecure":false,"cadata":"bXktY2EtY2VydAo=","secrets":null}`,
					},
				}},
			},
		},
	}, {
		desc: "cluster resource with secrets",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "target-cluster",
						Type: "cluster",
					}},
				},
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
						Name: "target-cluster",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "cluster2",
						},
					}},
				},
			},
		},
		build:   build(),
		wantErr: false,
		want: &buildv1alpha1.Build{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Build",
				APIVersion: "build.knative.dev/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-from-repo",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "pipeline.knative.dev/v1alpha1",
					Kind:               "TaskRun",
					Name:               "build-from-repo-run",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:  "kubeconfig",
					Image: "override-with-kubeconfig-writer:latest",
					Args: []string{
						"-clusterConfig", `{"name":"cluster2","type":"cluster","url":"http://10.10.10.10","revision":"","username":"","password":"","token":"","Insecure":false,"cadata":null,"secrets":[{"fieldName":"cadata","secretKey":"cadatakey","secretName":"secret1"}]}`,
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
				}},
			},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			setUp()
			got, err := AddInputResource(c.build, c.task.Name, &c.task.Spec, c.taskRun, pipelineResourceLister, logger)
			if (err != nil) != c.wantErr {
				t.Errorf("Test: %q; NewControllerConfigFromConfigMap() error = %v, WantErr %v", c.desc, err, c.wantErr)
			}
			if d := cmp.Diff(got, c.want); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
		})
	}
}
