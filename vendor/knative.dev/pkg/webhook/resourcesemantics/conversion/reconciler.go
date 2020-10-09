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
	"fmt"

	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apixclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apixlisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

type reconciler struct {
	pkgreconciler.LeaderAwareFuncs

	kinds       map[schema.GroupKind]GroupKindConversion
	path        string
	secretName  string
	withContext func(context.Context) context.Context

	secretLister corelisters.SecretLister
	crdLister    apixlisters.CustomResourceDefinitionLister
	client       apixclient.Interface
}

var _ webhook.ConversionController = (*reconciler)(nil)
var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)

// Path implements webhook.ConversionController
func (r *reconciler) Path() string {
	return r.path
}

// Reconciler implements controller.Reconciler
func (r *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	if !r.IsLeaderFor(types.NamespacedName{Name: key}) {
		logger.Debugf("Skipping key %q, not the leader.", key)
		return nil
	}

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := r.secretLister.Secrets(system.Namespace()).Get(r.secretName)
	if err != nil {
		logger.Errorf("Error fetching secret: %v", err)
		return err
	}

	cacert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", r.secretName, certresources.CACert)
	}

	return r.reconcileCRD(ctx, cacert, key)
}

func (r *reconciler) reconcileCRD(ctx context.Context, cacert []byte, key string) error {
	logger := logging.FromContext(ctx)

	configuredCRD, err := r.crdLister.Get(key)
	if err != nil {
		return fmt.Errorf("error retrieving crd: %w", err)
	}

	crd := configuredCRD.DeepCopy()

	if crd.Spec.Conversion == nil ||
		crd.Spec.Conversion.Strategy != apixv1.WebhookConverter ||
		crd.Spec.Conversion.Webhook.ClientConfig == nil ||
		crd.Spec.Conversion.Webhook.ClientConfig.Service == nil {
		return fmt.Errorf("custom resource %q isn't configured for webhook conversion", key)
	}

	crd.Spec.Conversion.Webhook.ClientConfig.CABundle = cacert
	crd.Spec.Conversion.Webhook.ClientConfig.Service.Path = ptr.String(r.path)

	if ok, err := kmp.SafeEqual(configuredCRD, crd); err != nil {
		return fmt.Errorf("error diffing custom resource definitions: %w", err)
	} else if !ok {
		logger.Infof("updating CRD")
		crdClient := r.client.ApiextensionsV1().CustomResourceDefinitions()
		if _, err := crdClient.Update(ctx, crd, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logger.Info("CRD is up to date")
	}

	return nil
}
