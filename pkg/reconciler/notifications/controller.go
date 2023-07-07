/*
Copyright 2026 The Tekton Authors

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

package notifications

import (
	"context"

	bc "github.com/allegro/bigcache/v3"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	cacheclient "github.com/tektoncd/pipeline/pkg/reconciler/events/cache"
	cloudeventclient "github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// ConfigStoreFromContext initialise the config store from the context
func ConfigStoreFromContext(ctx context.Context, cmw configmap.Watcher) *config.Store {
	logger := logging.FromContext(ctx)
	configStore := config.NewStore(logger.Named("config-store"))
	configStore.WatchConfigs(cmw)
	return configStore
}

// EventClientsFromContext extracts the cloud event clients from the context.
func EventClientsFromContext(ctx context.Context) (cloudeventclient.CEClient, *bc.BigCache) {
	return cloudeventclient.Get(ctx), cacheclient.Get(ctx)
}

// ControllerOptions returns a function that returns options for a controller implementation
//
// Options:
//   - SkipStatusUpdates: true - ensures that this is a readonly controller
//
// Note: no Concurrency limit is set. The knative work queue guarantees per-key
// serialization: the same object cannot be reconciled concurrently by two workers
// (client-go's processing set ensures this). The cache is therefore safe to use
// with the default concurrency.
func ControllerOptions(name string, store *config.Store) func(impl *controller.Impl) controller.Options {
	return func(impl *controller.Impl) controller.Options {
		return controller.Options{
			AgentName:         name,
			ConfigStore:       store,
			SkipStatusUpdates: true,
		}
	}
}
