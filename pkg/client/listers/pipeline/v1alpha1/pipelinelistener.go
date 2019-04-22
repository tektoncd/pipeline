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

// PipelineListenerLister helps list PipelineListeners.
type PipelineListenerLister interface {
	// List lists all PipelineListeners in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.PipelineListener, err error)
	// PipelineListeners returns an object that can list and get PipelineListeners.
	PipelineListeners(namespace string) PipelineListenerNamespaceLister
	PipelineListenerListerExpansion
}

// pipelineListenerLister implements the PipelineListenerLister interface.
type pipelineListenerLister struct {
	indexer cache.Indexer
}

// NewPipelineListenerLister returns a new PipelineListenerLister.
func NewPipelineListenerLister(indexer cache.Indexer) PipelineListenerLister {
	return &pipelineListenerLister{indexer: indexer}
}

// List lists all PipelineListeners in the indexer.
func (s *pipelineListenerLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineListener, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PipelineListener))
	})
	return ret, err
}

// PipelineListeners returns an object that can list and get PipelineListeners.
func (s *pipelineListenerLister) PipelineListeners(namespace string) PipelineListenerNamespaceLister {
	return pipelineListenerNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PipelineListenerNamespaceLister helps list and get PipelineListeners.
type PipelineListenerNamespaceLister interface {
	// List lists all PipelineListeners in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.PipelineListener, err error)
	// Get retrieves the PipelineListener from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.PipelineListener, error)
	PipelineListenerNamespaceListerExpansion
}

// pipelineListenerNamespaceLister implements the PipelineListenerNamespaceLister
// interface.
type pipelineListenerNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PipelineListeners in the indexer for a given namespace.
func (s pipelineListenerNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineListener, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PipelineListener))
	})
	return ret, err
}

// Get retrieves the PipelineListener from the indexer for a given namespace and name.
func (s pipelineListenerNamespaceLister) Get(name string) (*v1alpha1.PipelineListener, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("pipelinelistener"), name)
	}
	return obj.(*v1alpha1.PipelineListener), nil
}
