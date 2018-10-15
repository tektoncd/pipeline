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

package taskrun

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions"
	"github.com/knative/build-pipeline/pkg/reconciler"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	fakebuildclientset "github.com/knative/build/pkg/client/clientset/versioned/fake"
	buildinformers "github.com/knative/build/pkg/client/informers/externalversions"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

var simpleTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-task",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		BuildSpec: &buildv1alpha1.BuildSpec{
			Steps: []corev1.Container{
				{
					Name:  "simple-step",
					Image: "foo",
				},
			},
		},
	},
}

var templatedTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "task-with-templating",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		BuildSpec: &buildv1alpha1.BuildSpec{
			Steps: []corev1.Container{
				{
					Name:  "mycontainer",
					Image: "myimage",
					Args:  []string{"--my-arg=${inputs.params.myarg}"},
				},
				{
					Name:  "myothercontainer",
					Image: "myotherimage",
					Args:  []string{"--my-other-arg=${inputs.resources.git-resource.url}"},
				},
			},
		},
	},
}

var gitResource = &v1alpha1.PipelineResource{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "git-resource",
		Namespace: "foo",
	},
	Spec: v1alpha1.PipelineResourceSpec{
		Type: "git",
		Params: []v1alpha1.Param{
			{
				Name:  "URL",
				Value: "https://foo.git",
			},
		},
	},
}

func TestReconcileBuildsCreated(t *testing.T) {
	taskruns := []*v1alpha1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-success",
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: v1alpha1.TaskRef{
					Name:       "test-task",
					APIVersion: "a1",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-templating",
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: v1alpha1.TaskRef{
					Name:       "task-with-templating",
					APIVersion: "a1",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Params: []v1alpha1.Param{
						{
							Name:  "myarg",
							Value: "foo",
						},
					},
					Resources: []v1alpha1.PipelineResourceVersion{
						{
							ResourceRef: v1alpha1.PipelineResourceRef{
								Name:       "git-resource",
								APIVersion: "a1",
							},
							Version: "myversion",
						},
					},
				},
			},
		},
	}

	d := testData{
		taskruns:  taskruns,
		tasks:     []*v1alpha1.Task{simpleTask, templatedTask},
		resources: []*v1alpha1.PipelineResource{gitResource},
	}
	testcases := []struct {
		name            string
		taskRun         string
		wantedBuildSpec buildv1alpha1.BuildSpec
	}{
		{
			name:    "success",
			taskRun: "foo/test-taskrun-run-success",
			wantedBuildSpec: buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{
					{
						Name:  "simple-step",
						Image: "foo",
					},
				},
			},
		},
		{
			name:    "params",
			taskRun: "foo/test-taskrun-templating",
			wantedBuildSpec: buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{
					{
						Name:  "mycontainer",
						Image: "myimage",
						Args:  []string{"--my-arg=foo"},
					},
					{
						Name:  "myothercontainer",
						Image: "myotherimage",
						Args:  []string{"--my-other-arg=https://foo.git"},
					},
				},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, client := getController(d)
			if err := c.Reconciler.Reconcile(context.Background(), tc.taskRun); err != nil {
				t.Errorf("expected no error. Got error %v", err)
			}

			if len(client.Actions()) == 0 {
				t.Errorf("Expected actions to be logged in the buildclient, got none")
			}
			build := client.Actions()[0].(ktesting.CreateAction).GetObject().(*buildv1alpha1.Build)
			if d := cmp.Diff(build.Spec, tc.wantedBuildSpec); d != "" {
				t.Errorf("buildspec doesn't match, diff: %s", d)
			}
		})
	}
}

func TestReconcileBuildCreationErrors(t *testing.T) {
	taskRuns := []*v1alpha1.TaskRun{
		&v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "notaskrun",
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: v1alpha1.TaskRef{
					Name:       "notask",
					APIVersion: "a1",
				},
			},
		},
	}

	tasks := []*v1alpha1.Task{
		simpleTask,
	}

	d := testData{
		taskruns: taskRuns,
		tasks:    tasks,
	}

	testcases := []struct {
		name    string
		taskRun string
	}{
		{
			name:    "task run with no task",
			taskRun: "foo/notaskrun",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, client := getController(d)
			if err := c.Reconciler.Reconcile(context.Background(), tc.taskRun); err == nil {
				t.Error("Expected reconcile to error. Got nil")
			}
			if len(client.Actions()) != 0 {
				t.Errorf("expected no actions to be created by the reconciler, got %v", client.Actions())
			}
		})
	}

}
func getController(d testData) (*controller.Impl, *observer.ObservedLogs, *fakebuildclientset.Clientset) {
	pipelineClient := fakepipelineclientset.NewSimpleClientset()
	buildClient := fakebuildclientset.NewSimpleClientset()

	sharedInformer := informers.NewSharedInformerFactory(pipelineClient, 0)

	buildInformerFactory := buildinformers.NewSharedInformerFactory(buildClient, time.Second*30)
	buildInformer := buildInformerFactory.Build().V1alpha1().Builds()

	taskRunInformer := sharedInformer.Pipeline().V1alpha1().TaskRuns()
	taskInformer := sharedInformer.Pipeline().V1alpha1().Tasks()
	resourceInformer := sharedInformer.Pipeline().V1alpha1().PipelineResources()

	for _, tr := range d.taskruns {
		taskRunInformer.Informer().GetIndexer().Add(tr)
	}
	for _, t := range d.tasks {
		taskInformer.Informer().GetIndexer().Add(t)
	}

	for _, r := range d.resources {
		resourceInformer.Informer().GetIndexer().Add(r)
	}

	// Create a log observer to record all error logs.
	observer, logs := observer.New(zap.ErrorLevel)
	return NewController(
		reconciler.Options{
			Logger:            zap.New(observer).Sugar(),
			KubeClientSet:     fakekubeclientset.NewSimpleClientset(),
			PipelineClientSet: pipelineClient,
			BuildClientSet:    buildClient,
		},
		taskRunInformer,
		taskInformer,
		buildInformer,
		resourceInformer,
	), logs, buildClient
}

func getLogMessages(logs *observer.ObservedLogs) []string {
	messages := []string{}
	for _, l := range logs.All() {
		messages = append(messages, l.Message)
	}
	return messages
}

type testData struct {
	taskruns  []*v1alpha1.TaskRun
	tasks     []*v1alpha1.Task
	resources []*v1alpha1.PipelineResource
}
