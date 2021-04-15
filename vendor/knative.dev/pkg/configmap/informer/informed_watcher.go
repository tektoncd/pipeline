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

package informer

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
)

// NewInformedWatcherFromFactory watches a Kubernetes namespace for ConfigMap changes.
func NewInformedWatcherFromFactory(sif informers.SharedInformerFactory, namespace string) *InformedWatcher {
	return &InformedWatcher{
		sif:      sif,
		informer: sif.Core().V1().ConfigMaps(),
		ManualWatcher: configmap.ManualWatcher{
			Namespace: namespace,
		},
		defaults: make(map[string]*corev1.ConfigMap),
	}
}

// NewInformedWatcher watches a Kubernetes namespace for ConfigMap changes.
// Optional label requirements allow restricting the list of ConfigMap objects
// that is tracked by the underlying Informer.
func NewInformedWatcher(kc kubernetes.Interface, namespace string, lr ...labels.Requirement) *InformedWatcher {
	return NewInformedWatcherFromFactory(informers.NewSharedInformerFactoryWithOptions(
		kc,
		// We noticed that we're getting updates all the time anyway, due to the
		// watches being terminated and re-spawned.
		0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(addLabelRequirementsToListOptions(lr)),
	), namespace)
}

// addLabelRequirementsToListOptions returns a function which injects label
// requirements to existing metav1.ListOptions.
func addLabelRequirementsToListOptions(lr []labels.Requirement) internalinterfaces.TweakListOptionsFunc {
	if len(lr) == 0 {
		return nil
	}

	return func(lo *metav1.ListOptions) {
		sel, err := labels.Parse(lo.LabelSelector)
		if err != nil {
			panic(fmt.Errorf("could not parse label selector %q: %w", lo.LabelSelector, err))
		}
		lo.LabelSelector = sel.Add(lr...).String()
	}
}

// FilterConfigByLabelExists returns an "exists" label requirement for the
// given label key.
func FilterConfigByLabelExists(labelKey string) (*labels.Requirement, error) {
	req, err := labels.NewRequirement(labelKey, selection.Exists, nil)
	if err != nil {
		return nil, fmt.Errorf("could not construct label requirement: %w", err)
	}
	return req, nil
}

// InformedWatcher provides an informer-based implementation of Watcher.
type InformedWatcher struct {
	sif      informers.SharedInformerFactory
	informer corev1informers.ConfigMapInformer
	started  bool

	// defaults are the default ConfigMaps to use if the real ones do not exist or are deleted.
	defaults map[string]*corev1.ConfigMap

	// Embedding this struct allows us to reuse the logic
	// of registering and notifying observers. This simplifies the
	// InformedWatcher to just setting up the Kubernetes informer.
	configmap.ManualWatcher
}

// Asserts that InformedWatcher implements Watcher.
var _ configmap.Watcher = (*InformedWatcher)(nil)

// Asserts that InformedWatcher implements DefaultingWatcher.
var _ configmap.DefaultingWatcher = (*InformedWatcher)(nil)

// WatchWithDefault implements DefaultingWatcher. Adding a default for the configMap being watched means that when
// Start is called, Start will not wait for the add event from the API server.
func (i *InformedWatcher) WatchWithDefault(cm corev1.ConfigMap, o ...configmap.Observer) {
	i.defaults[cm.Name] = &cm

	i.Lock()
	started := i.started
	i.Unlock()
	if started {
		// TODO make both Watch and WatchWithDefault work after the InformedWatcher has started.
		// This likely entails changing this to `o(&cm)` and having Watch check started, if it has
		// started, then ensuring i.informer.Lister().ConfigMaps(i.Namespace).Get(cmName) exists and
		// calling this observer on it. It may require changing Watch and WatchWithDefault to return
		// an error.
		panic("cannot WatchWithDefault after the InformedWatcher has started")
	}

	i.Watch(cm.Name, o...)
}

func (i *InformedWatcher) triggerAddEventForDefaultedConfigMaps(addConfigMapEvent func(obj interface{})) {
	i.ForEach(func(k string, _ []configmap.Observer) error {
		if def, ok := i.defaults[k]; ok {
			addConfigMapEvent(def)
		}
		return nil
	})
}

func (i *InformedWatcher) getConfigMapNames() []string {
	var configMaps []string
	i.ForEach(func(k string, _ []configmap.Observer) error {
		configMaps = append(configMaps, k)
		return nil
	})
	return configMaps
}

// Start implements Watcher. Start will wait for all watched resources to exist and for the add event handler to be
// invoked at least once for each before continuing or for the stopCh to be signalled, whichever happens first. If
// the watched resource is defaulted, Start will invoke the add event handler directly and will not wait for a further
// add event from the API server.
func (i *InformedWatcher) Start(stopCh <-chan struct{}) error {
	// using the synced callback wrapper around the add event handler will allow the caller
	// to wait for the add event to be processed for all configmaps
	s := newSyncedCallback(i.getConfigMapNames(), i.addConfigMapEvent)
	addConfigMapEvent := func(obj interface{}) {
		configMap := obj.(*corev1.ConfigMap)
		s.Call(obj, configMap.Name)
	}
	// Pretend that all the defaulted ConfigMaps were just created. This is done before we start
	// the informer to ensure that if a defaulted ConfigMap does exist, then the real value is
	// processed after the default one.
	i.triggerAddEventForDefaultedConfigMaps(addConfigMapEvent)

	if err := i.registerCallbackAndStartInformer(addConfigMapEvent, stopCh); err != nil {
		return err
	}

	// Wait until the shared informer has been synced (WITHOUT holing the mutex, so callbacks happen)
	if ok := cache.WaitForCacheSync(stopCh, i.informer.Informer().HasSynced); !ok {
		return errors.New("error waiting for ConfigMap informer to sync")
	}

	if err := i.checkObservedResourcesExist(); err != nil {
		return err
	}

	// Wait until all config maps have been at least initially processed
	return s.WaitForAllKeys(stopCh)
}

func (i *InformedWatcher) registerCallbackAndStartInformer(addConfigMapEvent func(obj interface{}), stopCh <-chan struct{}) error {
	i.Lock()
	defer i.Unlock()
	if i.started {
		return errors.New("watcher already started")
	}
	i.started = true

	i.informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    addConfigMapEvent,
		UpdateFunc: i.updateConfigMapEvent,
		DeleteFunc: i.deleteConfigMapEvent,
	})

	// Start the shared informer factory (non-blocking).
	i.sif.Start(stopCh)

	return nil
}

func (i *InformedWatcher) checkObservedResourcesExist() error {
	i.RLock()
	defer i.RUnlock()
	// Check that all objects with Observers exist in our informers.
	return i.ForEach(func(k string, _ []configmap.Observer) error {
		if _, err := i.informer.Lister().ConfigMaps(i.Namespace).Get(k); err != nil {
			if _, ok := i.defaults[k]; ok && k8serrors.IsNotFound(err) {
				// It is defaulted, so it is OK that it doesn't exist.
				return nil
			}
			return err
		}
		return nil
	})
}

func (i *InformedWatcher) addConfigMapEvent(obj interface{}) {
	configMap := obj.(*corev1.ConfigMap)
	i.OnChange(configMap)
}

func (i *InformedWatcher) updateConfigMapEvent(o, n interface{}) {
	// Ignore updates that are idempotent. We are seeing those
	// periodically.
	if equality.Semantic.DeepEqual(o, n) {
		return
	}
	configMap := n.(*corev1.ConfigMap)
	i.OnChange(configMap)
}

func (i *InformedWatcher) deleteConfigMapEvent(obj interface{}) {
	configMap := obj.(*corev1.ConfigMap)
	if def, ok := i.defaults[configMap.Name]; ok {
		i.OnChange(def)
	}
	// If there is no default value, then don't do anything.
}
