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
package v1alpha1

import (
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	scheme "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PipelineResourcesGetter has a method to return a PipelineResourceInterface.
// A group's client should implement this interface.
type PipelineResourcesGetter interface {
	PipelineResources(namespace string) PipelineResourceInterface
}

// PipelineResourceInterface has methods to work with PipelineResource resources.
type PipelineResourceInterface interface {
	Create(*v1alpha1.PipelineResource) (*v1alpha1.PipelineResource, error)
	Update(*v1alpha1.PipelineResource) (*v1alpha1.PipelineResource, error)
	UpdateStatus(*v1alpha1.PipelineResource) (*v1alpha1.PipelineResource, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.PipelineResource, error)
	List(opts v1.ListOptions) (*v1alpha1.PipelineResourceList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineResource, err error)
	PipelineResourceExpansion
}

// pipelineResources implements PipelineResourceInterface
type pipelineResources struct {
	client rest.Interface
	ns     string
}

// newPipelineResources returns a PipelineResources
func newPipelineResources(c *TektonV1alpha1Client, namespace string) *pipelineResources {
	return &pipelineResources{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the pipelineResource, and returns the corresponding pipelineResource object, and an error if there is any.
func (c *pipelineResources) Get(name string, options v1.GetOptions) (result *v1alpha1.PipelineResource, err error) {
	result = &v1alpha1.PipelineResource{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("pipelineresources").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PipelineResources that match those selectors.
func (c *pipelineResources) List(opts v1.ListOptions) (result *v1alpha1.PipelineResourceList, err error) {
	result = &v1alpha1.PipelineResourceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("pipelineresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested pipelineResources.
func (c *pipelineResources) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("pipelineresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a pipelineResource and creates it.  Returns the server's representation of the pipelineResource, and an error, if there is any.
func (c *pipelineResources) Create(pipelineResource *v1alpha1.PipelineResource) (result *v1alpha1.PipelineResource, err error) {
	result = &v1alpha1.PipelineResource{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("pipelineresources").
		Body(pipelineResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a pipelineResource and updates it. Returns the server's representation of the pipelineResource, and an error, if there is any.
func (c *pipelineResources) Update(pipelineResource *v1alpha1.PipelineResource) (result *v1alpha1.PipelineResource, err error) {
	result = &v1alpha1.PipelineResource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("pipelineresources").
		Name(pipelineResource.Name).
		Body(pipelineResource).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *pipelineResources) UpdateStatus(pipelineResource *v1alpha1.PipelineResource) (result *v1alpha1.PipelineResource, err error) {
	result = &v1alpha1.PipelineResource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("pipelineresources").
		Name(pipelineResource.Name).
		SubResource("status").
		Body(pipelineResource).
		Do().
		Into(result)
	return
}

// Delete takes name of the pipelineResource and deletes it. Returns an error if one occurs.
func (c *pipelineResources) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("pipelineresources").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *pipelineResources) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("pipelineresources").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched pipelineResource.
func (c *pipelineResources) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineResource, err error) {
	result = &v1alpha1.PipelineResource{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("pipelineresources").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
