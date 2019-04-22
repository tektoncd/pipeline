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

// PipelineListenerSpecLister helps list PipelineListenerSpecs.
type PipelineListenerSpecLister interface {
	// List lists all PipelineListenerSpecs in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.PipelineListenerSpec, err error)
	// PipelineListenerSpecs returns an object that can list and get PipelineListenerSpecs.
	PipelineListenerSpecs(namespace string) PipelineListenerSpecNamespaceLister
	PipelineListenerSpecListerExpansion
}

// pipelineListenerSpecLister implements the PipelineListenerSpecLister interface.
type pipelineListenerSpecLister struct {
	indexer cache.Indexer
}

// NewPipelineListenerSpecLister returns a new PipelineListenerSpecLister.
func NewPipelineListenerSpecLister(indexer cache.Indexer) PipelineListenerSpecLister {
	return &pipelineListenerSpecLister{indexer: indexer}
}

// List lists all PipelineListenerSpecs in the indexer.
func (s *pipelineListenerSpecLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineListenerSpec, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PipelineListenerSpec))
	})
	return ret, err
}

// PipelineListenerSpecs returns an object that can list and get PipelineListenerSpecs.
func (s *pipelineListenerSpecLister) PipelineListenerSpecs(namespace string) PipelineListenerSpecNamespaceLister {
	return pipelineListenerSpecNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PipelineListenerSpecNamespaceLister helps list and get PipelineListenerSpecs.
type PipelineListenerSpecNamespaceLister interface {
	// List lists all PipelineListenerSpecs in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.PipelineListenerSpec, err error)
	// Get retrieves the PipelineListenerSpec from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.PipelineListenerSpec, error)
	PipelineListenerSpecNamespaceListerExpansion
}

// pipelineListenerSpecNamespaceLister implements the PipelineListenerSpecNamespaceLister
// interface.
type pipelineListenerSpecNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PipelineListenerSpecs in the indexer for a given namespace.
func (s pipelineListenerSpecNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineListenerSpec, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PipelineListenerSpec))
	})
	return ret, err
}

// Get retrieves the PipelineListenerSpec from the indexer for a given namespace and name.
func (s pipelineListenerSpecNamespaceLister) Get(name string) (*v1alpha1.PipelineListenerSpec, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("pipelinelistenerspec"), name)
	}
	return obj.(*v1alpha1.PipelineListenerSpec), nil
}
