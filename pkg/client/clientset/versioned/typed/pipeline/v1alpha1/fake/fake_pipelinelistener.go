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

// FakePipelineListeners implements PipelineListenerInterface
type FakePipelineListeners struct {
	Fake *FakeTektonV1alpha1
	ns   string
}

var pipelinelistenersResource = schema.GroupVersionResource{Group: "tekton.dev", Version: "v1alpha1", Resource: "pipelinelisteners"}

var pipelinelistenersKind = schema.GroupVersionKind{Group: "tekton.dev", Version: "v1alpha1", Kind: "PipelineListener"}

// Get takes name of the pipelineListener, and returns the corresponding pipelineListener object, and an error if there is any.
func (c *FakePipelineListeners) Get(name string, options v1.GetOptions) (result *v1alpha1.PipelineListener, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(pipelinelistenersResource, c.ns, name), &v1alpha1.PipelineListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListener), err
}

// List takes label and field selectors, and returns the list of PipelineListeners that match those selectors.
func (c *FakePipelineListeners) List(opts v1.ListOptions) (result *v1alpha1.PipelineListenerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(pipelinelistenersResource, pipelinelistenersKind, c.ns, opts), &v1alpha1.PipelineListenerList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.PipelineListenerList{ListMeta: obj.(*v1alpha1.PipelineListenerList).ListMeta}
	for _, item := range obj.(*v1alpha1.PipelineListenerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested pipelineListeners.
func (c *FakePipelineListeners) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(pipelinelistenersResource, c.ns, opts))

}

// Create takes the representation of a pipelineListener and creates it.  Returns the server's representation of the pipelineListener, and an error, if there is any.
func (c *FakePipelineListeners) Create(pipelineListener *v1alpha1.PipelineListener) (result *v1alpha1.PipelineListener, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(pipelinelistenersResource, c.ns, pipelineListener), &v1alpha1.PipelineListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListener), err
}

// Update takes the representation of a pipelineListener and updates it. Returns the server's representation of the pipelineListener, and an error, if there is any.
func (c *FakePipelineListeners) Update(pipelineListener *v1alpha1.PipelineListener) (result *v1alpha1.PipelineListener, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(pipelinelistenersResource, c.ns, pipelineListener), &v1alpha1.PipelineListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListener), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePipelineListeners) UpdateStatus(pipelineListener *v1alpha1.PipelineListener) (*v1alpha1.PipelineListener, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(pipelinelistenersResource, "status", c.ns, pipelineListener), &v1alpha1.PipelineListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListener), err
}

// Delete takes name of the pipelineListener and deletes it. Returns an error if one occurs.
func (c *FakePipelineListeners) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(pipelinelistenersResource, c.ns, name), &v1alpha1.PipelineListener{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePipelineListeners) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(pipelinelistenersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.PipelineListenerList{})
	return err
}

// Patch applies the patch and returns the patched pipelineListener.
func (c *FakePipelineListeners) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineListener, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(pipelinelistenersResource, c.ns, name, data, subresources...), &v1alpha1.PipelineListener{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListener), err
}
