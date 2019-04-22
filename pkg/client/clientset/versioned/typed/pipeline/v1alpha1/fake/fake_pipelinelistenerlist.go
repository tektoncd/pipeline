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
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePipelineListenerLists implements PipelineListenerListInterface
type FakePipelineListenerLists struct {
	Fake *FakeTektonV1alpha1
	ns   string
}

var pipelinelistenerlistsResource = schema.GroupVersionResource{Group: "tekton.dev", Version: "v1alpha1", Resource: "pipelinelistenerlists"}

var pipelinelistenerlistsKind = schema.GroupVersionKind{Group: "tekton.dev", Version: "v1alpha1", Kind: "PipelineListenerList"}

// Get takes name of the pipelineListenerList, and returns the corresponding pipelineListenerList object, and an error if there is any.
func (c *FakePipelineListenerLists) Get(name string, options v1.GetOptions) (result *v1alpha1.PipelineListenerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(pipelinelistenerlistsResource, c.ns, name), &v1alpha1.PipelineListenerList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerList), err
}

// List takes label and field selectors, and returns the list of PipelineListenerLists that match those selectors.
func (c *FakePipelineListenerLists) List(opts v1.ListOptions) (result *v1alpha1.PipelineListenerListList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(pipelinelistenerlistsResource, pipelinelistenerlistsKind, c.ns, opts), &v1alpha1.PipelineListenerListList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerListList), err
}

// Watch returns a watch.Interface that watches the requested pipelineListenerLists.
func (c *FakePipelineListenerLists) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(pipelinelistenerlistsResource, c.ns, opts))

}

// Create takes the representation of a pipelineListenerList and creates it.  Returns the server's representation of the pipelineListenerList, and an error, if there is any.
func (c *FakePipelineListenerLists) Create(pipelineListenerList *v1alpha1.PipelineListenerList) (result *v1alpha1.PipelineListenerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(pipelinelistenerlistsResource, c.ns, pipelineListenerList), &v1alpha1.PipelineListenerList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerList), err
}

// Update takes the representation of a pipelineListenerList and updates it. Returns the server's representation of the pipelineListenerList, and an error, if there is any.
func (c *FakePipelineListenerLists) Update(pipelineListenerList *v1alpha1.PipelineListenerList) (result *v1alpha1.PipelineListenerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(pipelinelistenerlistsResource, c.ns, pipelineListenerList), &v1alpha1.PipelineListenerList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerList), err
}

// Delete takes name of the pipelineListenerList and deletes it. Returns an error if one occurs.
func (c *FakePipelineListenerLists) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(pipelinelistenerlistsResource, c.ns, name), &v1alpha1.PipelineListenerList{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePipelineListenerLists) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(pipelinelistenerlistsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.PipelineListenerListList{})
	return err
}

// Patch applies the patch and returns the patched pipelineListenerList.
func (c *FakePipelineListenerLists) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineListenerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(pipelinelistenerlistsResource, c.ns, name, data, subresources...), &v1alpha1.PipelineListenerList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerList), err
}
