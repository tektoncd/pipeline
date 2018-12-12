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

// This file contains an object which encapsulated k8s clients which are useful for integration tests.

package test

import (
	"fmt"

	"github.com/knative/build-pipeline/pkg/client/clientset/versioned"
	"github.com/knative/build-pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	knativetest "github.com/knative/pkg/test"
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
func newClients(configPath, clusterName, namespace string) (*clients, error) {
	var err error
	c := &clients{}

	c.KubeClient, err = knativetest.NewKubeClient(configPath, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubeclient from config file at %s: %s", configPath, err)
	}

	cfg, err := knativetest.BuildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to create configuration obj from %s for cluster %s: %s", configPath, clusterName, err)
	}

	cs, err := versioned.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline clientset from config file at %s: %s", configPath, err)
	}
	c.PipelineClient = cs.PipelineV1alpha1().Pipelines(namespace)
	c.TaskClient = cs.PipelineV1alpha1().Tasks(namespace)
	c.TaskRunClient = cs.PipelineV1alpha1().TaskRuns(namespace)
	c.PipelineRunClient = cs.PipelineV1alpha1().PipelineRuns(namespace)
	c.PipelineResourceClient = cs.PipelineV1alpha1().PipelineResources(namespace)
	return c, nil
}
