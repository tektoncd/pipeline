/*
Copyright 2019 The Knative Authors

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

package hpa

import (
	"context"

	autoscalingv1 "k8s.io/client-go/informers/autoscaling/v1"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/injection"
	"github.com/knative/pkg/injection/informers/kubeinformers/factory"
	"github.com/knative/pkg/logging"
)

func init() {
	injection.Default.RegisterInformer(withInformer)
}

// Key is used as the key for associating information
// with a context.Context.
type Key struct{}

func withInformer(ctx context.Context) (context.Context, controller.Informer) {
	f := factory.Get(ctx)
	inf := f.Autoscaling().V1().HorizontalPodAutoscalers()
	return context.WithValue(ctx, Key{}, inf), inf.Informer()
}

// Get extracts the Kubernetes Hpa informer from the context.
func Get(ctx context.Context) autoscalingv1.HorizontalPodAutoscalerInformer {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		logging.FromContext(ctx).Panicf(
			"Unable to fetch %T from context.", (autoscalingv1.HorizontalPodAutoscalerInformer)(nil))
	}
	return untyped.(autoscalingv1.HorizontalPodAutoscalerInformer)
}
