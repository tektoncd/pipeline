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
	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePipelineParamses implements PipelineParamsInterface
type FakePipelineParamses struct {
	Fake *FakePipelineV1alpha1
	ns   string
}

var pipelineparamsesResource = schema.GroupVersionResource{Group: "pipeline.knative.dev", Version: "v1alpha1", Resource: "pipelineparamses"}

var pipelineparamsesKind = schema.GroupVersionKind{Group: "pipeline.knative.dev", Version: "v1alpha1", Kind: "PipelineParams"}

// Get takes name of the pipelineParams, and returns the corresponding pipelineParams object, and an error if there is any.
func (c *FakePipelineParamses) Get(name string, options v1.GetOptions) (result *v1alpha1.PipelineParams, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(pipelineparamsesResource, c.ns, name), &v1alpha1.PipelineParams{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineParams), err
}

// List takes label and field selectors, and returns the list of PipelineParamses that match those selectors.
func (c *FakePipelineParamses) List(opts v1.ListOptions) (result *v1alpha1.PipelineParamsList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(pipelineparamsesResource, pipelineparamsesKind, c.ns, opts), &v1alpha1.PipelineParamsList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.PipelineParamsList{ListMeta: obj.(*v1alpha1.PipelineParamsList).ListMeta}
	for _, item := range obj.(*v1alpha1.PipelineParamsList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested pipelineParamses.
func (c *FakePipelineParamses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(pipelineparamsesResource, c.ns, opts))

}

// Create takes the representation of a pipelineParams and creates it.  Returns the server's representation of the pipelineParams, and an error, if there is any.
func (c *FakePipelineParamses) Create(pipelineParams *v1alpha1.PipelineParams) (result *v1alpha1.PipelineParams, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(pipelineparamsesResource, c.ns, pipelineParams), &v1alpha1.PipelineParams{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineParams), err
}

// Update takes the representation of a pipelineParams and updates it. Returns the server's representation of the pipelineParams, and an error, if there is any.
func (c *FakePipelineParamses) Update(pipelineParams *v1alpha1.PipelineParams) (result *v1alpha1.PipelineParams, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(pipelineparamsesResource, c.ns, pipelineParams), &v1alpha1.PipelineParams{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineParams), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePipelineParamses) UpdateStatus(pipelineParams *v1alpha1.PipelineParams) (*v1alpha1.PipelineParams, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(pipelineparamsesResource, "status", c.ns, pipelineParams), &v1alpha1.PipelineParams{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineParams), err
}

// Delete takes name of the pipelineParams and deletes it. Returns an error if one occurs.
func (c *FakePipelineParamses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(pipelineparamsesResource, c.ns, name), &v1alpha1.PipelineParams{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePipelineParamses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(pipelineparamsesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.PipelineParamsList{})
	return err
}

// Patch applies the patch and returns the patched pipelineParams.
func (c *FakePipelineParamses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PipelineParams, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(pipelineparamsesResource, c.ns, name, data, subresources...), &v1alpha1.PipelineParams{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineParams), err
}
