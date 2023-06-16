/*
Copyright 2019 The Tekton Authors

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

package pipelinerun

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	pipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/pipelinerun"
	taskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/taskrun"
	verificationpolicyinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/verificationpolicy"
	customruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/customrun"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1/pipelinerun"
	resolutionclient "github.com/tektoncd/pipeline/pkg/client/resolution/injection/client"
	resolutioninformer "github.com/tektoncd/pipeline/pkg/client/resolution/injection/informers/resolution/v1beta1/resolutionrequest"
	"github.com/tektoncd/pipeline/pkg/pipelinerunmetrics"
	cloudeventclient "github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	resolution "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
func NewController(opts *pipeline.Options, clock clock.PassiveClock, tracerProvider trace.TracerProvider) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)
		kubeclientset := kubeclient.Get(ctx)
		pipelineclientset := pipelineclient.Get(ctx)
		taskRunInformer := taskruninformer.Get(ctx)
		customRunInformer := customruninformer.Get(ctx)
		pipelineRunInformer := pipelineruninformer.Get(ctx)
		resolutionInformer := resolutioninformer.Get(ctx)
		verificationpolicyInformer := verificationpolicyinformer.Get(ctx)
		configStore := config.NewStore(logger.Named("config-store"), pipelinerunmetrics.MetricsOnStore(logger))
		configStore.WatchConfigs(cmw)

		c := &Reconciler{
			KubeClientSet:            kubeclientset,
			PipelineClientSet:        pipelineclientset,
			Images:                   opts.Images,
			Clock:                    clock,
			pipelineRunLister:        pipelineRunInformer.Lister(),
			taskRunLister:            taskRunInformer.Lister(),
			customRunLister:          customRunInformer.Lister(),
			verificationPolicyLister: verificationpolicyInformer.Lister(),
			cloudEventClient:         cloudeventclient.Get(ctx),
			metrics:                  pipelinerunmetrics.Get(ctx),
			pvcHandler:               volumeclaim.NewPVCHandler(kubeclientset, logger),
			resolutionRequester:      resolution.NewCRDRequester(resolutionclient.Get(ctx), resolutionInformer.Lister()),
			tracerProvider:           tracerProvider,
		}
		impl := pipelinerunreconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
			return controller.Options{
				AgentName:   pipeline.PipelineRunControllerName,
				ConfigStore: configStore,
			}
		})

		pipelineRunInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		taskRunInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1.PipelineRun{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})
		customRunInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1.PipelineRun{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})
		resolutionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1.PipelineRun{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
