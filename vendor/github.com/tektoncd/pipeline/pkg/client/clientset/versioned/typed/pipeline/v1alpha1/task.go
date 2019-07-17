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

// TasksGetter has a method to return a TaskInterface.
// A group's client should implement this interface.
type TasksGetter interface {
	Tasks(namespace string) TaskInterface
}

// TaskInterface has methods to work with Task resources.
type TaskInterface interface {
	Create(*v1alpha1.Task) (*v1alpha1.Task, error)
	Update(*v1alpha1.Task) (*v1alpha1.Task, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.Task, error)
	List(opts v1.ListOptions) (*v1alpha1.TaskList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Task, err error)
	TaskExpansion
}

// tasks implements TaskInterface
type tasks struct {
	client rest.Interface
	ns     string
}

// newTasks returns a Tasks
func newTasks(c *TektonV1alpha1Client, namespace string) *tasks {
	return &tasks{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the task, and returns the corresponding task object, and an error if there is any.
func (c *tasks) Get(name string, options v1.GetOptions) (result *v1alpha1.Task, err error) {
	result = &v1alpha1.Task{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("tasks").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Tasks that match those selectors.
func (c *tasks) List(opts v1.ListOptions) (result *v1alpha1.TaskList, err error) {
	result = &v1alpha1.TaskList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("tasks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested tasks.
func (c *tasks) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("tasks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a task and creates it.  Returns the server's representation of the task, and an error, if there is any.
func (c *tasks) Create(task *v1alpha1.Task) (result *v1alpha1.Task, err error) {
	result = &v1alpha1.Task{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("tasks").
		Body(task).
		Do().
		Into(result)
	return
}

// Update takes the representation of a task and updates it. Returns the server's representation of the task, and an error, if there is any.
func (c *tasks) Update(task *v1alpha1.Task) (result *v1alpha1.Task, err error) {
	result = &v1alpha1.Task{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("tasks").
		Name(task.Name).
		Body(task).
		Do().
		Into(result)
	return
}

// Delete takes name of the task and deletes it. Returns an error if one occurs.
func (c *tasks) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("tasks").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *tasks) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("tasks").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched task.
func (c *tasks) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Task, err error) {
	result = &v1alpha1.Task{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("tasks").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
