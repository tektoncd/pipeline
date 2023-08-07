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
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

const ControllerName = "CustomRunEvents"

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
// This is a read-only controller, hence the SkipStatusUpdates set to true
func NewController() func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		configStore := notifications.ConfigStoreFromContext(ctx, cmw)

		c := &Reconciler{}
		notifications.ReconcilerFromContext(ctx, c)

		impl := customrunreconciler.NewImpl(ctx, c, notifications.ControllerOptions(ControllerName, configStore))

		customRunInformer := customruninformer.Get(ctx)
		customRunInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		return impl
	}
}
