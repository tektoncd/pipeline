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
package fake

import (
	v1alpha1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeTektonV1alpha1 struct {
	*testing.Fake
}

func (c *FakeTektonV1alpha1) ClusterTasks() v1alpha1.ClusterTaskInterface {
	return &FakeClusterTasks{c}
}

func (c *FakeTektonV1alpha1) Pipelines(namespace string) v1alpha1.PipelineInterface {
	return &FakePipelines{c, namespace}
}

func (c *FakeTektonV1alpha1) PipelineResources(namespace string) v1alpha1.PipelineResourceInterface {
	return &FakePipelineResources{c, namespace}
}

func (c *FakeTektonV1alpha1) PipelineRuns(namespace string) v1alpha1.PipelineRunInterface {
	return &FakePipelineRuns{c, namespace}
}

func (c *FakeTektonV1alpha1) Tasks(namespace string) v1alpha1.TaskInterface {
	return &FakeTasks{c, namespace}
}

func (c *FakeTektonV1alpha1) TaskRuns(namespace string) v1alpha1.TaskRunInterface {
	return &FakeTaskRuns{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeTektonV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
