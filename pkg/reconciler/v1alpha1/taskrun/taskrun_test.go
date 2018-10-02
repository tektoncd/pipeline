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
)

func TestReconcile(t *testing.T) {
	taskname := "test-task"
	taskruns := []*v1alpha1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-success",
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: v1alpha1.TaskRef{
					Name:       taskname,
					APIVersion: "a1",
				},
			}},
	}

	buildSpec := buildv1alpha1.BuildSpec{
		Template: &buildv1alpha1.TemplateInstantiationSpec{
			Name: "kaniko",
			Arguments: []buildv1alpha1.ArgumentSpec{
				buildv1alpha1.ArgumentSpec{
					Name:  "DOCKERFILE",
					Value: "${PATH_TO_DOCKERFILE}",
				},
				buildv1alpha1.ArgumentSpec{
					Name:  "REGISTRY",
					Value: "${REGISTRY}",
				},
			}},
	}

	tasks := []*v1alpha1.Task{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      taskname,
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskSpec{
				BuildSpec: &buildSpec,
			}},
	}
	d := testData{
		taskruns: taskruns,
		tasks:    tasks,
	}
	testcases := []struct {
		name         string
		taskRun      string
		shdErr       bool
		shdMakebuild bool
		log          string
	}{
		{"success", "foo/test-taskrun-run-success", false, true, ""},
		// all test modes
		// create a build
		// update a status
		// say finished
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c, logs, client := getController(d)
			err := c.Reconciler.Reconcile(context.Background(), tc.taskRun)
			if tc.shdErr != (err != nil) {
				t.Errorf("expected to see error %t. Got error %v", tc.shdErr, err)
			}
			if tc.log == "" && logs.Len() > 0 {
				t.Errorf("expected to see no error log. However found errors in logs: %v", logs)
			} else if tc.log != "" && logs.FilterMessage(tc.log).Len() == 0 {
				m := getLogMessages(logs)
				t.Errorf("Log lines diff %s", cmp.Diff(tc.log, m))
			} else if tc.shdMakebuild {
				if err == nil {
					if len(client.Actions()) == 0 {
						t.Errorf("Expected actions to be logged in the buildclient, got none")
					}
				}
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

	for _, tr := range d.taskruns {
		taskRunInformer.Informer().GetIndexer().Add(tr)
	}
	for _, t := range d.tasks {
		taskInformer.Informer().GetIndexer().Add(t)
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
	taskruns []*v1alpha1.TaskRun
	tasks    []*v1alpha1.Task
}
