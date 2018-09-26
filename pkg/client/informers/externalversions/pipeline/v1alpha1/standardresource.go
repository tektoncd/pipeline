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
	time "time"

	pipeline_v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	versioned "github.com/knative/build-pipeline/pkg/client/clientset/versioned"
	internalinterfaces "github.com/knative/build-pipeline/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// StandardResourceInformer provides access to a shared informer and lister for
// StandardResources.
type StandardResourceInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.StandardResourceLister
}

type standardResourceInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewStandardResourceInformer constructs a new informer for StandardResource type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewStandardResourceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredStandardResourceInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredStandardResourceInformer constructs a new informer for StandardResource type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredStandardResourceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PipelineV1alpha1().StandardResources(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PipelineV1alpha1().StandardResources(namespace).Watch(options)
			},
		},
		&pipeline_v1alpha1.StandardResource{},
		resyncPeriod,
		indexers,
	)
}

func (f *standardResourceInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredStandardResourceInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *standardResourceInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&pipeline_v1alpha1.StandardResource{}, f.defaultInformer)
}

func (f *standardResourceInformer) Lister() v1alpha1.StandardResourceLister {
	return v1alpha1.NewStandardResourceLister(f.Informer().GetIndexer())
}
