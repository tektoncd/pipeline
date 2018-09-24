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

// PipelineParamsLister helps list PipelineParamses.
type PipelineParamsLister interface {
	// List lists all PipelineParamses in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.PipelineParams, err error)
	// PipelineParamses returns an object that can list and get PipelineParamses.
	PipelineParamses(namespace string) PipelineParamsNamespaceLister
	PipelineParamsListerExpansion
}

// pipelineParamsLister implements the PipelineParamsLister interface.
type pipelineParamsLister struct {
	indexer cache.Indexer
}

// NewPipelineParamsLister returns a new PipelineParamsLister.
func NewPipelineParamsLister(indexer cache.Indexer) PipelineParamsLister {
	return &pipelineParamsLister{indexer: indexer}
}

// List lists all PipelineParamses in the indexer.
func (s *pipelineParamsLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineParams, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PipelineParams))
	})
	return ret, err
}

// PipelineParamses returns an object that can list and get PipelineParamses.
func (s *pipelineParamsLister) PipelineParamses(namespace string) PipelineParamsNamespaceLister {
	return pipelineParamsNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PipelineParamsNamespaceLister helps list and get PipelineParamses.
type PipelineParamsNamespaceLister interface {
	// List lists all PipelineParamses in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.PipelineParams, err error)
	// Get retrieves the PipelineParams from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.PipelineParams, error)
	PipelineParamsNamespaceListerExpansion
}

// pipelineParamsNamespaceLister implements the PipelineParamsNamespaceLister
// interface.
type pipelineParamsNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PipelineParamses in the indexer for a given namespace.
func (s pipelineParamsNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineParams, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PipelineParams))
	})
	return ret, err
}

// Get retrieves the PipelineParams from the indexer for a given namespace and name.
func (s pipelineParamsNamespaceLister) Get(name string) (*v1alpha1.PipelineParams, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("pipelineparams"), name)
	}
	return obj.(*v1alpha1.PipelineParams), nil
}
