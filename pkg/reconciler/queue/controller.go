/*
 *
 * Copyright 2019 The Tekton Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package queue

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	pipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/pipelinerun"
	queueclient "github.com/tektoncd/pipeline/pkg/client/queue/injection/client"
	queueinformer "github.com/tektoncd/pipeline/pkg/client/queue/injection/informers/queue/v1alpha1/queue"
	queuereconciler "github.com/tektoncd/pipeline/pkg/client/queue/injection/reconciler/queue/v1alpha1/queue"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/manager"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
// This is a read-only controller, hence the SkipStatusUpdates set to true
func NewController() func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)
		queueInformer := queueinformer.Get(ctx)
		pipelineClientSet := pipelineclient.Get(ctx)
		pipelineRunInformer := pipelineruninformer.Get(ctx)
		queueClientSet := queueclient.Get(ctx)

		configStore := config.NewStore(logger.Named("config-store"))
		configStore.WatchConfigs(cmw)

		r := &Reconciler{
			QueueLister:       queueInformer.Lister(),
			PipelineRunLister: pipelineRunInformer.Lister(),
			PipelineClientSet: pipelineClientSet,
		}
		impl := queuereconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
			return controller.Options{
				AgentName:         pipeline.QueueControllerName,
				ConfigStore:       configStore,
				SkipStatusUpdates: true,
			}
		})

		// initializing queue manager
		manager.Init(ctx,
			manager.WithPipelineRunLister(pipelineRunInformer.Lister()),
			manager.WithQueueLister(queueInformer.Lister()),
			manager.WithQueueClientSet(queueClientSet),
			manager.WithPipelineClientSet(pipelineClientSet))

		queueInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		return impl
	}
}
