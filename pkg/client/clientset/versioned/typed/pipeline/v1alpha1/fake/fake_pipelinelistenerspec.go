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

// FakePipelineListenerSpecs implements PipelineListenerSpecInterface
type FakePipelineListenerSpecs struct {
	Fake *FakeTektonV1alpha1
	ns   string
}

var pipelinelistenerspecsResource = schema.GroupVersionResource{Group: "tekton.dev", Version: "v1alpha1", Resource: "pipelinelistenerspecs"}

var pipelinelistenerspecsKind = schema.GroupVersionKind{Group: "tekton.dev", Version: "v1alpha1", Kind: "PipelineListenerSpec"}

// Get takes name of the pipelineListenerSpec, and returns the corresponding pipelineListenerSpec object, and an error if there is any.
func (c *FakePipelineListenerSpecs) Get(name string, options v1.GetOptions) (result *v1alpha1.PipelineListenerSpec, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(pipelinelistenerspecsResource, c.ns, name), &v1alpha1.PipelineListenerSpec{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerSpec), err
}

// List takes label and field selectors, and returns the list of PipelineListenerSpecs that match those selectors.
func (c *FakePipelineListenerSpecs) List(opts v1.ListOptions) (result *v1alpha1.PipelineListenerSpecList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(pipelinelistenerspecsResource, pipelinelistenerspecsKind, c.ns, opts), &v1alpha1.PipelineListenerSpecList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerSpecList), err
}

// Watch returns a watch.Interface that watches the requested pipelineListenerSpecs.
func (c *FakePipelineListenerSpecs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(pipelinelistenerspecsResource, c.ns, opts))

}

// Create takes the representation of a pipelineListenerSpec and creates it.  Returns the server's representation of the pipelineListenerSpec, and an error, if there is any.
func (c *FakePipelineListenerSpecs) Create(pipelineListenerSpec *v1alpha1.PipelineListenerSpec) (result *v1alpha1.PipelineListenerSpec, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(pipelinelistenerspecsResource, c.ns, pipelineListenerSpec), &v1alpha1.PipelineListenerSpec{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerSpec), err
}

// Update takes the representation of a pipelineListenerSpec and updates it. Returns the server's representation of the pipelineListenerSpec, and an error, if there is any.
func (c *FakePipelineListenerSpecs) Update(pipelineListenerSpec *v1alpha1.PipelineListenerSpec) (result *v1alpha1.PipelineListenerSpec, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(pipelinelistenerspecsResource, c.ns, pipelineListenerSpec), &v1alpha1.PipelineListenerSpec{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerSpec), err
}

// Delete takes name of the pipelineListenerSpec and deletes it. Returns an error if one occurs.
func (c *FakePipelineListenerSpecs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(pipelinelistenerspecsResource, c.ns, name), &v1alpha1.PipelineListenerSpec{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePipelineListenerSpecs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(pipelinelistenerspecsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.PipelineListenerSpecList{})
	return err
}

// Patch applies the patch and returns the patched pipelineListenerSpec.
func (c *FakePipelineListenerSpecs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineListenerSpec, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(pipelinelistenerspecsResource, c.ns, name, data, subresources...), &v1alpha1.PipelineListenerSpec{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineListenerSpec), err
}
