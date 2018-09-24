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

// StandardResourcesGetter has a method to return a StandardResourceInterface.
// A group's client should implement this interface.
type StandardResourcesGetter interface {
	StandardResources(namespace string) StandardResourceInterface
}

// StandardResourceInterface has methods to work with StandardResource resources.
type StandardResourceInterface interface {
	Create(*v1alpha1.StandardResource) (*v1alpha1.StandardResource, error)
	Update(*v1alpha1.StandardResource) (*v1alpha1.StandardResource, error)
	UpdateStatus(*v1alpha1.StandardResource) (*v1alpha1.StandardResource, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.StandardResource, error)
	List(opts v1.ListOptions) (*v1alpha1.StandardResourceList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.StandardResource, err error)
	StandardResourceExpansion
}

// standardResources implements StandardResourceInterface
type standardResources struct {
	client rest.Interface
	ns     string
}

// newStandardResources returns a StandardResources
func newStandardResources(c *PipelineV1alpha1Client, namespace string) *standardResources {
	return &standardResources{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the standardResource, and returns the corresponding standardResource object, and an error if there is any.
func (c *standardResources) Get(name string, options v1.GetOptions) (result *v1alpha1.StandardResource, err error) {
	result = &v1alpha1.StandardResource{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("standardresources").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of StandardResources that match those selectors.
func (c *standardResources) List(opts v1.ListOptions) (result *v1alpha1.StandardResourceList, err error) {
	result = &v1alpha1.StandardResourceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("standardresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested standardResources.
func (c *standardResources) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("standardresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a standardResource and creates it.  Returns the server's representation of the standardResource, and an error, if there is any.
func (c *standardResources) Create(standardResource *v1alpha1.StandardResource) (result *v1alpha1.StandardResource, err error) {
	result = &v1alpha1.StandardResource{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("standardresources").
		Body(standardResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a standardResource and updates it. Returns the server's representation of the standardResource, and an error, if there is any.
func (c *standardResources) Update(standardResource *v1alpha1.StandardResource) (result *v1alpha1.StandardResource, err error) {
	result = &v1alpha1.StandardResource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("standardresources").
		Name(standardResource.Name).
		Body(standardResource).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *standardResources) UpdateStatus(standardResource *v1alpha1.StandardResource) (result *v1alpha1.StandardResource, err error) {
	result = &v1alpha1.StandardResource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("standardresources").
		Name(standardResource.Name).
		SubResource("status").
		Body(standardResource).
		Do().
		Into(result)
	return
}

// Delete takes name of the standardResource and deletes it. Returns an error if one occurs.
func (c *standardResources) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("standardresources").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *standardResources) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("standardresources").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched standardResource.
func (c *standardResources) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.StandardResource, err error) {
	result = &v1alpha1.StandardResource{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("standardresources").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
