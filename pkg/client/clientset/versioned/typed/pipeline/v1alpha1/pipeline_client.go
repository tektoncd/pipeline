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
package v1alpha1

import (
	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/client/clientset/versioned/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type PipelineV1alpha1Interface interface {
	RESTClient() rest.Interface
	PipelinesGetter
	PipelineParamsesGetter
	PipelineRunsGetter
	StandardResourcesGetter
	TasksGetter
	TaskRunsGetter
}

// PipelineV1alpha1Client is used to interact with features provided by the pipeline.knative.dev group.
type PipelineV1alpha1Client struct {
	restClient rest.Interface
}

func (c *PipelineV1alpha1Client) Pipelines(namespace string) PipelineInterface {
	return newPipelines(c, namespace)
}

func (c *PipelineV1alpha1Client) PipelineParamses(namespace string) PipelineParamsInterface {
	return newPipelineParamses(c, namespace)
}

func (c *PipelineV1alpha1Client) PipelineRuns(namespace string) PipelineRunInterface {
	return newPipelineRuns(c, namespace)
}

func (c *PipelineV1alpha1Client) StandardResources(namespace string) StandardResourceInterface {
	return newStandardResources(c, namespace)
}

func (c *PipelineV1alpha1Client) Tasks(namespace string) TaskInterface {
	return newTasks(c, namespace)
}

func (c *PipelineV1alpha1Client) TaskRuns(namespace string) TaskRunInterface {
	return newTaskRuns(c, namespace)
}

// NewForConfig creates a new PipelineV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*PipelineV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &PipelineV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new PipelineV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *PipelineV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new PipelineV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *PipelineV1alpha1Client {
	return &PipelineV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *PipelineV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
