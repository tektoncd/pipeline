/*
Copyright 2019 The Knative Authors

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

package injection

import (
	"context"
	"sync"

	"k8s.io/client-go/rest"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

// Interface is the interface for interacting with injection
// implementations, such as our Default and Fake below.
type Interface interface {
	// RegisterClient registers a new injector callback for associating
	// a new client with a context.
	RegisterClient(ClientInjector)

	// GetClients fetches all of the registered client injectors.
	GetClients() []ClientInjector

	// RegisterClientFetcher registers a new callback that fetches a client from
	// a given context.
	RegisterClientFetcher(ClientFetcher)

	// FetchAllClients returns all known clients from the given context.
	FetchAllClients(context.Context) []interface{}

	// RegisterInformerFactory registers a new injector callback for associating
	// a new informer factory with a context.
	RegisterInformerFactory(InformerFactoryInjector)

	// GetInformerFactories fetches all of the registered informer factory injectors.
	GetInformerFactories() []InformerFactoryInjector

	// RegisterDuck registers a new duck.InformerFactory for a particular type.
	RegisterDuck(ii DuckFactoryInjector)

	// GetDucks accesses the set of registered ducks.
	GetDucks() []DuckFactoryInjector

	// RegisterInformer registers a new injector callback for associating
	// a new informer with a context.
	RegisterInformer(InformerInjector)

	// RegisterFilteredInformers registers a new filtered informer injector callback for associating
	// a new set of informers with a context.
	RegisterFilteredInformers(FilteredInformersInjector)

	// GetInformers fetches all of the registered informer injectors.
	GetInformers() []InformerInjector

	// GetFilteredInformers fetches all of the registered filtered informer injectors.
	GetFilteredInformers() []FilteredInformersInjector

	// SetupInformers runs all of the injectors against a context, starting with
	// the clients and the given rest.Config.  The resulting context is returned
	// along with a list of the .Informer() for each of the injected informers,
	// which is suitable for passing to controller.StartInformers().
	// This does not setup or start any controllers.
	SetupInformers(context.Context, *rest.Config) (context.Context, []controller.Informer)
}

type ControllerConstructor func(context.Context, configmap.Watcher) *controller.Impl

// NamedControllerConstructor is a ControllerConstructor with an associated name.
type NamedControllerConstructor struct {
	// Name is the name associated with the controller returned by ControllerConstructor.
	Name string
	// ControllerConstructor is a constructor for a controller.
	ControllerConstructor ControllerConstructor
}

var (
	// Check that impl implements Interface
	_ Interface = (*impl)(nil)

	// Default is the injection interface with which informers should register
	// to make themselves available to the controller process when reconcilers
	// are being run for real.
	Default Interface = &impl{}

	// Fake is the injection interface with which informers should register
	// to make themselves available to the controller process when it is being
	// unit tested.
	Fake Interface = &impl{}
)

type impl struct {
	m sync.RWMutex

	clients           []ClientInjector
	clientFetchers    []ClientFetcher
	factories         []InformerFactoryInjector
	informers         []InformerInjector
	filteredInformers []FilteredInformersInjector
	ducks             []DuckFactoryInjector
}
