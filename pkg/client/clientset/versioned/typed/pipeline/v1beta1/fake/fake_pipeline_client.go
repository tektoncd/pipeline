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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1beta1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeTektonV1beta1 struct {
	*testing.Fake
}

func (c *FakeTektonV1beta1) CustomRuns(namespace string) v1beta1.CustomRunInterface {
	return &FakeCustomRuns{c, namespace}
}

func (c *FakeTektonV1beta1) Pipelines(namespace string) v1beta1.PipelineInterface {
	return &FakePipelines{c, namespace}
}

func (c *FakeTektonV1beta1) PipelineRuns(namespace string) v1beta1.PipelineRunInterface {
	return &FakePipelineRuns{c, namespace}
}

func (c *FakeTektonV1beta1) StepActions(namespace string) v1beta1.StepActionInterface {
	return &FakeStepActions{c, namespace}
}

func (c *FakeTektonV1beta1) Tasks(namespace string) v1beta1.TaskInterface {
	return &FakeTasks{c, namespace}
}

func (c *FakeTektonV1beta1) TaskRuns(namespace string) v1beta1.TaskRunInterface {
	return &FakeTaskRuns{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeTektonV1beta1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
