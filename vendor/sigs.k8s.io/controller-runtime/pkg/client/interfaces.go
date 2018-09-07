/*
Copyright 2018 The Kubernetes Authors.

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

package client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// ObjectKey identifies a Kubernetes Object.
type ObjectKey = types.NamespacedName

// TODO(directxman12): is there a sane way to deal with get/delete options?

// Reader knows how to read and list Kubernetes objects.
type Reader interface {
	// Get retrieves an obj for the given object key from the Kubernetes Cluster.
	// obj must be a struct pointer so that obj can be updated with the response
	// returned by the Server.
	Get(ctx context.Context, key ObjectKey, obj runtime.Object) error

	// List retrieves list of objects for a given namespace and list options. On a
	// successful call, Items field in the list will be populated with the
	// result returned from the server.
	List(ctx context.Context, opts *ListOptions, list runtime.Object) error
}

// Writer knows how to create, delete, and update Kubernetes objects.
type Writer interface {
	// Create saves the object obj in the Kubernetes cluster.
	Create(ctx context.Context, obj runtime.Object) error

	// Delete deletes the given obj from Kubernetes cluster.
	Delete(ctx context.Context, obj runtime.Object) error

	// Update updates the given obj in the Kubernetes cluster. obj must be a
	// struct pointer so that obj can be updated with the content returned by the Server.
	Update(ctx context.Context, obj runtime.Object) error
}

// StatusClient knows how to create a client which can update status subresource
// for kubernetes objects.
type StatusClient interface {
	Status() StatusWriter
}

// StatusWriter knows how to update status subresource of a Kubernetes object.
type StatusWriter interface {
	// Update updates the fields corresponding to the status subresource for the
	// given obj. obj must be a struct pointer so that obj can be updated
	// with the content returned by the Server.
	Update(ctx context.Context, obj runtime.Object) error
}

// Client knows how to perform CRUD operations on Kubernetes objects.
type Client interface {
	Reader
	Writer
	StatusClient
}

// IndexerFunc knows how to take an object and turn it into a series
// of (non-namespaced) keys for that object.
type IndexerFunc func(runtime.Object) []string

// FieldIndexer knows how to index over a particular "field" such that it
// can later be used by a field selector.
type FieldIndexer interface {
	// IndexFields adds an index with the given field name on the given object type
	// by using the given function to extract the value for that field.  If you want
	// compatibility with the Kubernetes API server, only return one key, and only use
	// fields that the API server supports.  Otherwise, you can return multiple keys,
	// and "equality" in the field selector means that at least one key matches the value.
	IndexField(obj runtime.Object, field string, extractValue IndexerFunc) error
}

// ListOptions contains options for limitting or filtering results.
// It's generally a subset of metav1.ListOptions, with support for
// pre-parsed selectors (since generally, selectors will be executed
// against the cache).
type ListOptions struct {
	// LabelSelector filters results by label.  Use SetLabelSelector to
	// set from raw string form.
	LabelSelector labels.Selector
	// FieldSelector filters results by a particular field.  In order
	// to use this with cache-based implementations, restrict usage to
	// a single field-value pair that's been added to the indexers.
	FieldSelector fields.Selector

	// Namespace represents the namespace to list for, or empty for
	// non-namespaced objects, or to list across all namespaces.
	Namespace string

	// Raw represents raw ListOptions, as passed to the API server.  Note
	// that these may not be respected by all implementations of interface,
	// and the LabelSelector and FieldSelector fields are ignored.
	Raw *metav1.ListOptions
}

// SetLabelSelector sets this the label selector of these options
// from a string form of the selector.
func (o *ListOptions) SetLabelSelector(selRaw string) error {
	sel, err := labels.Parse(selRaw)
	if err != nil {
		return err
	}
	o.LabelSelector = sel
	return nil
}

// SetFieldSelector sets this the label selector of these options
// from a string form of the selector.
func (o *ListOptions) SetFieldSelector(selRaw string) error {
	sel, err := fields.ParseSelector(selRaw)
	if err != nil {
		return err
	}
	o.FieldSelector = sel
	return nil
}

// AsListOptions returns these options as a flattened metav1.ListOptions.
// This may mutate the Raw field.
func (o *ListOptions) AsListOptions() *metav1.ListOptions {
	if o == nil {
		return &metav1.ListOptions{}
	}
	if o.Raw == nil {
		o.Raw = &metav1.ListOptions{}
	}
	if o.LabelSelector != nil {
		o.Raw.LabelSelector = o.LabelSelector.String()
	}
	if o.FieldSelector != nil {
		o.Raw.FieldSelector = o.FieldSelector.String()
	}
	return o.Raw
}

// MatchingLabels is a convenience function that sets the label selector
// to match the given labels, and then returns the options.
// It mutates the list options.
func (o *ListOptions) MatchingLabels(lbls map[string]string) *ListOptions {
	sel := labels.SelectorFromSet(lbls)
	o.LabelSelector = sel
	return o
}

// MatchingField is a convenience function that sets the field selector
// to match the given field, and then returns the options.
// It mutates the list options.
func (o *ListOptions) MatchingField(name, val string) *ListOptions {
	sel := fields.SelectorFromSet(fields.Set{name: val})
	o.FieldSelector = sel
	return o
}

// InNamespace is a convenience function that sets the namespace,
// and then returns the options. It mutates the list options.
func (o *ListOptions) InNamespace(ns string) *ListOptions {
	o.Namespace = ns
	return o
}

// MatchingLabels is a convenience function that constructs list options
// to match the given labels.
func MatchingLabels(lbls map[string]string) *ListOptions {
	return (&ListOptions{}).MatchingLabels(lbls)
}

// MatchingField is a convenience function that constructs list options
// to match the given field.
func MatchingField(name, val string) *ListOptions {
	return (&ListOptions{}).MatchingField(name, val)
}

// InNamespace is a convenience function that constructs list
// options to list in the given namespace.
func InNamespace(ns string) *ListOptions {
	return (&ListOptions{}).InNamespace(ns)
}
