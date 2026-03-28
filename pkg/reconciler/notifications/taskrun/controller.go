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

package taskrun

import (
	"context"

	taskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/taskrun"
	taskrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1/taskrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const controllerName = "TaskRunEvents"

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
// This is a read-only controller, hence the SkipStatusUpdates set to true
func NewController() func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		configStore := notifications.ConfigStoreFromContext(ctx, cmw)

		ceClient, cacheClient := notifications.EventClientsFromContext(ctx)
		c := NewReconciler(ceClient, cacheClient)

		impl := taskrunreconciler.NewImpl(ctx, c, notifications.ControllerOptions(controllerName, configStore))

		taskRunInformer := taskruninformer.Get(ctx)
		if _, err := taskRunInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue)); err != nil {
			logging.FromContext(ctx).Panicf("Couldn't register TaskRun informer event handler: %w", err)
		}

		return impl
	}
}
