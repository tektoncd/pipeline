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
	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// StandardResourceLister helps list StandardResources.
type StandardResourceLister interface {
	// List lists all StandardResources in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.StandardResource, err error)
	// StandardResources returns an object that can list and get StandardResources.
	StandardResources(namespace string) StandardResourceNamespaceLister
	StandardResourceListerExpansion
}

// standardResourceLister implements the StandardResourceLister interface.
type standardResourceLister struct {
	indexer cache.Indexer
}

// NewStandardResourceLister returns a new StandardResourceLister.
func NewStandardResourceLister(indexer cache.Indexer) StandardResourceLister {
	return &standardResourceLister{indexer: indexer}
}

// List lists all StandardResources in the indexer.
func (s *standardResourceLister) List(selector labels.Selector) (ret []*v1alpha1.StandardResource, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.StandardResource))
	})
	return ret, err
}

// StandardResources returns an object that can list and get StandardResources.
func (s *standardResourceLister) StandardResources(namespace string) StandardResourceNamespaceLister {
	return standardResourceNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// StandardResourceNamespaceLister helps list and get StandardResources.
type StandardResourceNamespaceLister interface {
	// List lists all StandardResources in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.StandardResource, err error)
	// Get retrieves the StandardResource from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.StandardResource, error)
	StandardResourceNamespaceListerExpansion
}

// standardResourceNamespaceLister implements the StandardResourceNamespaceLister
// interface.
type standardResourceNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all StandardResources in the indexer for a given namespace.
func (s standardResourceNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.StandardResource, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.StandardResource))
	})
	return ret, err
}

// Get retrieves the StandardResource from the indexer for a given namespace and name.
func (s standardResourceNamespaceLister) Get(name string) (*v1alpha1.StandardResource, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("standardresource"), name)
	}
	return obj.(*v1alpha1.StandardResource), nil
}
