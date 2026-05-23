/*
Copyright 2022 The Tekton Authors

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

package customrun

import (
	"context"

	customruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/customrun"
	customrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/customrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"github.com/tektoncd/pipeline/pkg/tracing"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/secret"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const controllerName = "CustomRunEvents"

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
// This is a read-only controller, hence the SkipStatusUpdates set to true
func NewController() func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)
		secretInformer := secret.Get(ctx)
		tracerProvider := tracing.New(TracerProviderName, logger.Named("tracing"))

		//nolint:contextcheck // OnStore methods does not support context as a parameter
		configStore := notifications.ConfigStoreFromContext(ctx, cmw, tracerProvider.OnStore(secretInformer.Lister()))

		ceClient, cacheClient := notifications.EventClientsFromContext(ctx)
		c := NewReconciler(ceClient, cacheClient, tracerProvider)

		impl := customrunreconciler.NewImpl(ctx, c, notifications.ControllerOptions(controllerName, configStore))

		customRunInformer := customruninformer.Get(ctx)
		if _, err := customRunInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue)); err != nil {
			logging.FromContext(ctx).Panicf("Couldn't register CustomRun informer event handler: %w", err)
		}

		if _, err := secretInformer.Informer().AddEventHandler(controller.HandleAll(tracerProvider.Handler)); err != nil {
			logging.FromContext(ctx).Panicf("Couldn't register Secret informer event handler: %w", err)
		}

		return impl
	}
}
