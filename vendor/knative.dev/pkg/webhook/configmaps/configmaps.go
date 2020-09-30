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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

// reconciler implements the AdmissionController for ConfigMaps
type reconciler struct {
	webhook.StatelessAdmissionImpl
	pkgreconciler.LeaderAwareFuncs

	key          types.NamespacedName
	path         string
	constructors map[string]reflect.Value

	client       kubernetes.Interface
	vwhlister    admissionlisters.ValidatingWebhookConfigurationLister
	secretlister corelisters.SecretLister

	secretName string
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)
var _ webhook.StatelessAdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	if !ac.IsLeaderFor(ac.key) {
		logger.Debugf("Skipping key %q, not the leader.", ac.key)
		return nil
	}

	secret, err := ac.secretlister.Secrets(system.Namespace()).Get(ac.secretName)
	if err != nil {
		logger.Errorf("Error fetching secret: %v", err)
		return err
	}

	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", ac.secretName, certresources.CACert)
	}

	return ac.reconcileValidatingWebhook(ctx, caCert)
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	logger := logging.FromContext(ctx)
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
	default:
		logger.Infof("Unhandled webhook operation, letting it through %v", request.Operation)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	if err := ac.validate(ctx, request); err != nil {
		return webhook.MakeErrorStatus("validation failed: %v", err)
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

func (ac *reconciler) reconcileValidatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)

	ruleScope := admissionregistrationv1.NamespacedScope
	rules := []admissionregistrationv1.RuleWithOperations{{
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{""},
			APIVersions: []string{"v1"},
			Resources:   []string{"configmaps/*"},
			Scope:       &ruleScope,
		},
	}}

	configuredWebhook, err := ac.vwhlister.Get(ac.key.Name)
	if err != nil {
		return fmt.Errorf("error retrieving webhook: %w", err)
	}

	webhook := configuredWebhook.DeepCopy()

	// Clear out any previous (bad) OwnerReferences.
	// See: https://github.com/knative/serving/issues/5845
	webhook.OwnerReferences = nil

	for i, wh := range webhook.Webhooks {
		if wh.Name != webhook.Name {
			continue
		}
		webhook.Webhooks[i].Rules = rules
		webhook.Webhooks[i].ClientConfig.CABundle = caCert
		if webhook.Webhooks[i].ClientConfig.Service == nil {
			return errors.New("missing service reference for webhook: " + wh.Name)
		}
		webhook.Webhooks[i].ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, webhook); err != nil {
		return fmt.Errorf("error diffing webhooks: %w", err)
	} else if !ok {
		logger.Info("Updating webhook")
		vwhclient := ac.client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
		if _, err := vwhclient.Update(ctx, webhook, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}

	return nil
}

func (ac *reconciler) validate(ctx context.Context, req *admissionv1.AdmissionRequest) error {
	logger := logging.FromContext(ctx)
	kind := req.Kind
	newBytes := req.Object.Raw

	// Why, oh why are these different types...
	gvk := schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}

	resourceGVK := corev1.SchemeGroupVersion.WithKind("ConfigMap")
	if gvk != resourceGVK {
		logger.Errorf("Unhandled kind: %v", gvk)
		return fmt.Errorf("unhandled kind: %v", gvk)
	}

	var newObj corev1.ConfigMap
	if len(newBytes) != 0 {
		newDecoder := json.NewDecoder(bytes.NewBuffer(newBytes))
		if err := newDecoder.Decode(&newObj); err != nil {
			return fmt.Errorf("cannot decode incoming new object: %w", err)
		}
	}

	if constructor, ok := ac.constructors[newObj.Name]; ok {
		// Only validate example data if this is a configMap we know about.
		exampleData, hasExampleData := newObj.Data[configmap.ExampleKey]
		exampleChecksum, hasExampleChecksumAnnotation := newObj.Annotations[configmap.ExampleChecksumAnnotation]
		if hasExampleData && hasExampleChecksumAnnotation &&
			exampleChecksum != configmap.Checksum(exampleData) {
			return fmt.Errorf(
				"the update modifies a key in %q which is probably not what you want. Instead, copy the respective setting to the top-level of the ConfigMap, directly below %q",
				configmap.ExampleKey, "data")
		}

		inputs := []reflect.Value{
			reflect.ValueOf(&newObj),
		}

		outputs := constructor.Call(inputs)
		errVal := outputs[1]

		if !errVal.IsNil() {
			return errVal.Interface().(error)
		}
	}

	return nil
}

func (ac *reconciler) registerConfig(name string, constructor interface{}) {
	if err := configmap.ValidateConstructor(constructor); err != nil {
		panic(err)
	}

	ac.constructors[name] = reflect.ValueOf(constructor)
}
