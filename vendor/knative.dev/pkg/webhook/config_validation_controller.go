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

package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/markbates/inflect"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
)

// ConfigValidationController implements the AdmissionController for ConfigMaps
type ConfigValidationController struct {
	constructors map[string]reflect.Value
	options      ControllerOptions
}

// NewConfigValidationController constructs a ConfigValidationController
func NewConfigValidationController(
	constructors configmap.Constructors,
	opts ControllerOptions) AdmissionController {
	cfgValidations := &ConfigValidationController{
		constructors: make(map[string]reflect.Value),
		options:      opts,
	}

	for configName, constructor := range constructors {
		cfgValidations.registerConfig(configName, constructor)
	}

	return cfgValidations
}

func (ac *ConfigValidationController) Admit(ctx context.Context, request *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	logger := logging.FromContext(ctx)
	switch request.Operation {
	case admissionv1beta1.Create, admissionv1beta1.Update:
	default:
		logger.Infof("Unhandled webhook operation, letting it through %v", request.Operation)
		return &admissionv1beta1.AdmissionResponse{Allowed: true}
	}

	if err := ac.validate(ctx, request); err != nil {
		return makeErrorStatus("validation failed: %v", err)
	}

	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}
}

func (ac *ConfigValidationController) Register(ctx context.Context, kubeClient kubernetes.Interface, caCert []byte) error {
	client := kubeClient.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations()
	logger := logging.FromContext(ctx)
	failurePolicy := admissionregistrationv1beta1.Fail

	resourceGVK := corev1.SchemeGroupVersion.WithKind("ConfigMap")
	var rules []admissionregistrationv1beta1.RuleWithOperations
	plural := strings.ToLower(inflect.Pluralize(resourceGVK.Kind))

	ruleScope := admissionregistrationv1beta1.NamespacedScope
	rules = append(rules, admissionregistrationv1beta1.RuleWithOperations{
		Operations: []admissionregistrationv1beta1.OperationType{
			admissionregistrationv1beta1.Create,
			admissionregistrationv1beta1.Update,
		},
		Rule: admissionregistrationv1beta1.Rule{
			APIGroups:   []string{resourceGVK.Group},
			APIVersions: []string{resourceGVK.Version},
			Resources:   []string{plural + "/*"},
			Scope:       &ruleScope,
		},
	})

	webhook := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: ac.options.ConfigValidationWebhookName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{{
			Name:  ac.options.ConfigValidationWebhookName,
			Rules: rules,
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Namespace: ac.options.Namespace,
					Name:      ac.options.ServiceName,
					Path:      &ac.options.ConfigValidationControllerPath,
				},
				CABundle: caCert,
			},
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      ac.options.ConfigValidationNamespaceLabel,
					Operator: metav1.LabelSelectorOpExists,
				}},
			},
			FailurePolicy: &failurePolicy,
		}},
	}

	// Set the owner to our deployment.
	deployment, err := kubeClient.AppsV1().Deployments(ac.options.Namespace).Get(ac.options.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch our deployment: %v", err)
	}
	deploymentRef := metav1.NewControllerRef(deployment, deploymentKind)
	webhook.OwnerReferences = append(webhook.OwnerReferences, *deploymentRef)

	// Try to create the webhook and if it already exists validate webhook rules.
	_, err = client.Create(webhook)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create a webhook: %v", err)
		}
		logger.Info("Webhook already exists")
		configuredWebhook, err := client.Get(ac.options.ConfigValidationWebhookName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error retrieving webhook: %v", err)
		}
		if ok, err := kmp.SafeEqual(configuredWebhook.Webhooks, webhook.Webhooks); err != nil {
			return fmt.Errorf("error diffing webhooks: %v", err)
		} else if !ok {
			logger.Info("Updating webhook")
			// Set the ResourceVersion as required by update.
			webhook.ObjectMeta.ResourceVersion = configuredWebhook.ObjectMeta.ResourceVersion
			if _, err := client.Update(webhook); err != nil {
				return fmt.Errorf("failed to update webhook: %s", err)
			}
		} else {
			logger.Info("Webhook is already valid")
		}
	} else {
		logger.Info("Created a webhook")
	}

	return nil
}

func (ac *ConfigValidationController) validate(ctx context.Context, req *admissionv1beta1.AdmissionRequest) error {
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
			return fmt.Errorf("cannot decode incoming new object: %v", err)
		}
	}

	var err error
	if constructor, ok := ac.constructors[newObj.Name]; ok {

		inputs := []reflect.Value{
			reflect.ValueOf(&newObj),
		}

		outputs := constructor.Call(inputs)
		errVal := outputs[1]

		if !errVal.IsNil() {
			err = errVal.Interface().(error)
		}
	}

	return err
}

func (ac *ConfigValidationController) registerConfig(name string, constructor interface{}) {
	if err := configmap.ValidateConstructor(constructor); err != nil {
		panic(err)
	}

	ac.constructors[name] = reflect.ValueOf(constructor)
}
