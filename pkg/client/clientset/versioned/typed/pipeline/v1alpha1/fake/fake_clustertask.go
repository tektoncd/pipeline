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

// FakeClusterTasks implements ClusterTaskInterface
type FakeClusterTasks struct {
	Fake *FakeTektonV1alpha1
}

var clustertasksResource = schema.GroupVersionResource{Group: "tekton.dev", Version: "v1alpha1", Resource: "clustertasks"}

var clustertasksKind = schema.GroupVersionKind{Group: "tekton.dev", Version: "v1alpha1", Kind: "ClusterTask"}

// Get takes name of the clusterTask, and returns the corresponding clusterTask object, and an error if there is any.
func (c *FakeClusterTasks) Get(name string, options v1.GetOptions) (result *v1alpha1.ClusterTask, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clustertasksResource, name), &v1alpha1.ClusterTask{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterTask), err
}

// List takes label and field selectors, and returns the list of ClusterTasks that match those selectors.
func (c *FakeClusterTasks) List(opts v1.ListOptions) (result *v1alpha1.ClusterTaskList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clustertasksResource, clustertasksKind, opts), &v1alpha1.ClusterTaskList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ClusterTaskList{ListMeta: obj.(*v1alpha1.ClusterTaskList).ListMeta}
	for _, item := range obj.(*v1alpha1.ClusterTaskList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterTasks.
func (c *FakeClusterTasks) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clustertasksResource, opts))
}

// Create takes the representation of a clusterTask and creates it.  Returns the server's representation of the clusterTask, and an error, if there is any.
func (c *FakeClusterTasks) Create(clusterTask *v1alpha1.ClusterTask) (result *v1alpha1.ClusterTask, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clustertasksResource, clusterTask), &v1alpha1.ClusterTask{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterTask), err
}

// Update takes the representation of a clusterTask and updates it. Returns the server's representation of the clusterTask, and an error, if there is any.
func (c *FakeClusterTasks) Update(clusterTask *v1alpha1.ClusterTask) (result *v1alpha1.ClusterTask, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clustertasksResource, clusterTask), &v1alpha1.ClusterTask{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterTask), err
}

// Delete takes name of the clusterTask and deletes it. Returns an error if one occurs.
func (c *FakeClusterTasks) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clustertasksResource, name), &v1alpha1.ClusterTask{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterTasks) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clustertasksResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.ClusterTaskList{})
	return err
}

// Patch applies the patch and returns the patched clusterTask.
func (c *FakeClusterTasks) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ClusterTask, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clustertasksResource, name, data, subresources...), &v1alpha1.ClusterTask{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterTask), err
}
