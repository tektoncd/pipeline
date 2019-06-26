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
package v1alpha1

import (
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// TaskRunLister helps list TaskRuns.
type TaskRunLister interface {
	// List lists all TaskRuns in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.TaskRun, err error)
	// TaskRuns returns an object that can list and get TaskRuns.
	TaskRuns(namespace string) TaskRunNamespaceLister
	TaskRunListerExpansion
}

// taskRunLister implements the TaskRunLister interface.
type taskRunLister struct {
	indexer cache.Indexer
}

// NewTaskRunLister returns a new TaskRunLister.
func NewTaskRunLister(indexer cache.Indexer) TaskRunLister {
	return &taskRunLister{indexer: indexer}
}

// List lists all TaskRuns in the indexer.
func (s *taskRunLister) List(selector labels.Selector) (ret []*v1alpha1.TaskRun, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.TaskRun))
	})
	return ret, err
}

// TaskRuns returns an object that can list and get TaskRuns.
func (s *taskRunLister) TaskRuns(namespace string) TaskRunNamespaceLister {
	return taskRunNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// TaskRunNamespaceLister helps list and get TaskRuns.
type TaskRunNamespaceLister interface {
	// List lists all TaskRuns in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.TaskRun, err error)
	// Get retrieves the TaskRun from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.TaskRun, error)
	TaskRunNamespaceListerExpansion
}

// taskRunNamespaceLister implements the TaskRunNamespaceLister
// interface.
type taskRunNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all TaskRuns in the indexer for a given namespace.
func (s taskRunNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.TaskRun, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.TaskRun))
	})
	return ret, err
}

// Get retrieves the TaskRun from the indexer for a given namespace and name.
func (s taskRunNamespaceLister) Get(name string) (*v1alpha1.TaskRun, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("taskrun"), name)
	}
	return obj.(*v1alpha1.TaskRun), nil
}
