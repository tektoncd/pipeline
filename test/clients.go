/*
Copyright 2018 Knative Authors LLC
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

	_, err = clients.PipelineClient.Pipelines.Create(test.Pipeline(namespaceName, pipelineName))

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

	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
)

// clients holds instances of interfaces for making requests to the Pipeline controllers.
type clients struct {
	KubeClient *knativetest.KubeClient

	PipelineClient         v1alpha1.PipelineInterface
	TaskClient             v1alpha1.TaskInterface
	TaskRunClient          v1alpha1.TaskRunInterface
	PipelineRunClient      v1alpha1.PipelineRunInterface
	PipelineResourceClient v1alpha1.PipelineResourceInterface
}

// newClients instantiates and returns several clientsets required for making requests to the
// Pipeline cluster specified by the combination of clusterName and configPath. Clients can
// make requests within namespace.
func newClients(t *testing.T, configPath, clusterName, namespace string) *clients {
	t.Helper()
	var err error
	c := &clients{}

	c.KubeClient, err = knativetest.NewKubeClient(configPath, clusterName)
	if err != nil {
		t.Fatalf("failed to create kubeclient from config file at %s: %s", configPath, err)
	}

	cfg, err := knativetest.BuildClientConfig(configPath, clusterName)
	if err != nil {
		t.Fatalf("failed to create configuration obj from %s for cluster %s: %s", configPath, clusterName, err)
	}

	cs, err := versioned.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create pipeline clientset from config file at %s: %s", configPath, err)
	}
	c.PipelineClient = cs.TektonV1alpha1().Pipelines(namespace)
	c.TaskClient = cs.TektonV1alpha1().Tasks(namespace)
	c.TaskRunClient = cs.TektonV1alpha1().TaskRuns(namespace)
	c.PipelineRunClient = cs.TektonV1alpha1().PipelineRuns(namespace)
	c.PipelineResourceClient = cs.TektonV1alpha1().PipelineResources(namespace)
	return c
}
