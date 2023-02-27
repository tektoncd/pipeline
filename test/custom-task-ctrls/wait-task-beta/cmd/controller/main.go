/*
Copyright 2021 The Tekton Authors

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

package main // import "github.com/tektoncd/pipeline/test/wait-task-beta/cmd/controller"

import (
	"context"

	customruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/customrun"
	customrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/customrun"
	tkncontroller "github.com/tektoncd/pipeline/pkg/controller"
	"github.com/tektoncd/pipeline/test/wait-task-beta/pkg/reconciler"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
)

const controllerName = "wait-task-controller"

func main() {
	sharedmain.Main(controllerName, newController)
}

func newController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	c := &reconciler.Reconciler{
		Clock: clock.RealClock{},
	}
	impl := customrunreconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			AgentName: controllerName,
		}
	})

	customruninformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: tkncontroller.FilterCustomRunRef("wait.testing.tekton.dev/v1beta1", "Wait"),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	return impl
}
