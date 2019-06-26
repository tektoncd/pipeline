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

// FakePipelineResources implements PipelineResourceInterface
type FakePipelineResources struct {
	Fake *FakeTektonV1alpha1
	ns   string
}

var pipelineresourcesResource = schema.GroupVersionResource{Group: "tekton.dev", Version: "v1alpha1", Resource: "pipelineresources"}

var pipelineresourcesKind = schema.GroupVersionKind{Group: "tekton.dev", Version: "v1alpha1", Kind: "PipelineResource"}

// Get takes name of the pipelineResource, and returns the corresponding pipelineResource object, and an error if there is any.
func (c *FakePipelineResources) Get(name string, options v1.GetOptions) (result *v1alpha1.PipelineResource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(pipelineresourcesResource, c.ns, name), &v1alpha1.PipelineResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineResource), err
}

// List takes label and field selectors, and returns the list of PipelineResources that match those selectors.
func (c *FakePipelineResources) List(opts v1.ListOptions) (result *v1alpha1.PipelineResourceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(pipelineresourcesResource, pipelineresourcesKind, c.ns, opts), &v1alpha1.PipelineResourceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.PipelineResourceList{ListMeta: obj.(*v1alpha1.PipelineResourceList).ListMeta}
	for _, item := range obj.(*v1alpha1.PipelineResourceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested pipelineResources.
func (c *FakePipelineResources) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(pipelineresourcesResource, c.ns, opts))

}

// Create takes the representation of a pipelineResource and creates it.  Returns the server's representation of the pipelineResource, and an error, if there is any.
func (c *FakePipelineResources) Create(pipelineResource *v1alpha1.PipelineResource) (result *v1alpha1.PipelineResource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(pipelineresourcesResource, c.ns, pipelineResource), &v1alpha1.PipelineResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineResource), err
}

// Update takes the representation of a pipelineResource and updates it. Returns the server's representation of the pipelineResource, and an error, if there is any.
func (c *FakePipelineResources) Update(pipelineResource *v1alpha1.PipelineResource) (result *v1alpha1.PipelineResource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(pipelineresourcesResource, c.ns, pipelineResource), &v1alpha1.PipelineResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineResource), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePipelineResources) UpdateStatus(pipelineResource *v1alpha1.PipelineResource) (*v1alpha1.PipelineResource, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(pipelineresourcesResource, "status", c.ns, pipelineResource), &v1alpha1.PipelineResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineResource), err
}

// Delete takes name of the pipelineResource and deletes it. Returns an error if one occurs.
func (c *FakePipelineResources) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(pipelineresourcesResource, c.ns, name), &v1alpha1.PipelineResource{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePipelineResources) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(pipelineresourcesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.PipelineResourceList{})
	return err
}

// Patch applies the patch and returns the patched pipelineResource.
func (c *FakePipelineResources) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineResource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(pipelineresourcesResource, c.ns, name, data, subresources...), &v1alpha1.PipelineResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineResource), err
}
