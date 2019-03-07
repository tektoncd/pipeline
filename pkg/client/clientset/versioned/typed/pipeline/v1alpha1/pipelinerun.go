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
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	scheme "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PipelineRunsGetter has a method to return a PipelineRunInterface.
// A group's client should implement this interface.
type PipelineRunsGetter interface {
	PipelineRuns(namespace string) PipelineRunInterface
}

// PipelineRunInterface has methods to work with PipelineRun resources.
type PipelineRunInterface interface {
	Create(*v1alpha1.PipelineRun) (*v1alpha1.PipelineRun, error)
	Update(*v1alpha1.PipelineRun) (*v1alpha1.PipelineRun, error)
	UpdateStatus(*v1alpha1.PipelineRun) (*v1alpha1.PipelineRun, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.PipelineRun, error)
	List(opts v1.ListOptions) (*v1alpha1.PipelineRunList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineRun, err error)
	PipelineRunExpansion
}

// pipelineRuns implements PipelineRunInterface
type pipelineRuns struct {
	client rest.Interface
	ns     string
}

// newPipelineRuns returns a PipelineRuns
func newPipelineRuns(c *TektonV1alpha1Client, namespace string) *pipelineRuns {
	return &pipelineRuns{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the pipelineRun, and returns the corresponding pipelineRun object, and an error if there is any.
func (c *pipelineRuns) Get(name string, options v1.GetOptions) (result *v1alpha1.PipelineRun, err error) {
	result = &v1alpha1.PipelineRun{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("pipelineruns").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PipelineRuns that match those selectors.
func (c *pipelineRuns) List(opts v1.ListOptions) (result *v1alpha1.PipelineRunList, err error) {
	result = &v1alpha1.PipelineRunList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("pipelineruns").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested pipelineRuns.
func (c *pipelineRuns) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("pipelineruns").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a pipelineRun and creates it.  Returns the server's representation of the pipelineRun, and an error, if there is any.
func (c *pipelineRuns) Create(pipelineRun *v1alpha1.PipelineRun) (result *v1alpha1.PipelineRun, err error) {
	result = &v1alpha1.PipelineRun{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("pipelineruns").
		Body(pipelineRun).
		Do().
		Into(result)
	return
}

// Update takes the representation of a pipelineRun and updates it. Returns the server's representation of the pipelineRun, and an error, if there is any.
func (c *pipelineRuns) Update(pipelineRun *v1alpha1.PipelineRun) (result *v1alpha1.PipelineRun, err error) {
	result = &v1alpha1.PipelineRun{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("pipelineruns").
		Name(pipelineRun.Name).
		Body(pipelineRun).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *pipelineRuns) UpdateStatus(pipelineRun *v1alpha1.PipelineRun) (result *v1alpha1.PipelineRun, err error) {
	result = &v1alpha1.PipelineRun{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("pipelineruns").
		Name(pipelineRun.Name).
		SubResource("status").
		Body(pipelineRun).
		Do().
		Into(result)
	return
}

// Delete takes name of the pipelineRun and deletes it. Returns an error if one occurs.
func (c *pipelineRuns) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("pipelineruns").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *pipelineRuns) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("pipelineruns").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched pipelineRun.
func (c *pipelineRuns) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineRun, err error) {
	result = &v1alpha1.PipelineRun{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("pipelineruns").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
