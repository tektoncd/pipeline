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
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	clustertaskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/clustertask"
	conditioninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/condition"
	pipelineinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/pipeline"
	pipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/pipelinerun"
	taskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/task"
	taskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/taskrun"
	resourceinformer "github.com/tektoncd/pipeline/pkg/client/resource/injection/informers/resource/v1alpha1/pipelineresource"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/config"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
)

const (
	resyncPeriod = 10 * time.Hour
)

func NewController(images pipeline.Images) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)
		kubeclientset := kubeclient.Get(ctx)
		pipelineclientset := pipelineclient.Get(ctx)
		taskRunInformer := taskruninformer.Get(ctx)
		taskInformer := taskinformer.Get(ctx)
		clusterTaskInformer := clustertaskinformer.Get(ctx)
		pipelineRunInformer := pipelineruninformer.Get(ctx)
		pipelineInformer := pipelineinformer.Get(ctx)
		resourceInformer := resourceinformer.Get(ctx)
		conditionInformer := conditioninformer.Get(ctx)
		timeoutHandler := reconciler.NewTimeoutHandler(ctx.Done(), logger)
		metrics, err := NewRecorder()
		if err != nil {
			logger.Errorf("Failed to create pipelinerun metrics recorder %v", err)
		}

		opt := reconciler.Options{
			KubeClientSet:     kubeclientset,
			PipelineClientSet: pipelineclientset,
			ConfigMapWatcher:  cmw,
			ResyncPeriod:      resyncPeriod,
			Logger:            logger,
		}

		c := &Reconciler{
			Base:              reconciler.NewBase(opt, pipelineRunAgentName, images),
			pipelineRunLister: pipelineRunInformer.Lister(),
			pipelineLister:    pipelineInformer.Lister(),
			taskLister:        taskInformer.Lister(),
			clusterTaskLister: clusterTaskInformer.Lister(),
			taskRunLister:     taskRunInformer.Lister(),
			resourceLister:    resourceInformer.Lister(),
			conditionLister:   conditionInformer.Lister(),
			timeoutHandler:    timeoutHandler,
			metrics:           metrics,
		}
		impl := controller.NewImpl(c, c.Logger, pipeline.PipelineRunControllerName)

		timeoutHandler.SetPipelineRunCallbackFunc(impl.Enqueue)
		timeoutHandler.CheckTimeouts(kubeclientset, pipelineclientset)

		c.Logger.Info("Setting up event handlers")
		pipelineRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    impl.Enqueue,
			UpdateFunc: controller.PassNew(impl.Enqueue),
			DeleteFunc: impl.Enqueue,
		})

		c.tracker = tracker.New(impl.EnqueueKey, 30*time.Minute)
		taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			UpdateFunc: controller.PassNew(impl.EnqueueControllerOf),
		})

		c.Logger.Info("Setting up ConfigMap receivers")
		c.configStore = config.NewStore(images, c.Logger.Named("config-store"))
		c.configStore.WatchConfigs(opt.ConfigMapWatcher)

		return impl
	}
}
