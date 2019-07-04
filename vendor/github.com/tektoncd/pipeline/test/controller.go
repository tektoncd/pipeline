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
	"testing"

	"github.com/knative/pkg/controller"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	informersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
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

// Data represents the desired state of the system (i.e. existing resources) to seed controllers
// with.
type Data struct {
	PipelineRuns      []*v1alpha1.PipelineRun
	Pipelines         []*v1alpha1.Pipeline
	TaskRuns          []*v1alpha1.TaskRun
	Tasks             []*v1alpha1.Task
	ClusterTasks      []*v1alpha1.ClusterTask
	PipelineResources []*v1alpha1.PipelineResource
	Pods              []*corev1.Pod
	Namespaces        []*corev1.Namespace
}

// Clients holds references to clients which are useful for reconciler tests.
type Clients struct {
	Pipeline *fakepipelineclientset.Clientset
	Kube     *fakekubeclientset.Clientset
}

// Informers holds references to informers which are useful for reconciler tests.
type Informers struct {
	PipelineRun      informersv1alpha1.PipelineRunInformer
	Pipeline         informersv1alpha1.PipelineInformer
	TaskRun          informersv1alpha1.TaskRunInformer
	Task             informersv1alpha1.TaskInformer
	ClusterTask      informersv1alpha1.ClusterTaskInformer
	PipelineResource informersv1alpha1.PipelineResourceInformer
	Pod              coreinformers.PodInformer
}

// TestAssets holds references to the controller, logs, clients, and informers.
type TestAssets struct {
	Controller *controller.Impl
	Logs       *observer.ObservedLogs
	Clients    Clients
	Informers  Informers
}

// SeedTestData returns Clients and Informers populated with the
// given Data.
func SeedTestData(t *testing.T, d Data) (Clients, Informers) {
	objs := []runtime.Object{}
	for _, r := range d.PipelineResources {
		objs = append(objs, r)
	}
	for _, p := range d.Pipelines {
		objs = append(objs, p)
	}
	for _, pr := range d.PipelineRuns {
		objs = append(objs, pr)
	}
	for _, t := range d.Tasks {
		objs = append(objs, t)
	}
	for _, ct := range d.ClusterTasks {
		objs = append(objs, ct)
	}
	for _, tr := range d.TaskRuns {
		objs = append(objs, tr)
	}

	kubeObjs := []runtime.Object{}
	for _, p := range d.Pods {
		kubeObjs = append(kubeObjs, p)
	}
	for _, n := range d.Namespaces {
		kubeObjs = append(kubeObjs, n)
	}
	c := Clients{
		Pipeline: fakepipelineclientset.NewSimpleClientset(objs...),
		Kube:     fakekubeclientset.NewSimpleClientset(kubeObjs...),
	}
	sharedInformer := informers.NewSharedInformerFactory(c.Pipeline, 0)
	kubeInformer := kubeinformers.NewSharedInformerFactory(c.Kube, 0)

	i := Informers{
		PipelineRun:      sharedInformer.Tekton().V1alpha1().PipelineRuns(),
		Pipeline:         sharedInformer.Tekton().V1alpha1().Pipelines(),
		TaskRun:          sharedInformer.Tekton().V1alpha1().TaskRuns(),
		Task:             sharedInformer.Tekton().V1alpha1().Tasks(),
		ClusterTask:      sharedInformer.Tekton().V1alpha1().ClusterTasks(),
		PipelineResource: sharedInformer.Tekton().V1alpha1().PipelineResources(),
		Pod:              kubeInformer.Core().V1().Pods(),
	}

	for _, pr := range d.PipelineRuns {
		if err := i.PipelineRun.Informer().GetIndexer().Add(pr); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range d.Pipelines {
		if err := i.Pipeline.Informer().GetIndexer().Add(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, tr := range d.TaskRuns {
		if err := i.TaskRun.Informer().GetIndexer().Add(tr); err != nil {
			t.Fatal(err)
		}
	}
	for _, ta := range d.Tasks {
		if err := i.Task.Informer().GetIndexer().Add(ta); err != nil {
			t.Fatal(err)
		}
	}
	for _, ct := range d.ClusterTasks {
		if err := i.ClusterTask.Informer().GetIndexer().Add(ct); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range d.PipelineResources {
		if err := i.PipelineResource.Informer().GetIndexer().Add(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range d.Pods {
		if err := i.Pod.Informer().GetIndexer().Add(p); err != nil {
			t.Fatal(err)
		}
	}
	return c, i
}
