/*
Copyright 2018 The Kubernetes Authors.

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

package manager

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.KBLog.WithName("manager")

type controllerManager struct {
	// config is the rest.config used to talk to the apiserver.  Required.
	config *rest.Config

	// scheme is the scheme injected into Controllers, EventHandlers, Sources and Predicates.  Defaults
	// to scheme.scheme.
	scheme *runtime.Scheme

	// runnables is the set of Controllers that the controllerManager injects deps into and Starts.
	runnables []Runnable

	cache cache.Cache

	// TODO(directxman12): Provide an escape hatch to get individual indexers
	// client is the client injected into Controllers (and EventHandlers, Sources and Predicates).
	client client.Client

	// fieldIndexes knows how to add field indexes over the Cache used by this controller,
	// which can later be consumed via field selectors from the injected client.
	fieldIndexes client.FieldIndexer

	// recorderProvider is used to generate event recorders that will be injected into Controllers
	// (and EventHandlers, Sources and Predicates).
	recorderProvider recorder.Provider

	mu      sync.Mutex
	started bool
	errChan chan error
	stop    <-chan struct{}

	startCache func(stop <-chan struct{}) error
}

// Add sets dependencies on i, and adds it to the list of runnables to start.
func (cm *controllerManager) Add(r Runnable) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Set dependencies on the object
	if err := cm.SetFields(r); err != nil {
		return err
	}

	// Add the runnable to the list
	cm.runnables = append(cm.runnables, r)
	if cm.started {
		// If already started, start the controller
		go func() {
			cm.errChan <- r.Start(cm.stop)
		}()
	}

	return nil
}

func (cm *controllerManager) SetFields(i interface{}) error {
	if _, err := inject.ConfigInto(cm.config, i); err != nil {
		return err
	}
	if _, err := inject.ClientInto(cm.client, i); err != nil {
		return err
	}
	if _, err := inject.SchemeInto(cm.scheme, i); err != nil {
		return err
	}
	if _, err := inject.CacheInto(cm.cache, i); err != nil {
		return err
	}
	if _, err := inject.InjectorInto(cm.SetFields, i); err != nil {
		return err
	}
	if _, err := inject.StopChannelInto(cm.stop, i); err != nil {
		return err
	}
	return nil
}

func (cm *controllerManager) GetConfig() *rest.Config {
	return cm.config
}

func (cm *controllerManager) GetClient() client.Client {
	return cm.client
}

func (cm *controllerManager) GetScheme() *runtime.Scheme {
	return cm.scheme
}

func (cm *controllerManager) GetFieldIndexer() client.FieldIndexer {
	return cm.fieldIndexes
}

func (cm *controllerManager) GetCache() cache.Cache {
	return cm.cache
}

func (cm *controllerManager) GetRecorder(name string) record.EventRecorder {
	return cm.recorderProvider.GetEventRecorderFor(name)
}

func (cm *controllerManager) Start(stop <-chan struct{}) error {
	func() {
		cm.mu.Lock()
		defer cm.mu.Unlock()

		cm.stop = stop

		// Start the Cache. Allow the function to start the cache to be mocked out for testing
		if cm.startCache == nil {
			cm.startCache = cm.cache.Start
		}
		go func() {
			if err := cm.startCache(stop); err != nil {
				cm.errChan <- err
			}
		}()

		// Wait for the caches to sync.
		// TODO(community): Check the return value and write a test
		cm.cache.WaitForCacheSync(stop)

		// Start the runnables after the cache has synced
		for _, c := range cm.runnables {
			// Controllers block, but we want to return an error if any have an error starting.
			// Write any Start errors to a channel so we can return them
			ctrl := c
			go func() {
				cm.errChan <- ctrl.Start(stop)
			}()
		}

		cm.started = true
	}()

	select {
	case <-stop:
		// We are done
		return nil
	case err := <-cm.errChan:
		// Error starting a controller
		return err
	}
}
