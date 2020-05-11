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

package configmaps

import (
	"context"
	"reflect"

	// Injection stuff
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	vwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1beta1/validatingwebhookconfiguration"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"

	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
)

// NewAdmissionController constructs a reconciler
func NewAdmissionController(
	ctx context.Context,
	name, path string,
	constructors configmap.Constructors,
) *controller.Impl {

	client := kubeclient.Get(ctx)
	vwhInformer := vwhinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	options := webhook.GetOptions(ctx)

	wh := &reconciler{
		name: name,
		path: path,

		constructors: make(map[string]reflect.Value),
		secretName:   options.SecretName,

		client:       client,
		vwhlister:    vwhInformer.Lister(),
		secretlister: secretInformer.Lister(),
	}

	for configName, constructor := range constructors {
		wh.registerConfig(configName, constructor)
	}

	logger := logging.FromContext(ctx)
	c := controller.NewImpl(wh, logger, "ConfigMapWebhook")

	// Reconcile when the named ValidatingWebhookConfiguration changes.
	vwhInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName(name),
		// It doesn't matter what we enqueue because we will always Reconcile
		// the named VWH resource.
		Handler: controller.HandleAll(c.Enqueue),
	})

	// Reconcile when the cert bundle changes.
	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), wh.secretName),
		// It doesn't matter what we enqueue because we will always Reconcile
		// the named VWH resource.
		Handler: controller.HandleAll(c.Enqueue),
	})

	return c
}
