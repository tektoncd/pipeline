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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// TektonListenerLister helps list TektonListeners.
type TektonListenerLister interface {
	// List lists all TektonListeners in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.TektonListener, err error)
	// TektonListeners returns an object that can list and get TektonListeners.
	TektonListeners(namespace string) TektonListenerNamespaceLister
	TektonListenerListerExpansion
}

// tektonListenerLister implements the TektonListenerLister interface.
type tektonListenerLister struct {
	indexer cache.Indexer
}

// NewTektonListenerLister returns a new TektonListenerLister.
func NewTektonListenerLister(indexer cache.Indexer) TektonListenerLister {
	return &tektonListenerLister{indexer: indexer}
}

// List lists all TektonListeners in the indexer.
func (s *tektonListenerLister) List(selector labels.Selector) (ret []*v1alpha1.TektonListener, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.TektonListener))
	})
	return ret, err
}

// TektonListeners returns an object that can list and get TektonListeners.
func (s *tektonListenerLister) TektonListeners(namespace string) TektonListenerNamespaceLister {
	return tektonListenerNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// TektonListenerNamespaceLister helps list and get TektonListeners.
type TektonListenerNamespaceLister interface {
	// List lists all TektonListeners in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.TektonListener, err error)
	// Get retrieves the TektonListener from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.TektonListener, error)
	TektonListenerNamespaceListerExpansion
}

// tektonListenerNamespaceLister implements the TektonListenerNamespaceLister
// interface.
type tektonListenerNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all TektonListeners in the indexer for a given namespace.
func (s tektonListenerNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.TektonListener, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.TektonListener))
	})
	return ret, err
}

// Get retrieves the TektonListener from the indexer for a given namespace and name.
func (s tektonListenerNamespaceLister) Get(name string) (*v1alpha1.TektonListener, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("tektonlistener"), name)
	}
	return obj.(*v1alpha1.TektonListener), nil
}
