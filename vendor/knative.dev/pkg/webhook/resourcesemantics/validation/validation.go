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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/markbates/inflect"
	"go.uber.org/zap"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1beta1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var errMissingNewObject = errors.New("the new object may not be nil")

// reconciler implements the AdmissionController for resources
type reconciler struct {
	name     string
	path     string
	handlers map[schema.GroupVersionKind]resourcesemantics.GenericCRD

	withContext func(context.Context) context.Context

	client       kubernetes.Interface
	vwhlister    admissionlisters.ValidatingWebhookConfigurationLister
	secretlister corelisters.SecretLister

	disallowUnknownFields bool
	secretName            string
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := ac.secretlister.Secrets(system.Namespace()).Get(ac.secretName)
	if err != nil {
		logger.Errorw("Error fetching secret", zap.Error(err))
		return err
	}
	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", ac.secretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return ac.reconcileValidatingWebhook(ctx, caCert)
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	if ac.withContext != nil {
		ctx = ac.withContext(ctx)
	}

	logger := logging.FromContext(ctx)
	switch request.Operation {
	case admissionv1beta1.Create, admissionv1beta1.Update:
	default:
		logger.Infof("Unhandled webhook operation, letting it through %v", request.Operation)
		return &admissionv1beta1.AdmissionResponse{Allowed: true}
	}

	if err := ac.validate(ctx, request); err != nil {
		return webhook.MakeErrorStatus("validation failed: %v", err)
	}

	return &admissionv1beta1.AdmissionResponse{Allowed: true}
}

func (ac *reconciler) reconcileValidatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)

	var rules []admissionregistrationv1beta1.RuleWithOperations
	for gvk := range ac.handlers {
		plural := strings.ToLower(inflect.Pluralize(gvk.Kind))

		rules = append(rules, admissionregistrationv1beta1.RuleWithOperations{
			Operations: []admissionregistrationv1beta1.OperationType{
				admissionregistrationv1beta1.Create,
				admissionregistrationv1beta1.Update,
			},
			Rule: admissionregistrationv1beta1.Rule{
				APIGroups:   []string{gvk.Group},
				APIVersions: []string{gvk.Version},
				Resources:   []string{plural + "/*"},
			},
		})
	}

	// Sort the rules by Group, Version, Kind so that things are deterministically ordered.
	sort.Slice(rules, func(i, j int) bool {
		lhs, rhs := rules[i], rules[j]
		if lhs.APIGroups[0] != rhs.APIGroups[0] {
			return lhs.APIGroups[0] < rhs.APIGroups[0]
		}
		if lhs.APIVersions[0] != rhs.APIVersions[0] {
			return lhs.APIVersions[0] < rhs.APIVersions[0]
		}
		return lhs.Resources[0] < rhs.Resources[0]
	})

	configuredWebhook, err := ac.vwhlister.Get(ac.name)
	if err != nil {
		return fmt.Errorf("error retrieving webhook: %v", err)
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
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		webhook.Webhooks[i].ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, webhook); err != nil {
		return fmt.Errorf("error diffing webhooks: %v", err)
	} else if !ok {
		logger.Info("Updating webhook")
		vwhclient := ac.client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations()
		if _, err := vwhclient.Update(webhook); err != nil {
			return fmt.Errorf("failed to update webhook: %v", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}
	return nil
}

func (ac *reconciler) validate(ctx context.Context, req *admissionv1beta1.AdmissionRequest) error {
	kind := req.Kind
	newBytes := req.Object.Raw
	oldBytes := req.OldObject.Raw
	// Why, oh why are these different types...
	gvk := schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}

	logger := logging.FromContext(ctx)
	handler, ok := ac.handlers[gvk]
	if !ok {
		logger.Errorf("Unhandled kind: %v", gvk)
		return fmt.Errorf("unhandled kind: %v", gvk)
	}

	// nil values denote absence of `old` (create) or `new` (delete) objects.
	var oldObj, newObj resourcesemantics.GenericCRD

	if len(newBytes) != 0 {
		newObj = handler.DeepCopyObject().(resourcesemantics.GenericCRD)
		newDecoder := json.NewDecoder(bytes.NewBuffer(newBytes))
		if ac.disallowUnknownFields {
			newDecoder.DisallowUnknownFields()
		}
		if err := newDecoder.Decode(&newObj); err != nil {
			return fmt.Errorf("cannot decode incoming new object: %v", err)
		}
	}
	if len(oldBytes) != 0 {
		oldObj = handler.DeepCopyObject().(resourcesemantics.GenericCRD)
		oldDecoder := json.NewDecoder(bytes.NewBuffer(oldBytes))
		if ac.disallowUnknownFields {
			oldDecoder.DisallowUnknownFields()
		}
		if err := oldDecoder.Decode(&oldObj); err != nil {
			return fmt.Errorf("cannot decode incoming old object: %v", err)
		}
	}

	// Set up the context for defaulting and validation
	if oldObj != nil {
		if req.SubResource == "" {
			ctx = apis.WithinUpdate(ctx, oldObj)
		} else {
			ctx = apis.WithinSubResourceUpdate(ctx, oldObj, req.SubResource)
		}
	} else {
		ctx = apis.WithinCreate(ctx)
	}
	ctx = apis.WithUserInfo(ctx, &req.UserInfo)

	// None of the validators will accept a nil value for newObj.
	if newObj == nil {
		return errMissingNewObject
	}

	if err := validate(ctx, newObj); err != nil {
		logger.Errorw("Failed the resource specific validation", zap.Error(err))
		// Return the error message as-is to give the validation callback
		// discretion over (our portion of) the message that the user sees.
		return err
	}

	return nil
}

// validate performs validation on the provided "new" CRD.
func validate(ctx context.Context, new apis.Validatable) error {
	// Can't just `return new.Validate()` because it doesn't properly nil-check.
	if err := new.Validate(ctx); err != nil {
		return err
	}

	return nil
}
