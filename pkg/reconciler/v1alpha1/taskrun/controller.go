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
	"time"

	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	clustertaskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/clustertask"
	resourceinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/pipelineresource"
	taskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/task"
	taskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/taskrun"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/kubeclient"
	podinformer "knative.dev/pkg/injection/informers/kubeinformers/corev1/pod"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
)

const (
	resyncPeriod = 10 * time.Hour
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)
	kubeclientset := kubeclient.Get(ctx)
	pipelineclientset := pipelineclient.Get(ctx)
	taskRunInformer := taskruninformer.Get(ctx)
	taskInformer := taskinformer.Get(ctx)
	clusterTaskInformer := clustertaskinformer.Get(ctx)
	podInformer := podinformer.Get(ctx)
	resourceInformer := resourceinformer.Get(ctx)
	timeoutHandler := reconciler.NewTimeoutHandler(ctx.Done(), logger)

	opt := reconciler.Options{
		KubeClientSet:     kubeclientset,
		PipelineClientSet: pipelineclientset,
		ConfigMapWatcher:  cmw,
		ResyncPeriod:      resyncPeriod,
		Logger:            logger,
	}

	c := &Reconciler{
		Base:              reconciler.NewBase(opt, taskRunAgentName),
		taskRunLister:     taskRunInformer.Lister(),
		taskLister:        taskInformer.Lister(),
		clusterTaskLister: clusterTaskInformer.Lister(),
		resourceLister:    resourceInformer.Lister(),
		timeoutHandler:    timeoutHandler,
	}
	impl := controller.NewImpl(c, c.Logger, taskRunControllerName)

	timeoutHandler.SetTaskRunCallbackFunc(impl.Enqueue)
	timeoutHandler.CheckTimeouts(kubeclientset, pipelineclientset)

	c.Logger.Info("Setting up event handlers")
	taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
	})

	c.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.EnqueueControllerOf,
		UpdateFunc: controller.PassNew(impl.EnqueueControllerOf),
		DeleteFunc: impl.EnqueueControllerOf,
	})

	// FIXME(vdemeester) it was never set
	//entrypoint cache will be initialized by controller if not provided
	c.Logger.Info("Setting up Entrypoint cache")
	c.cache = nil
	if c.cache == nil {
		c.cache, _ = entrypoint.NewCache()
	}

	return impl
}
