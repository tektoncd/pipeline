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

package taskrun

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	taskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/taskrun"
	verificationpolicyinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/verificationpolicy"
	taskrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1/taskrun"
	resolutionclient "github.com/tektoncd/pipeline/pkg/client/resolution/injection/client"
	resolutioninformer "github.com/tektoncd/pipeline/pkg/client/resolution/injection/informers/resolution/v1beta1/resolutionrequest"
	"github.com/tektoncd/pipeline/pkg/pod"
	cloudeventclient "github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	resolution "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/pkg/spire"
	"github.com/tektoncd/pipeline/pkg/taskrunmetrics"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	limitrangeinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/limitrange"
	filteredpodinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod/filtered"
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
		podInformer := filteredpodinformer.Get(ctx, v1.ManagedByLabelKey)
		limitrangeInformer := limitrangeinformer.Get(ctx)
		verificationpolicyInformer := verificationpolicyinformer.Get(ctx)
		resolutionInformer := resolutioninformer.Get(ctx)
		spireClient := spire.GetControllerAPIClient(ctx)
		configStore := config.NewStore(logger.Named("config-store"), taskrunmetrics.MetricsOnStore(logger), spire.OnStore(ctx, logger))
		configStore.WatchConfigs(cmw)

		entrypointCache, err := pod.NewEntrypointCache(kubeclientset)
		if err != nil {
			logger.Fatalf("Error creating entrypoint cache: %v", err)
		}

		c := &Reconciler{
			KubeClientSet:            kubeclientset,
			PipelineClientSet:        pipelineclientset,
			Images:                   opts.Images,
			Clock:                    clock,
			spireClient:              spireClient,
			taskRunLister:            taskRunInformer.Lister(),
			limitrangeLister:         limitrangeInformer.Lister(),
			verificationPolicyLister: verificationpolicyInformer.Lister(),
			cloudEventClient:         cloudeventclient.Get(ctx),
			metrics:                  taskrunmetrics.Get(ctx),
			entrypointCache:          entrypointCache,
			podLister:                podInformer.Lister(),
			pvcHandler:               volumeclaim.NewPVCHandler(kubeclientset, logger),
			resolutionRequester:      resolution.NewCRDRequester(resolutionclient.Get(ctx), resolutionInformer.Lister()),
			tracerProvider:           tracerProvider,
		}
		impl := taskrunreconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
			return controller.Options{
				AgentName:   pipeline.TaskRunControllerName,
				ConfigStore: configStore,
			}
		})

		taskRunInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1.TaskRun{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
