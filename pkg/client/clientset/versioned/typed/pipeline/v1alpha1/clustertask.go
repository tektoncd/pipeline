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

// ClusterTasksGetter has a method to return a ClusterTaskInterface.
// A group's client should implement this interface.
type ClusterTasksGetter interface {
	ClusterTasks() ClusterTaskInterface
}

// ClusterTaskInterface has methods to work with ClusterTask resources.
type ClusterTaskInterface interface {
	Create(*v1alpha1.ClusterTask) (*v1alpha1.ClusterTask, error)
	Update(*v1alpha1.ClusterTask) (*v1alpha1.ClusterTask, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.ClusterTask, error)
	List(opts v1.ListOptions) (*v1alpha1.ClusterTaskList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ClusterTask, err error)
	ClusterTaskExpansion
}

// clusterTasks implements ClusterTaskInterface
type clusterTasks struct {
	client rest.Interface
}

// newClusterTasks returns a ClusterTasks
func newClusterTasks(c *TektonV1alpha1Client) *clusterTasks {
	return &clusterTasks{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterTask, and returns the corresponding clusterTask object, and an error if there is any.
func (c *clusterTasks) Get(name string, options v1.GetOptions) (result *v1alpha1.ClusterTask, err error) {
	result = &v1alpha1.ClusterTask{}
	err = c.client.Get().
		Resource("clustertasks").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterTasks that match those selectors.
func (c *clusterTasks) List(opts v1.ListOptions) (result *v1alpha1.ClusterTaskList, err error) {
	result = &v1alpha1.ClusterTaskList{}
	err = c.client.Get().
		Resource("clustertasks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterTasks.
func (c *clusterTasks) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("clustertasks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterTask and creates it.  Returns the server's representation of the clusterTask, and an error, if there is any.
func (c *clusterTasks) Create(clusterTask *v1alpha1.ClusterTask) (result *v1alpha1.ClusterTask, err error) {
	result = &v1alpha1.ClusterTask{}
	err = c.client.Post().
		Resource("clustertasks").
		Body(clusterTask).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterTask and updates it. Returns the server's representation of the clusterTask, and an error, if there is any.
func (c *clusterTasks) Update(clusterTask *v1alpha1.ClusterTask) (result *v1alpha1.ClusterTask, err error) {
	result = &v1alpha1.ClusterTask{}
	err = c.client.Put().
		Resource("clustertasks").
		Name(clusterTask.Name).
		Body(clusterTask).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterTask and deletes it. Returns an error if one occurs.
func (c *clusterTasks) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clustertasks").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterTasks) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("clustertasks").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterTask.
func (c *clusterTasks) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ClusterTask, err error) {
	result = &v1alpha1.ClusterTask{}
	err = c.client.Patch(pt).
		Resource("clustertasks").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
