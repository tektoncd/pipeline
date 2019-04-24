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

// TektonListenersGetter has a method to return a TektonListenerInterface.
// A group's client should implement this interface.
type TektonListenersGetter interface {
	TektonListeners(namespace string) TektonListenerInterface
}

// TektonListenerInterface has methods to work with TektonListener resources.
type TektonListenerInterface interface {
	Create(*v1alpha1.TektonListener) (*v1alpha1.TektonListener, error)
	Update(*v1alpha1.TektonListener) (*v1alpha1.TektonListener, error)
	UpdateStatus(*v1alpha1.TektonListener) (*v1alpha1.TektonListener, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.TektonListener, error)
	List(opts v1.ListOptions) (*v1alpha1.TektonListenerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.TektonListener, err error)
	TektonListenerExpansion
}

// tektonListeners implements TektonListenerInterface
type tektonListeners struct {
	client rest.Interface
	ns     string
}

// newTektonListeners returns a TektonListeners
func newTektonListeners(c *TektonV1alpha1Client, namespace string) *tektonListeners {
	return &tektonListeners{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the tektonListener, and returns the corresponding tektonListener object, and an error if there is any.
func (c *tektonListeners) Get(name string, options v1.GetOptions) (result *v1alpha1.TektonListener, err error) {
	result = &v1alpha1.TektonListener{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("tektonlisteners").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of TektonListeners that match those selectors.
func (c *tektonListeners) List(opts v1.ListOptions) (result *v1alpha1.TektonListenerList, err error) {
	result = &v1alpha1.TektonListenerList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("tektonlisteners").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested tektonListeners.
func (c *tektonListeners) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("tektonlisteners").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a tektonListener and creates it.  Returns the server's representation of the tektonListener, and an error, if there is any.
func (c *tektonListeners) Create(tektonListener *v1alpha1.TektonListener) (result *v1alpha1.TektonListener, err error) {
	result = &v1alpha1.TektonListener{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("tektonlisteners").
		Body(tektonListener).
		Do().
		Into(result)
	return
}

// Update takes the representation of a tektonListener and updates it. Returns the server's representation of the tektonListener, and an error, if there is any.
func (c *tektonListeners) Update(tektonListener *v1alpha1.TektonListener) (result *v1alpha1.TektonListener, err error) {
	result = &v1alpha1.TektonListener{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("tektonlisteners").
		Name(tektonListener.Name).
		Body(tektonListener).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *tektonListeners) UpdateStatus(tektonListener *v1alpha1.TektonListener) (result *v1alpha1.TektonListener, err error) {
	result = &v1alpha1.TektonListener{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("tektonlisteners").
		Name(tektonListener.Name).
		SubResource("status").
		Body(tektonListener).
		Do().
		Into(result)
	return
}

// Delete takes name of the tektonListener and deletes it. Returns an error if one occurs.
func (c *tektonListeners) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("tektonlisteners").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *tektonListeners) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("tektonlisteners").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched tektonListener.
func (c *tektonListeners) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.TektonListener, err error) {
	result = &v1alpha1.TektonListener{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("tektonlisteners").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
