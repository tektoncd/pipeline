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
	scheme "github.com/knative/build-pipeline/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PipelineParamsesGetter has a method to return a PipelineParamsInterface.
// A group's client should implement this interface.
type PipelineParamsesGetter interface {
	PipelineParamses(namespace string) PipelineParamsInterface
}

// PipelineParamsInterface has methods to work with PipelineParams resources.
type PipelineParamsInterface interface {
	Create(*v1alpha1.PipelineParams) (*v1alpha1.PipelineParams, error)
	Update(*v1alpha1.PipelineParams) (*v1alpha1.PipelineParams, error)
	UpdateStatus(*v1alpha1.PipelineParams) (*v1alpha1.PipelineParams, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.PipelineParams, error)
	List(opts v1.ListOptions) (*v1alpha1.PipelineParamsList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineParams, err error)
	PipelineParamsExpansion
}

// pipelineParamses implements PipelineParamsInterface
type pipelineParamses struct {
	client rest.Interface
	ns     string
}

// newPipelineParamses returns a PipelineParamses
func newPipelineParamses(c *PipelineV1alpha1Client, namespace string) *pipelineParamses {
	return &pipelineParamses{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the pipelineParams, and returns the corresponding pipelineParams object, and an error if there is any.
func (c *pipelineParamses) Get(name string, options v1.GetOptions) (result *v1alpha1.PipelineParams, err error) {
	result = &v1alpha1.PipelineParams{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("pipelineparamses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PipelineParamses that match those selectors.
func (c *pipelineParamses) List(opts v1.ListOptions) (result *v1alpha1.PipelineParamsList, err error) {
	result = &v1alpha1.PipelineParamsList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("pipelineparamses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested pipelineParamses.
func (c *pipelineParamses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("pipelineparamses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a pipelineParams and creates it.  Returns the server's representation of the pipelineParams, and an error, if there is any.
func (c *pipelineParamses) Create(pipelineParams *v1alpha1.PipelineParams) (result *v1alpha1.PipelineParams, err error) {
	result = &v1alpha1.PipelineParams{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("pipelineparamses").
		Body(pipelineParams).
		Do().
		Into(result)
	return
}

// Update takes the representation of a pipelineParams and updates it. Returns the server's representation of the pipelineParams, and an error, if there is any.
func (c *pipelineParamses) Update(pipelineParams *v1alpha1.PipelineParams) (result *v1alpha1.PipelineParams, err error) {
	result = &v1alpha1.PipelineParams{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("pipelineparamses").
		Name(pipelineParams.Name).
		Body(pipelineParams).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *pipelineParamses) UpdateStatus(pipelineParams *v1alpha1.PipelineParams) (result *v1alpha1.PipelineParams, err error) {
	result = &v1alpha1.PipelineParams{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("pipelineparamses").
		Name(pipelineParams.Name).
		SubResource("status").
		Body(pipelineParams).
		Do().
		Into(result)
	return
}

// Delete takes name of the pipelineParams and deletes it. Returns an error if one occurs.
func (c *pipelineParamses) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("pipelineparamses").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *pipelineParamses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("pipelineparamses").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched pipelineParams.
func (c *pipelineParamses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineParams, err error) {
	result = &v1alpha1.PipelineParams{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("pipelineparamses").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
