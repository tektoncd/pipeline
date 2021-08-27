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

package certificates

import (
	"context"

	// Injection stuff
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
)

// NewController constructs a controller for materializing webhook certificates.
// In order for it to bootstrap, an empty secret should be created with the
// expected name (and lifecycle managed accordingly), and thereafter this controller
// will ensure it has the appropriate shape for the webhook.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	client := kubeclient.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	options := webhook.GetOptions(ctx)

	key := types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      options.SecretName,
	}

	wh := &reconciler{
		LeaderAwareFuncs: pkgreconciler.LeaderAwareFuncs{
			// Enqueue the key whenever we become leader.
			PromoteFunc: func(bkt pkgreconciler.Bucket, enq func(pkgreconciler.Bucket, types.NamespacedName)) error {
				enq(bkt, key)
				return nil
			},
		},
		key:         key,
		serviceName: options.ServiceName,

		client:       client,
		secretlister: secretInformer.Lister(),
	}

	const queueName = "WebhookCertificates"
	c := controller.NewContext(ctx, wh, controller.ControllerOptions{WorkQueueName: queueName, Logger: logging.FromContext(ctx).Named(queueName)})

	// Reconcile when the cert bundle changes.
	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(key.Namespace, key.Name),
		// It doesn't matter what we enqueue because we will always Reconcile
		// the named MWH resource.
		Handler: controller.HandleAll(c.Enqueue),
	})

	return c
}
