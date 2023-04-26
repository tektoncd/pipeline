package node

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"

	"github.com/tektoncd/pipeline/pkg/pipelinerunmetrics"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	nodeinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/node"
	nodereconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/node"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
func NewController() func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)
		kubeclientset := kubeclient.Get(ctx)
		nodeInformer := nodeinformer.Get(ctx)
		configStore := config.NewStore(logger.Named("config-store"), pipelinerunmetrics.MetricsOnStore(logger))
		configStore.WatchConfigs(cmw)

		c := &Reconciler{
			KubeClientSet: kubeclientset,
		}

		impl := nodereconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
			return controller.Options{
				AgentName:         "pod",
				SkipStatusUpdates: true,
				ConfigStore:       configStore,
			}
		})

		nodeInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
		return impl
	}
}
