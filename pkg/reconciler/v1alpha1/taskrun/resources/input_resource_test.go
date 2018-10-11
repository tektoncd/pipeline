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

	res := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []v1alpha1.Param{
				v1alpha1.Param{
					Name:  "Url",
					Value: "https://github.com/grafeas/kritis",
				},
			},
		},
	}
	pipelineResourceInformer.Informer().GetIndexer().Add(res)
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
				Resources: []v1alpha1.TaskResource{
					v1alpha1.TaskResource{
						Name: "workspace",
						Type: "git",
					},
				},
			},
		},
	}
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-from-repo-run",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "simpleTask",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.PipelineResourceVersion{
					v1alpha1.PipelineResourceVersion{
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "workspace",
						},
						Version: "master",
					},
				},
			},
		},
	}
	build := &buildv1alpha1.Build{
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
		Spec: buildv1alpha1.BuildSpec{},
	}

	for _, c := range []struct {
		desc    string
		task    *v1alpha1.Task
		taskRun *v1alpha1.TaskRun
		build   *buildv1alpha1.Build
		wantErr bool
		want    *buildv1alpha1.Build
	}{{
		desc:    "simple",
		task:    task,
		taskRun: taskRun,
		build:   build,
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
				Source: &buildv1alpha1.SourceSpec{
					Git: &buildv1alpha1.GitSourceSpec{
						Url:      "https://github.com/grafeas/kritis",
						Revision: "master",
					},
				},
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
				TaskRef: v1alpha1.TaskRef{
					Name: "simpleTask",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.PipelineResourceVersion{
						v1alpha1.PipelineResourceVersion{
							ResourceRef: v1alpha1.PipelineResourceRef{
								Name: "workspace",
							},
							Version: "branch",
						},
					},
				},
			},
		},
		build:   build,
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
				Source: &buildv1alpha1.SourceSpec{
					Git: &buildv1alpha1.GitSourceSpec{
						Url:      "https://github.com/grafeas/kritis",
						Revision: "branch",
					},
				},
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
				TaskRef: v1alpha1.TaskRef{
					Name: "simpleTask",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.PipelineResourceVersion{
						v1alpha1.PipelineResourceVersion{
							ResourceRef: v1alpha1.PipelineResourceRef{
								Name: "workspace",
							},
						},
					},
				},
			},
		},
		build:   build,
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
				Source: &buildv1alpha1.SourceSpec{
					Git: &buildv1alpha1.GitSourceSpec{
						Url:      "https://github.com/grafeas/kritis",
						Revision: "master",
					},
				},
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
					Resources: []v1alpha1.TaskResource{
						v1alpha1.TaskResource{
							Name: "workspace-invalid",
							Type: "git",
						},
					},
				},
			},
		},
		taskRun: taskRun,
		build:   build,
		wantErr: true,
		want:    nil,
	},
	} {
		setUp()
		t.Run(c.desc, func(t *testing.T) {
			got, err := AddInputResource(c.build, c.task, c.taskRun, pipelineResourceLister, logger)
			if (err != nil) != c.wantErr {
				t.Errorf("Test: %q; NewControllerConfigFromConfigMap() error = %v, WantErr %v", c.desc, err, c.wantErr)
			}
			if d := cmp.Diff(got, c.want); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
		})
	}
}
