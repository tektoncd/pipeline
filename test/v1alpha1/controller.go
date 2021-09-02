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

package test

import (
	"context"
	"testing"

	// Link in the fakes so they get injected into injection.Fake
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	informersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	fakepipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client/fake"
	fakeclustertaskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/clustertask/fake"
	fakeconditioninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/condition/fake"
	fakepipelineinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/pipeline/fake"
	fakepipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/pipelinerun/fake"
	faketaskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/task/fake"
	faketaskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/taskrun/fake"
	fakeresourceclientset "github.com/tektoncd/pipeline/pkg/client/resource/clientset/versioned/fake"
	resourceinformersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/resource/informers/externalversions/resource/v1alpha1"
	fakeresourceclient "github.com/tektoncd/pipeline/pkg/client/resource/injection/client/fake"
	fakeresourceinformer "github.com/tektoncd/pipeline/pkg/client/resource/injection/informers/resource/v1alpha1/pipelineresource/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakefilteredpodinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod/filtered/fake"
	"knative.dev/pkg/controller"
)

// Data represents the desired state of the system (i.e. existing resources) to seed controllers
// with.
type Data struct {
	PipelineRuns      []*v1alpha1.PipelineRun
	Pipelines         []*v1alpha1.Pipeline
	TaskRuns          []*v1alpha1.TaskRun
	Tasks             []*v1alpha1.Task
	ClusterTasks      []*v1alpha1.ClusterTask
	PipelineResources []*v1alpha1.PipelineResource
	Conditions        []*v1alpha1.Condition
	Pods              []*corev1.Pod
	Namespaces        []*corev1.Namespace
}

// Clients holds references to clients which are useful for reconciler tests.
type Clients struct {
	Pipeline *fakepipelineclientset.Clientset
	Resource *fakeresourceclientset.Clientset
	Kube     *fakekubeclientset.Clientset
}

// Informers holds references to informers which are useful for reconciler tests.
type Informers struct {
	PipelineRun      informersv1alpha1.PipelineRunInformer
	Pipeline         informersv1alpha1.PipelineInformer
	TaskRun          informersv1alpha1.TaskRunInformer
	Task             informersv1alpha1.TaskInformer
	ClusterTask      informersv1alpha1.ClusterTaskInformer
	PipelineResource resourceinformersv1alpha1.PipelineResourceInformer
	Condition        informersv1alpha1.ConditionInformer
	Pod              coreinformers.PodInformer
}

// Assets holds references to the controller, logs, clients, and informers.
type Assets struct {
	Controller *controller.Impl
	Clients    Clients
}

// SeedTestData returns Clients and Informers populated with the
// given Data.
// nolint: revive
func SeedTestData(t *testing.T, ctx context.Context, d Data) (Clients, Informers) {
	c := Clients{
		Kube:     fakekubeclient.Get(ctx),
		Pipeline: fakepipelineclient.Get(ctx),
		Resource: fakeresourceclient.Get(ctx),
	}

	i := Informers{
		PipelineRun:      fakepipelineruninformer.Get(ctx),
		Pipeline:         fakepipelineinformer.Get(ctx),
		TaskRun:          faketaskruninformer.Get(ctx),
		Task:             faketaskinformer.Get(ctx),
		ClusterTask:      fakeclustertaskinformer.Get(ctx),
		PipelineResource: fakeresourceinformer.Get(ctx),
		Condition:        fakeconditioninformer.Get(ctx),
		Pod:              fakefilteredpodinformer.Get(ctx, v1alpha1.ManagedByLabelKey),
	}

	for _, pr := range d.PipelineRuns {
		if err := i.PipelineRun.Informer().GetIndexer().Add(pr); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pipeline.TektonV1alpha1().PipelineRuns(pr.Namespace).Create(ctx, pr, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range d.Pipelines {
		if err := i.Pipeline.Informer().GetIndexer().Add(p); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pipeline.TektonV1alpha1().Pipelines(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tr := range d.TaskRuns {
		if err := i.TaskRun.Informer().GetIndexer().Add(tr); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pipeline.TektonV1alpha1().TaskRuns(tr.Namespace).Create(ctx, tr, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, ta := range d.Tasks {
		if err := i.Task.Informer().GetIndexer().Add(ta); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pipeline.TektonV1alpha1().Tasks(ta.Namespace).Create(ctx, ta, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, ct := range d.ClusterTasks {
		if err := i.ClusterTask.Informer().GetIndexer().Add(ct); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pipeline.TektonV1alpha1().ClusterTasks().Create(ctx, ct, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range d.PipelineResources {
		if err := i.PipelineResource.Informer().GetIndexer().Add(r); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Resource.TektonV1alpha1().PipelineResources(r.Namespace).Create(ctx, r, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, cond := range d.Conditions {
		if err := i.Condition.Informer().GetIndexer().Add(cond); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pipeline.TektonV1alpha1().Conditions(cond.Namespace).Create(ctx, cond, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range d.Pods {
		if err := i.Pod.Informer().GetIndexer().Add(p); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Kube.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range d.Namespaces {
		if _, err := c.Kube.CoreV1().Namespaces().Create(ctx, n, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Pipeline.ClearActions()
	c.Kube.ClearActions()
	return c, i
}
