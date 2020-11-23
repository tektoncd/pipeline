/*
Copyright 2020 The Tekton Authors

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

// This file contains an object which encapsulates k8s clients which are useful for e2e tests.

package clients

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	pipelinev1alpha1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	resourceversioned "github.com/tektoncd/pipeline/pkg/client/resource/clientset/versioned"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/client/resource/clientset/versioned/typed/resource/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	logging "knative.dev/pkg/logging"
	knativetest "knative.dev/pkg/test"
)

// Clients holds instances of interfaces for making requests to Knative Serving.
type Clients struct {
	KubeClient          *knativetest.KubeClient
	PipelineAlphaClient *PipelineAlphaClients
	PipelineBetaClient  *PipelineBetaClients
	Dynamic             dynamic.Interface
}

// ServingAlphaClients holds instances of interfaces for making requests to knative serving clients.
type PipelineAlphaClients struct {
	PipelineResources resourcev1alpha1.PipelineResourceInterface
	Runs              pipelinev1alpha1.RunInterface
	Conditions        pipelinev1alpha1.ConditionInterface
}

// ServingClients holds instances of interfaces for making requests to knative serving clients.
type PipelineBetaClients struct {
	Pipelines    pipelinev1beta1.PipelineInterface
	PipelineRuns pipelinev1beta1.PipelineRunInterface
	Tasks        pipelinev1beta1.TaskInterface
	TaskRuns     pipelinev1beta1.TaskRunInterface
	ClusterTasks pipelinev1beta1.ClusterTaskInterface
}

// NewClients instantiates and returns several clientsets required for making request to the
// Knative Serving cluster specified by the combination of clusterName and configPath. Clients can
// make requests within namespace.
func NewClients(configPath, clusterName, namespace string) (*Clients, error) {
	cfg, err := knativetest.BuildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	// We poll, so set our limits high.
	cfg.QPS = 100
	cfg.Burst = 200

	return NewClientsFromConfig(cfg, namespace)
}

// NewClientsFromConfig instantiates and returns several clientsets required for making request to the
// Knative Serving cluster specified by the rest Config. Clients can make requests within namespace.
func NewClientsFromConfig(cfg *rest.Config, namespace string) (*Clients, error) {
	clients := &Clients{}
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	clients.KubeClient = &knativetest.KubeClient{Kube: kubeClient}

	clients.PipelineBetaClient, err = newPipelineBetaClients(cfg, namespace)
	if err != nil {
		return nil, err
	}
	clients.PipelineAlphaClient, err = newPipelineAlphaClients(cfg, namespace)
	if err != nil {
		return nil, err
	}

	clients.Dynamic, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clients, nil
}

// newPipelineBetaClients instantiates and returns the pipeline clientset required to make requests to the
// tekton pipeline cluster. Those are v1beta1 clients.
func newPipelineBetaClients(cfg *rest.Config, namespace string) (*PipelineBetaClients, error) {
	cs, err := versioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &PipelineBetaClients{
		Pipelines:    cs.TektonV1beta1().Pipelines(namespace),
		PipelineRuns: cs.TektonV1beta1().PipelineRuns(namespace),
		Tasks:        cs.TektonV1beta1().Tasks(namespace),
		TaskRuns:     cs.TektonV1beta1().TaskRuns(namespace),
		ClusterTasks: cs.TektonV1beta1().ClusterTasks(),
	}, nil
}

// newPipelineAlphaClients instantiates and returns the pipeline clientset required to make requests to the
// tekton pipeline cluster. Those are v1Alpha1 clients.
func newPipelineAlphaClients(cfg *rest.Config, namespace string) (*PipelineAlphaClients, error) {
	cs, err := versioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	rcs, err := resourceversioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &PipelineAlphaClients{
		PipelineResources: rcs.TektonV1alpha1().PipelineResources(namespace),
		Runs:              cs.TektonV1alpha1().Runs(namespace),
		Conditions:        cs.TektonV1alpha1().Conditions(namespace),
	}, nil
}

type Deletion struct {
	client interface {
		Delete(ctx context.Context, name string, options metav1.DeleteOptions) error
	}
	items []string
}

// Delete will delete all v1beta1 object passed (Pipeline, PipelineRun, Task, TaskRun, ClusterTask)
// if clients have been successfully initialized.
func (clients *PipelineBetaClients) Delete(deletions ...Deletion) []error {
	return clientsDelete(deletions...)
}

func (clients *PipelineBetaClients) PipelinesD(items []string) Deletion {
	return Deletion{
		client: clients.Pipelines,
		items:  items,
	}
}

func (clients *PipelineBetaClients) PipelineRunsD(items []string) Deletion {
	return Deletion{
		client: clients.PipelineRuns,
		items:  items,
	}
}

func (clients *PipelineBetaClients) TasksD(items []string) Deletion {
	return Deletion{
		client: clients.Tasks,
		items:  items,
	}
}

func (clients *PipelineBetaClients) TaskRunsD(items []string) Deletion {
	return Deletion{
		client: clients.TaskRuns,
		items:  items,
	}
}

func (clients *PipelineBetaClients) ClusterTasksD(items []string) Deletion {
	return Deletion{
		client: clients.ClusterTasks,
		items:  items,
	}
}

// Delete will delete all v1alpha1 object passed (Pipeline, PipelineRun, Task, TaskRun, ClusterTask)
// if clients have been successfully initialized.
func (clients *PipelineAlphaClients) Delete(deletions ...Deletion) []error {
	return clientsDelete(deletions...)
}

func (clients *PipelineAlphaClients) PipelineResourcesD(items []string) Deletion {
	return Deletion{
		client: clients.PipelineResources,
		items:  items,
	}
}

func (clients *PipelineAlphaClients) RunsD(items []string) Deletion {
	return Deletion{
		client: clients.Runs,
		items:  items,
	}
}

func (clients *PipelineAlphaClients) ConditionsD(items []string) Deletion {
	return Deletion{
		client: clients.Conditions,
		items:  items,
	}
}

func clientsDelete(deletions ...Deletion) []error {
	propPolicy := metav1.DeletePropagationForeground
	dopt := metav1.DeleteOptions{
		PropagationPolicy: &propPolicy,
	}

	var errs []error
	for _, deletion := range deletions {
		if deletion.client == nil {
			continue
		}

		for _, item := range deletion.items {
			if item == "" {
				continue
			}

			if err := deletion.client.Delete(context.Background(), item, dopt); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

// Key is used as the key for associating information with a context.Context.
type Key struct{}

func WithClients(ctx context.Context, c *Clients) context.Context {
	return context.WithValue(ctx, Key{}, c)
}

// Get extracts the Clients from the context
func Get(ctx context.Context) *Clients {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		logging.FromContext(ctx).Panic(
			"Unable to fetch Clients from context.")
	}
	return untyped.(*Clients)
}
