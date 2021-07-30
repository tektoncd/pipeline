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

package defaulting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gobuffalo/flect"
	"go.uber.org/zap"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var errMissingNewObject = errors.New("the new object may not be nil")

// reconciler implements the AdmissionController for resources
type reconciler struct {
	webhook.StatelessAdmissionImpl
	pkgreconciler.LeaderAwareFuncs

	key      types.NamespacedName
	path     string
	handlers map[schema.GroupVersionKind]resourcesemantics.GenericCRD

	withContext func(context.Context) context.Context

	client       kubernetes.Interface
	mwhlister    admissionlisters.MutatingWebhookConfigurationLister
	secretlister corelisters.SecretLister

	disallowUnknownFields bool
	secretName            string
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)
var _ webhook.StatelessAdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	if !ac.IsLeaderFor(ac.key) {
		return controller.NewSkipKey(key)
	}

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
	return ac.reconcileMutatingWebhook(ctx, caCert)
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if ac.withContext != nil {
		ctx = ac.withContext(ctx)
	}

	logger := logging.FromContext(ctx)
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
	default:
		logger.Info("Unhandled webhook operation, letting it through ", request.Operation)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	patchBytes, err := ac.mutate(ctx, request)
	if err != nil {
		return webhook.MakeErrorStatus("mutation failed: %v", err)
	}
	logger.Infof("Kind: %q PatchBytes: %v", request.Kind, string(patchBytes))

	return &admissionv1.AdmissionResponse{
		Patch:   patchBytes,
		Allowed: true,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (ac *reconciler) reconcileMutatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)

	rules := make([]admissionregistrationv1.RuleWithOperations, 0, len(ac.handlers))
	for gvk := range ac.handlers {
		plural := strings.ToLower(flect.Pluralize(gvk.Kind))

		rules = append(rules, admissionregistrationv1.RuleWithOperations{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{gvk.Group},
				APIVersions: []string{gvk.Version},
				Resources:   []string{plural, plural + "/status"},
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

	configuredWebhook, err := ac.mwhlister.Get(ac.key.Name)
	if err != nil {
		return fmt.Errorf("error retrieving webhook: %w", err)
	}

	current := configuredWebhook.DeepCopy()

	ns, err := ac.client.CoreV1().Namespaces().Get(ctx, system.Namespace(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch namespace: %w", err)
	}
	nsRef := *metav1.NewControllerRef(ns, corev1.SchemeGroupVersion.WithKind("Namespace"))
	current.OwnerReferences = []metav1.OwnerReference{nsRef}

	for i, wh := range current.Webhooks {
		if wh.Name != current.Name {
			continue
		}

		cur := &current.Webhooks[i]
		cur.Rules = rules

		cur.NamespaceSelector = webhook.EnsureLabelSelectorExpressions(
			cur.NamespaceSelector,
			&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "webhooks.knative.dev/exclude",
					Operator: metav1.LabelSelectorOpDoesNotExist,
				}},
			})

		cur.ClientConfig.CABundle = caCert
		if cur.ClientConfig.Service == nil {
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		cur.ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, current); err != nil {
		return fmt.Errorf("error diffing webhooks: %w", err)
	} else if !ok {
		logger.Info("Updating webhook")
		mwhclient := ac.client.AdmissionregistrationV1().MutatingWebhookConfigurations()
		if _, err := mwhclient.Update(ctx, current, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}
	return nil
}

func (ac *reconciler) mutate(ctx context.Context, req *admissionv1.AdmissionRequest) ([]byte, error) {
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
		logger.Error("Unhandled kind: ", gvk)
		return nil, fmt.Errorf("unhandled kind: %v", gvk)
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
			return nil, fmt.Errorf("cannot decode incoming new object: %w", err)
		}
	}
	if len(oldBytes) != 0 {
		oldObj = handler.DeepCopyObject().(resourcesemantics.GenericCRD)
		oldDecoder := json.NewDecoder(bytes.NewBuffer(oldBytes))
		if ac.disallowUnknownFields {
			oldDecoder.DisallowUnknownFields()
		}
		if err := oldDecoder.Decode(&oldObj); err != nil {
			return nil, fmt.Errorf("cannot decode incoming old object: %w", err)
		}
	}
	var patches duck.JSONPatch

	var err error
	// Skip this step if the type we're dealing with is a duck type, since it is inherently
	// incomplete and this will patch away all of the unspecified fields.
	if _, ok := newObj.(duck.Populatable); !ok {
		// Add these before defaulting fields, otherwise defaulting may cause an illegal patch
		// because it expects the round tripped through Golang fields to be present already.
		rtp, err := roundTripPatch(newBytes, newObj)
		if err != nil {
			return nil, fmt.Errorf("cannot create patch for round tripped newBytes: %w", err)
		}
		patches = append(patches, rtp...)
	}

	// Set up the context for defaulting and validation
	if oldObj != nil {
		// Copy the old object and set defaults so that we don't reject our own
		// defaulting done earlier in the webhook.
		oldObj = oldObj.DeepCopyObject().(resourcesemantics.GenericCRD)
		oldObj.SetDefaults(ctx)

		s, ok := oldObj.(apis.HasSpec)
		if ok {
			setUserInfoAnnotations(ctx, s, req.Resource.Group)
		}

		if req.SubResource == "" {
			ctx = apis.WithinUpdate(ctx, oldObj)
		} else {
			ctx = apis.WithinSubResourceUpdate(ctx, oldObj, req.SubResource)
		}
	} else {
		ctx = apis.WithinCreate(ctx)
	}
	ctx = apis.WithUserInfo(ctx, &req.UserInfo)

	// Default the new object.
	if patches, err = setDefaults(ctx, patches, newObj); err != nil {
		logger.Errorw("Failed the resource specific defaulter", zap.Error(err))
		// Return the error message as-is to give the defaulter callback
		// discretion over (our portion of) the message that the user sees.
		return nil, err
	}

	if patches, err = ac.setUserInfoAnnotations(ctx, patches, newObj, req.Resource.Group); err != nil {
		logger.Errorw("Failed the resource user info annotator", zap.Error(err))
		return nil, err
	}

	// None of the validators will accept a nil value for newObj.
	if newObj == nil {
		return nil, errMissingNewObject
	}
	return json.Marshal(patches)
}

func (ac *reconciler) setUserInfoAnnotations(ctx context.Context, patches duck.JSONPatch, new resourcesemantics.GenericCRD, groupName string) (duck.JSONPatch, error) {
	if new == nil {
		return patches, nil
	}
	nh, ok := new.(apis.HasSpec)
	if !ok {
		return patches, nil
	}

	b, a := new.DeepCopyObject().(apis.HasSpec), nh

	setUserInfoAnnotations(ctx, nh, groupName)

	patch, err := duck.CreatePatch(b, a)
	if err != nil {
		return nil, err
	}
	return append(patches, patch...), nil
}

// roundTripPatch generates the JSONPatch that corresponds to round tripping the given bytes through
// the Golang type (JSON -> Golang type -> JSON). Because it is not always true that
// bytes == json.Marshal(json.Unmarshal(bytes)).
//
// For example, if bytes did not contain a 'spec' field and the Golang type specifies its 'spec'
// field without omitempty, then by round tripping through the Golang type, we would have added
// `'spec': {}`.
func roundTripPatch(bytes []byte, unmarshalled interface{}) (duck.JSONPatch, error) {
	if unmarshalled == nil {
		return duck.JSONPatch{}, nil
	}
	marshaledBytes, err := json.Marshal(unmarshalled)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal interface: %w", err)
	}
	return jsonpatch.CreatePatch(bytes, marshaledBytes)
}

// setDefaults simply leverages apis.Defaultable to set defaults.
func setDefaults(ctx context.Context, patches duck.JSONPatch, crd resourcesemantics.GenericCRD) (duck.JSONPatch, error) {
	before, after := crd.DeepCopyObject(), crd
	after.SetDefaults(ctx)

	patch, err := duck.CreatePatch(before, after)
	if err != nil {
		return nil, err
	}

	return append(patches, patch...), nil
}
