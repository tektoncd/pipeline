/*
Copyright 2020 The Knative Authors

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

package conversion

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	apixclient "knative.dev/pkg/client/injection/apiextensions/client"
	crdinformer "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1/customresourcedefinition"
	"knative.dev/pkg/controller"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
)

// ConvertibleObject defines the functionality our API types
// are required to implement in order to be convertible from
// one version to another
//
// Optionally if the object implements apis.Defaultable the
// ConversionController will apply defaults before returning
// the response
type ConvertibleObject interface {
	// ConvertTo(ctx, to)
	// ConvertFrom(ctx, from)
	apis.Convertible

	// DeepCopyObject()
	// GetObjectKind() => SetGroupVersionKind(gvk)
	runtime.Object
}

// GroupKindConversion specifies how a specific Kind for a given
// group should be converted
type GroupKindConversion struct {
	// DefinitionName specifies the CustomResourceDefinition that should
	// be reconciled with by the controller.
	//
	// The conversion webhook configuration will be updated
	// when the CA bundle changes
	DefinitionName string

	// HubVersion specifies which version of the CustomResource supports
	// conversions to and from all types
	//
	// It is expected that the Zygotes map contains an entry for the
	// specified HubVersion
	HubVersion string

	// Zygotes contains a map of version strings (ie. v1, v2) to empty
	// ConvertibleObject objects
	//
	// During a conversion request these zygotes will be deep copied
	// and manipulated using the apis.Convertible interface
	Zygotes map[string]ConvertibleObject
}

// NewConversionController returns a K8s controller that will
// will reconcile CustomResourceDefinitions and update their
// conversion webhook attributes such as path & CA bundle.
//
// Additionally the controller's Reconciler implements
// webhook.ConversionController for the purposes of converting
// resources between different versions
func NewConversionController(
	ctx context.Context,
	path string,
	kinds map[schema.GroupKind]GroupKindConversion,
	withContext func(context.Context) context.Context,
) *controller.Impl {

	secretInformer := secretinformer.Get(ctx)
	crdInformer := crdinformer.Get(ctx)
	client := apixclient.Get(ctx)
	options := webhook.GetOptions(ctx)

	r := &reconciler{
		LeaderAwareFuncs: pkgreconciler.LeaderAwareFuncs{
			// Have this reconciler enqueue our types whenever it becomes leader.
			PromoteFunc: func(bkt pkgreconciler.Bucket, enq func(pkgreconciler.Bucket, types.NamespacedName)) error {
				for _, gkc := range kinds {
					name := gkc.DefinitionName
					enq(bkt, types.NamespacedName{Name: name})
				}
				return nil
			},
		},

		kinds:       kinds,
		path:        path,
		secretName:  options.SecretName,
		withContext: withContext,

		client:       client,
		secretLister: secretInformer.Lister(),
		crdLister:    crdInformer.Lister(),
	}

	const queueName = "ConversionWebhook"
	logger := logging.FromContext(ctx)
	c := controller.NewContext(ctx, r, controller.ControllerOptions{WorkQueueName: queueName, Logger: logger.Named(queueName)})

	// Reconciler when the named CRDs change.
	for _, gkc := range kinds {
		name := gkc.DefinitionName

		crdInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterWithName(name),
			Handler:    controller.HandleAll(c.Enqueue),
		})

		sentinel := c.EnqueueSentinel(types.NamespacedName{Name: name})

		// Reconcile when the cert bundle changes.
		secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), options.SecretName),
			Handler:    controller.HandleAll(sentinel),
		})
	}

	return c
}
