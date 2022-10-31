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

/*
Get access to client objects

To initialize client objects you can use the setup function. It returns a clients struct
that contains initialized clients for accessing:

	- Kubernetes objects
	- Pipelines (https://github.com/tektoncd/pipeline#pipeline)

For example, to create a Pipeline

	_, err = clients.V1beta1PipelineClient.Pipelines.Create(test.Pipeline(namespaceName, pipelineName))

And you can use the client to clean up resources created by your test

	func tearDown(clients *test.Clients) {
	    if clients != nil {
	        clients.Delete([]string{routeName}, []string{configName})
	    }
	}

*/

package test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	resolutionversioned "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned"
	resolutionv1alpha1 "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned/typed/resolution/v1alpha1"
	resourceversioned "github.com/tektoncd/pipeline/pkg/client/resource/clientset/versioned"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/client/resource/clientset/versioned/typed/resource/v1alpha1"
	"k8s.io/client-go/kubernetes"
	knativetest "knative.dev/pkg/test"
)

// clients holds instances of interfaces for making requests to the Pipeline controllers.
type clients struct {
	KubeClient kubernetes.Interface

	V1beta1PipelineClient           v1beta1.PipelineInterface
	V1beta1ClusterTaskClient        v1beta1.ClusterTaskInterface
	V1beta1TaskClient               v1beta1.TaskInterface
	V1beta1TaskRunClient            v1beta1.TaskRunInterface
	V1beta1PipelineRunClient        v1beta1.PipelineRunInterface
	V1alpha1PipelineResourceClient  resourcev1alpha1.PipelineResourceInterface
	V1alpha1RunClient               v1alpha1.RunInterface
	V1alpha1ResolutionRequestclient resolutionv1alpha1.ResolutionRequestInterface
}

// newClients instantiates and returns several clientsets required for making requests to the
// Pipeline cluster specified by the combination of clusterName and configPath. Clients can
// make requests within namespace.
func newClients(t *testing.T, configPath, clusterName, namespace string) *clients {
	t.Helper()
	var err error
	c := &clients{}

	cfg, err := knativetest.BuildClientConfig(configPath, clusterName)
	if err != nil {
		t.Fatalf("failed to create configuration obj from %s for cluster %s: %s", configPath, clusterName, err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create kubeclient from config file at %s: %s", configPath, err)
	}
	c.KubeClient = kubeClient

	cs, err := versioned.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create pipeline clientset from config file at %s: %s", configPath, err)
	}
	rcs, err := resourceversioned.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create pipeline resource clientset from config file at %s: %s", configPath, err)
	}
	rrcs, err := resolutionversioned.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create resolution clientset from config file at %s: %s", configPath, err)
	}
	c.V1beta1PipelineClient = cs.TektonV1beta1().Pipelines(namespace)
	c.V1beta1ClusterTaskClient = cs.TektonV1beta1().ClusterTasks()
	c.V1beta1TaskClient = cs.TektonV1beta1().Tasks(namespace)
	c.V1beta1TaskRunClient = cs.TektonV1beta1().TaskRuns(namespace)
	c.V1beta1PipelineRunClient = cs.TektonV1beta1().PipelineRuns(namespace)
	c.V1alpha1PipelineResourceClient = rcs.TektonV1alpha1().PipelineResources(namespace)
	c.V1alpha1RunClient = cs.TektonV1alpha1().Runs(namespace)
	c.V1alpha1ResolutionRequestclient = rrcs.ResolutionV1alpha1().ResolutionRequests(namespace)
	return c
}
