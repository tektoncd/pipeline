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

package test

import (
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions"
	informersv1alpha1 "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/pipelinerun"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun"
	fakebuildclientset "github.com/knative/build/pkg/client/clientset/versioned/fake"
	buildinformers "github.com/knative/build/pkg/client/informers/externalversions"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

// GetLogMessages returns a list of all string logs in logs.
func GetLogMessages(logs *observer.ObservedLogs) []string {
	messages := []string{}
	for _, l := range logs.All() {
		messages = append(messages, l.Message)
	}
	return messages
}

// TestData represents the desired state of the system (i.e. existing resources) to seed controllers
// with.
type TestData struct {
	PipelineRuns      []*v1alpha1.PipelineRun
	Pipelines         []*v1alpha1.Pipeline
	TaskRuns          []*v1alpha1.TaskRun
	Tasks             []*v1alpha1.Task
	PipelineParams    []*v1alpha1.PipelineParams
	PipelineResources []*v1alpha1.PipelineResource
}

func seedTestData(d TestData) (*fakepipelineclientset.Clientset,
	informersv1alpha1.PipelineRunInformer, informersv1alpha1.PipelineInformer,
	informersv1alpha1.TaskRunInformer, informersv1alpha1.TaskInformer,
	informersv1alpha1.PipelineParamsInformer, informersv1alpha1.PipelineResourceInformer) {
	objs := []runtime.Object{}
	for _, pr := range d.PipelineRuns {
		objs = append(objs, pr)
	}
	for _, p := range d.Pipelines {
		objs = append(objs, p)
	}
	for _, tr := range d.TaskRuns {
		objs = append(objs, tr)
	}
	for _, t := range d.Tasks {
		objs = append(objs, t)
	}
	for _, r := range d.PipelineResources {
		objs = append(objs, r)
	}
	pipelineClient := fakepipelineclientset.NewSimpleClientset(objs...)

	sharedInformer := informers.NewSharedInformerFactory(pipelineClient, 0)
	pipelineRunsInformer := sharedInformer.Pipeline().V1alpha1().PipelineRuns()
	pipelineInformer := sharedInformer.Pipeline().V1alpha1().Pipelines()
	taskRunInformer := sharedInformer.Pipeline().V1alpha1().TaskRuns()
	taskInformer := sharedInformer.Pipeline().V1alpha1().Tasks()
	pipelineParamsInformer := sharedInformer.Pipeline().V1alpha1().PipelineParamses()
	resourceInformer := sharedInformer.Pipeline().V1alpha1().PipelineResources()

	for _, pr := range d.PipelineRuns {
		pipelineRunsInformer.Informer().GetIndexer().Add(pr)
	}
	for _, p := range d.Pipelines {
		pipelineInformer.Informer().GetIndexer().Add(p)
	}
	for _, tr := range d.TaskRuns {
		taskRunInformer.Informer().GetIndexer().Add(tr)
	}
	for _, t := range d.Tasks {
		taskInformer.Informer().GetIndexer().Add(t)
	}
	for _, t := range d.PipelineParams {
		pipelineParamsInformer.Informer().GetIndexer().Add(t)
	}
	for _, r := range d.PipelineResources {
		resourceInformer.Informer().GetIndexer().Add(r)
	}
	return pipelineClient, pipelineRunsInformer, pipelineInformer, taskRunInformer, taskInformer, pipelineParamsInformer, resourceInformer
}

// GetTaskRunController returns an instance of the TaskRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func GetTaskRunController(d TestData) (*controller.Impl, *observer.ObservedLogs, *fakepipelineclientset.Clientset, *fakebuildclientset.Clientset) {
	pipelineClient, _, _, taskRunInformer, taskInformer, _, resourceInformer := seedTestData(d)

	buildClient := fakebuildclientset.NewSimpleClientset()
	buildInformerFactory := buildinformers.NewSharedInformerFactory(buildClient, 0)
	buildInformer := buildInformerFactory.Build().V1alpha1().Builds()

	// Create a log observer to record all error logs.
	observer, logs := observer.New(zap.ErrorLevel)
	return taskrun.NewController(
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
	), logs, pipelineClient, buildClient
}

// GetPipelineRunController returns an instance of the PipelineRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func GetPipelineRunController(d TestData) (*controller.Impl, *observer.ObservedLogs, *fakepipelineclientset.Clientset) {
	pipelineClient, pipelineRunsInformer, pipelineInformer, taskRunInformer, taskInformer, pipelineParamsInformer, _ := seedTestData(d)
	// Create a log observer to record all error logs.
	observer, logs := observer.New(zap.ErrorLevel)
	return pipelinerun.NewController(
		reconciler.Options{
			Logger:            zap.New(observer).Sugar(),
			KubeClientSet:     fakekubeclientset.NewSimpleClientset(),
			PipelineClientSet: pipelineClient,
		},
		pipelineRunsInformer,
		pipelineInformer,
		taskInformer,
		taskRunInformer,
		pipelineParamsInformer,
	), logs, pipelineClient
}
