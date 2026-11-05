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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// ConfigStoreFromContext initialises the config store from the context.
// Optional onAfterStore hooks are forwarded to config.NewStore so callers can
// react to config changes (e.g. tracing provider updates).
func ConfigStoreFromContext(ctx context.Context, cmw configmap.Watcher, onAfterStore ...func(name string, value interface{})) *config.Store {
	logger := logging.FromContext(ctx)
	configStore := config.NewStore(logger.Named("config-store"), onAfterStore...)
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

// EventHandler returns the informer event handler used by the notification
// controllers. Adds and deletes are always enqueued. Updates are enqueued only
// when the Succeeded condition (Status, Reason, Message) differs between the
// previous and the new version of the object.
//
// The notification controllers are read-only and their only side effect is
// sending CloudEvents, which are derived solely from the Succeeded condition.
// Deduplication otherwise relies on an in-memory cache whose entries expire
// (see pkg/reconciler/events/cache). Without this filter, an update that does
// not change the condition — such as the updates informers deliver for every
// object on periodic resync and relist, or a metadata-only change to a label,
// annotation, finalizer or deletionTimestamp — would re-send the events of the
// object once its cache entry has been evicted.
//
// The comparison uses the same condition fields as cache.ObjectKey, so an
// update that is enqueued is one the cache would also have treated as new.
// Objects that do not expose a status condition fall back to comparing their
// resourceVersion.
func EventHandler(enqueue func(interface{})) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) {
			if !conditionChanged(oldObj, newObj) {
				return
			}
			enqueue(newObj)
		},
		DeleteFunc: enqueue,
	}
}

// statusConditionGetter is implemented by all run objects (v1beta1.RunObject).
type statusConditionGetter interface {
	GetStatusCondition() apis.ConditionAccessor
}

// conditionChanged reports whether an informer update could produce a new
// CloudEvent for the object.
func conditionChanged(oldObj, newObj interface{}) bool {
	oldRun, oldOK := oldObj.(statusConditionGetter)
	newRun, newOK := newObj.(statusConditionGetter)
	if oldOK && newOK {
		oldCond, newCond := succeededCondition(oldRun), succeededCondition(newRun)
		return oldCond.Status != newCond.Status ||
			oldCond.Reason != newCond.Reason ||
			oldCond.Message != newCond.Message
	}
	oldMeta, err := meta.Accessor(oldObj)
	if err != nil {
		return true
	}
	newMeta, err := meta.Accessor(newObj)
	if err != nil {
		return true
	}
	rv := newMeta.GetResourceVersion()
	return rv == "" || rv != oldMeta.GetResourceVersion()
}

// succeededCondition returns the Succeeded condition of a run, or an empty
// condition when it is not set, mirroring cache.ObjectKey.
func succeededCondition(run statusConditionGetter) apis.Condition {
	if accessor := run.GetStatusCondition(); accessor != nil {
		if c := accessor.GetCondition(apis.ConditionSucceeded); c != nil {
			return *c
		}
	}
	return apis.Condition{}
}
