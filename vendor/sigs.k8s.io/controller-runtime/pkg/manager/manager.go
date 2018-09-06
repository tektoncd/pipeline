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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	internalrecorder "sigs.k8s.io/controller-runtime/pkg/internal/recorder"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
)

// Manager initializes shared dependencies such as Caches and Clients, and provides them to Runnables.
// A Manager is required to create Controllers.
type Manager interface {
	// Add will set reqeusted dependencies on the component, and cause the component to be
	// started when Start is called.  Add will inject any dependencies for which the argument
	// implements the inject interface - e.g. inject.Client
	Add(Runnable) error

	// SetFields will set any dependencies on an object for which the object has implemented the inject
	// interface - e.g. inject.Client.
	SetFields(interface{}) error

	// Start starts all registered Controllers and blocks until the Stop channel is closed.
	// Returns an error if there is an error starting any controller.
	Start(<-chan struct{}) error

	// GetConfig returns an initialized Config
	GetConfig() *rest.Config

	// GetScheme returns and initialized Scheme
	GetScheme() *runtime.Scheme

	// GetClient returns a client configured with the Config
	GetClient() client.Client

	// GetFieldIndexer returns a client.FieldIndexer configured with the client
	GetFieldIndexer() client.FieldIndexer

	// GetCache returns a cache.Cache
	GetCache() cache.Cache

	// GetRecorder returns a new EventRecorder for the provided name
	GetRecorder(name string) record.EventRecorder
}

// Options are the arguments for creating a new Manager
type Options struct {
	// Scheme is the scheme used to resolve runtime.Objects to GroupVersionKinds / Resources
	// Defaults to the kubernetes/client-go scheme.Scheme
	Scheme *runtime.Scheme

	// MapperProvider provides the rest mapper used to map go types to Kubernetes APIs
	MapperProvider func(c *rest.Config) (meta.RESTMapper, error)

	// SyncPeriod determines the minimum frequency at which watched resources are
	// reconciled. A lower period will correct entropy more quickly, but reduce
	// responsiveness to change if there are many watched resources. Change this
	// value only if you know what you are doing. Defaults to 10 hours if unset.
	SyncPeriod *time.Duration

	// Dependency injection for testing
	newCache            func(config *rest.Config, opts cache.Options) (cache.Cache, error)
	newClient           func(config *rest.Config, options client.Options) (client.Client, error)
	newRecorderProvider func(config *rest.Config, scheme *runtime.Scheme) (recorder.Provider, error)
}

// Runnable allows a component to be started.
type Runnable interface {
	// Start starts running the component.  The component will stop running when the channel is closed.
	// Start blocks until the channel is closed or an error occurs.
	Start(<-chan struct{}) error
}

// RunnableFunc implements Runnable
type RunnableFunc func(<-chan struct{}) error

// Start implements Runnable
func (r RunnableFunc) Start(s <-chan struct{}) error {
	return r(s)
}

// New returns a new Manager for creating Controllers.
func New(config *rest.Config, options Options) (Manager, error) {
	// Initialize a rest.config if none was specified
	if config == nil {
		return nil, fmt.Errorf("must specify Config")
	}

	// Set default values for options fields
	options = setOptionsDefaults(options)

	// Create the mapper provider
	mapper, err := options.MapperProvider(config)
	if err != nil {
		log.Error(err, "Failed to get API Group-Resources")
		return nil, err
	}

	// Create the Client for Write operations.
	writeObj, err := options.newClient(config, client.Options{Scheme: options.Scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}

	// Create the cache for the cached read client and registering informers
	cache, err := options.newCache(config, cache.Options{Scheme: options.Scheme, Mapper: mapper, Resync: options.SyncPeriod})
	if err != nil {
		return nil, err
	}
	// Create the recorder provider to inject event recorders for the components.
	recorderProvider, err := options.newRecorderProvider(config, options.Scheme)
	if err != nil {
		return nil, err
	}

	return &controllerManager{
		config:           config,
		scheme:           options.Scheme,
		errChan:          make(chan error),
		cache:            cache,
		fieldIndexes:     cache,
		client:           client.DelegatingClient{Reader: cache, Writer: writeObj, StatusClient: writeObj},
		recorderProvider: recorderProvider,
	}, nil
}

// setOptionsDefaults set default values for Options fields
func setOptionsDefaults(options Options) Options {
	// Use the Kubernetes client-go scheme if none is specified
	if options.Scheme == nil {
		options.Scheme = scheme.Scheme
	}

	if options.MapperProvider == nil {
		options.MapperProvider = apiutil.NewDiscoveryRESTMapper
	}

	// Allow newClient to be mocked
	if options.newClient == nil {
		options.newClient = client.New
	}

	// Allow newCache to be mocked
	if options.newCache == nil {
		options.newCache = cache.New
	}

	// Allow newRecorderProvider to be mocked
	if options.newRecorderProvider == nil {
		options.newRecorderProvider = internalrecorder.NewProvider
	}

	return options
}
