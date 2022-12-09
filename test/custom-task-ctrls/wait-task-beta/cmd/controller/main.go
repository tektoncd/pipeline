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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	customruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/customrun"
	customrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/customrun"
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

	apiVersion := "wait.testing.tekton.dev/v1beta1"
	kind := "Wait"

	customruninformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		// TODO: Replace with the following once tkncontroller.FilterCustomRunRef is merged (fromhttps://github.com/tektoncd/pipeline/pull/5822)
		//      and released.
		// FilterFunc: tkncontroller.FilterCustomRunRef("wait.testing.tekton.dev/v1beta1", "Wait"),
		FilterFunc: func(obj interface{}) bool {
			r, ok := obj.(*v1beta1.CustomRun)
			if !ok {
				// Somehow got informed of a non-Run object.
				// Ignore.
				return false
			}
			if r == nil || (r.Spec.CustomRef == nil && r.Spec.CustomSpec == nil) {
				// These are invalid, but just in case they get
				// created somehow, don't panic.
				return false
			}
			result := false
			if r.Spec.CustomRef != nil {
				result = r.Spec.CustomRef.APIVersion == apiVersion && r.Spec.CustomRef.Kind == v1beta1.TaskKind(kind)
			} else if r.Spec.CustomSpec != nil {
				result = r.Spec.CustomSpec.APIVersion == apiVersion && r.Spec.CustomSpec.Kind == kind
			}
			return result
		},
		Handler: controller.HandleAll(impl.Enqueue),
	})

	return impl
}
