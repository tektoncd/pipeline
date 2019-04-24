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
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTektonListeners implements TektonListenerInterface
type FakeTektonListeners struct {
	Fake *FakeTektonV1alpha1
	ns   string
}

var tektonlistenersResource = schema.GroupVersionResource{Group: "tekton.dev", Version: "v1alpha1", Resource: "tektonlisteners"}

var tektonlistenersKind = schema.GroupVersionKind{Group: "tekton.dev", Version: "v1alpha1", Kind: "TektonListener"}

// Get takes name of the tektonListener, and returns the corresponding tektonListener object, and an error if there is any.
func (c *FakeTektonListeners) Get(name string, options v1.GetOptions) (result *v1alpha1.TektonListener, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(tektonlistenersResource, c.ns, name), &v1alpha1.TektonListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonListener), err
}

// List takes label and field selectors, and returns the list of TektonListeners that match those selectors.
func (c *FakeTektonListeners) List(opts v1.ListOptions) (result *v1alpha1.TektonListenerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(tektonlistenersResource, tektonlistenersKind, c.ns, opts), &v1alpha1.TektonListenerList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.TektonListenerList{ListMeta: obj.(*v1alpha1.TektonListenerList).ListMeta}
	for _, item := range obj.(*v1alpha1.TektonListenerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested tektonListeners.
func (c *FakeTektonListeners) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(tektonlistenersResource, c.ns, opts))

}

// Create takes the representation of a tektonListener and creates it.  Returns the server's representation of the tektonListener, and an error, if there is any.
func (c *FakeTektonListeners) Create(tektonListener *v1alpha1.TektonListener) (result *v1alpha1.TektonListener, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(tektonlistenersResource, c.ns, tektonListener), &v1alpha1.TektonListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonListener), err
}

// Update takes the representation of a tektonListener and updates it. Returns the server's representation of the tektonListener, and an error, if there is any.
func (c *FakeTektonListeners) Update(tektonListener *v1alpha1.TektonListener) (result *v1alpha1.TektonListener, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(tektonlistenersResource, c.ns, tektonListener), &v1alpha1.TektonListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonListener), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeTektonListeners) UpdateStatus(tektonListener *v1alpha1.TektonListener) (*v1alpha1.TektonListener, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(tektonlistenersResource, "status", c.ns, tektonListener), &v1alpha1.TektonListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonListener), err
}

// Delete takes name of the tektonListener and deletes it. Returns an error if one occurs.
func (c *FakeTektonListeners) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(tektonlistenersResource, c.ns, name), &v1alpha1.TektonListener{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeTektonListeners) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(tektonlistenersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.TektonListenerList{})
	return err
}

// Patch applies the patch and returns the patched tektonListener.
func (c *FakeTektonListeners) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.TektonListener, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(tektonlistenersResource, c.ns, name, data, subresources...), &v1alpha1.TektonListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonListener), err
}
