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

package validation

import (
	"context"

	// Injection stuff
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	vwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1beta1/validatingwebhookconfiguration"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// NewAdmissionController constructs a reconciler
func NewAdmissionController(
	ctx context.Context,
	name, path string,
	handlers map[schema.GroupVersionKind]resourcesemantics.GenericCRD,
	wc func(context.Context) context.Context,
	disallowUnknownFields bool,
	callbacks ...map[schema.GroupVersionKind]Callback,
) *controller.Impl {

	client := kubeclient.Get(ctx)
	vwhInformer := vwhinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	options := webhook.GetOptions(ctx)

	// This not ideal, we are using a variadic argument to effectively make callbacks optional
	// This allows this addition to be non-breaking to consumers of /pkg
	// TODO: once all sub-repos have adoped this, we might move this back to a traditional param.
	var unwrappedCallbacks map[schema.GroupVersionKind]Callback
	switch len(callbacks) {
	case 0:
		unwrappedCallbacks = map[schema.GroupVersionKind]Callback{}
	case 1:
		unwrappedCallbacks = callbacks[0]
	default:
		panic("NewAdmissionController may not be called with multiple callback maps")
	}

	wh := &reconciler{
		name:      name,
		path:      path,
		handlers:  handlers,
		callbacks: unwrappedCallbacks,

		withContext:           wc,
		disallowUnknownFields: disallowUnknownFields,
		secretName:            options.SecretName,

		client:       client,
		vwhlister:    vwhInformer.Lister(),
		secretlister: secretInformer.Lister(),
	}

	logger := logging.FromContext(ctx)
	c := controller.NewImpl(wh, logger, "ValidationWebhook")

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
