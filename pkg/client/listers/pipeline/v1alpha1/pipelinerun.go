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

// PipelineRunLister helps list PipelineRuns.
type PipelineRunLister interface {
	// List lists all PipelineRuns in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.PipelineRun, err error)
	// PipelineRuns returns an object that can list and get PipelineRuns.
	PipelineRuns(namespace string) PipelineRunNamespaceLister
	PipelineRunListerExpansion
}

// pipelineRunLister implements the PipelineRunLister interface.
type pipelineRunLister struct {
	indexer cache.Indexer
}

// NewPipelineRunLister returns a new PipelineRunLister.
func NewPipelineRunLister(indexer cache.Indexer) PipelineRunLister {
	return &pipelineRunLister{indexer: indexer}
}

// List lists all PipelineRuns in the indexer.
func (s *pipelineRunLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineRun, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PipelineRun))
	})
	return ret, err
}

// PipelineRuns returns an object that can list and get PipelineRuns.
func (s *pipelineRunLister) PipelineRuns(namespace string) PipelineRunNamespaceLister {
	return pipelineRunNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PipelineRunNamespaceLister helps list and get PipelineRuns.
type PipelineRunNamespaceLister interface {
	// List lists all PipelineRuns in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.PipelineRun, err error)
	// Get retrieves the PipelineRun from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.PipelineRun, error)
	PipelineRunNamespaceListerExpansion
}

// pipelineRunNamespaceLister implements the PipelineRunNamespaceLister
// interface.
type pipelineRunNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PipelineRuns in the indexer for a given namespace.
func (s pipelineRunNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineRun, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PipelineRun))
	})
	return ret, err
}

// Get retrieves the PipelineRun from the indexer for a given namespace and name.
func (s pipelineRunNamespaceLister) Get(name string) (*v1alpha1.PipelineRun, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("pipelinerun"), name)
	}
	return obj.(*v1alpha1.PipelineRun), nil
}
