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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	clustertaskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/clustertask"
	taskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/task"
	taskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/taskrun"
	taskrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/taskrun"
	resourceinformer "github.com/tektoncd/pipeline/pkg/client/resource/injection/informers/resource/v1alpha1/pipelineresource"
	"github.com/tektoncd/pipeline/pkg/pod"
	cloudeventclient "github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"github.com/tektoncd/pipeline/pkg/timeout"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
)

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
func NewController(namespace string, images pipeline.Images) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)
		kubeclientset := kubeclient.Get(ctx)
		pipelineclientset := pipelineclient.Get(ctx)
		taskRunInformer := taskruninformer.Get(ctx)
		taskInformer := taskinformer.Get(ctx)
		clusterTaskInformer := clustertaskinformer.Get(ctx)
		podInformer := podinformer.Get(ctx)
		resourceInformer := resourceinformer.Get(ctx)
		timeoutHandler := timeout.NewHandler(ctx.Done(), logger)
		metrics, err := NewRecorder()
		if err != nil {
			logger.Errorf("Failed to create taskrun metrics recorder %v", err)
		}

		entrypointCache, err := pod.NewEntrypointCache(kubeclientset)
		if err != nil {
			logger.Fatalf("Error creating entrypoint cache: %v", err)
		}

		c := &Reconciler{
			KubeClientSet:     kubeclientset,
			PipelineClientSet: pipelineclientset,
			Images:            images,
			taskRunLister:     taskRunInformer.Lister(),
			taskLister:        taskInformer.Lister(),
			clusterTaskLister: clusterTaskInformer.Lister(),
			resourceLister:    resourceInformer.Lister(),
			timeoutHandler:    timeoutHandler,
			cloudEventClient:  cloudeventclient.Get(ctx),
			metrics:           metrics,
			entrypointCache:   entrypointCache,
			pvcHandler:        volumeclaim.NewPVCHandler(kubeclientset, logger),
		}
		impl := taskrunreconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
			configStore := config.NewStore(logger.Named("config-store"))
			configStore.WatchConfigs(cmw)

			return controller.Options{
				AgentName:   pipeline.TaskRunControllerName,
				ConfigStore: configStore,
			}
		})

		timeoutHandler.SetCallbackFunc(impl.EnqueueKey)
		go timeoutHandler.CheckTimeouts(ctx, namespace, kubeclientset, pipelineclientset)

		logger.Info("Setting up event handlers")
		taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    impl.Enqueue,
			UpdateFunc: controller.PassNew(impl.Enqueue),
		})

		c.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))

		podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterGroupKind(v1beta1.Kind("TaskRun")),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		go metrics.ReportRunningTaskRuns(ctx, taskRunInformer.Lister())

		return impl
	}
}
