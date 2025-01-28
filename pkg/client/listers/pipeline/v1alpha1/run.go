/*
Copyright 2020 The Tekton Authors

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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"
)

// RunLister helps list Runs.
// All objects returned here must be treated as read-only.
type RunLister interface {
	// List lists all Runs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Run, err error)
	// Runs returns an object that can list and get Runs.
	Runs(namespace string) RunNamespaceLister
	RunListerExpansion
}

// runLister implements the RunLister interface.
type runLister struct {
	listers.ResourceIndexer[*v1alpha1.Run]
}

// NewRunLister returns a new RunLister.
func NewRunLister(indexer cache.Indexer) RunLister {
	return &runLister{listers.New[*v1alpha1.Run](indexer, v1alpha1.Resource("run"))}
}

// Runs returns an object that can list and get Runs.
func (s *runLister) Runs(namespace string) RunNamespaceLister {
	return runNamespaceLister{listers.NewNamespaced[*v1alpha1.Run](s.ResourceIndexer, namespace)}
}

// RunNamespaceLister helps list and get Runs.
// All objects returned here must be treated as read-only.
type RunNamespaceLister interface {
	// List lists all Runs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Run, err error)
	// Get retrieves the Run from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.Run, error)
	RunNamespaceListerExpansion
}

// runNamespaceLister implements the RunNamespaceLister
// interface.
type runNamespaceLister struct {
	listers.ResourceIndexer[*v1alpha1.Run]
}
