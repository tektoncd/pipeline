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
	"sort"
	"strings"

	"github.com/markbates/inflect"
	"github.com/mattbaird/jsonpatch"
	"go.uber.org/zap"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
)

// ResourceCallback defines a signature for resource specific (Route, Configuration, etc.)
// handlers that can validate and mutate an object. If non-nil error is returned, object mutation
// is denied. Mutations should be appended to the patches operations.
type ResourceCallback func(patches *[]jsonpatch.JsonPatchOperation, old GenericCRD, new GenericCRD) error

// ResourceDefaulter defines a signature for resource specific (Route, Configuration, etc.)
// handlers that can set defaults on an object. If non-nil error is returned, object mutation
// is denied. Mutations should be appended to the patches operations.
type ResourceDefaulter func(patches *[]jsonpatch.JsonPatchOperation, crd GenericCRD) error

// GenericCRD is the interface definition that allows us to perform the generic
// CRD actions like deciding whether to increment generation and so forth.
type GenericCRD interface {
	apis.Defaultable
	apis.Validatable
	runtime.Object
}

// ResourceAdmissionController implements the AdmissionController for resources
type ResourceAdmissionController struct {
	handlers map[schema.GroupVersionKind]GenericCRD
	options  ControllerOptions

	disallowUnknownFields bool
}

// NewResourceAdmissionController constructs a ResourceAdmissionController
func NewResourceAdmissionController(
	handlers map[schema.GroupVersionKind]GenericCRD,
	opts ControllerOptions,
	disallowUnknownFields bool) AdmissionController {
	return &ResourceAdmissionController{
		handlers:              handlers,
		options:               opts,
		disallowUnknownFields: disallowUnknownFields,
	}
}

func (ac *ResourceAdmissionController) Admit(ctx context.Context, request *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	logger := logging.FromContext(ctx)
	switch request.Operation {
	case admissionv1beta1.Create, admissionv1beta1.Update:
	default:
		logger.Infof("Unhandled webhook operation, letting it through %v", request.Operation)
		return &admissionv1beta1.AdmissionResponse{Allowed: true}
	}

	patchBytes, err := ac.mutate(ctx, request)
	if err != nil {
		return makeErrorStatus("mutation failed: %v", err)
	}
	logger.Infof("Kind: %q PatchBytes: %v", request.Kind, string(patchBytes))

	return &admissionv1beta1.AdmissionResponse{
		Patch:   patchBytes,
		Allowed: true,
		PatchType: func() *admissionv1beta1.PatchType {
			pt := admissionv1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (ac *ResourceAdmissionController) Register(ctx context.Context, kubeClient kubernetes.Interface, caCert []byte) error {
	client := kubeClient.AdmissionregistrationV1beta1().MutatingWebhookConfigurations()
	logger := logging.FromContext(ctx)
	failurePolicy := admissionregistrationv1beta1.Fail

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

	webhook := &admissionregistrationv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: ac.options.WebhookName,
		},
		Webhooks: []admissionregistrationv1beta1.Webhook{{
			Name:  ac.options.WebhookName,
			Rules: rules,
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Namespace: ac.options.Namespace,
					Name:      ac.options.ServiceName,
					Path:      &ac.options.ResourceAdmissionControllerPath,
				},
				CABundle: caCert,
			},
			FailurePolicy: &failurePolicy,
		}},
	}

	// Set the owner to our deployment.
	deployment, err := kubeClient.Apps().Deployments(ac.options.Namespace).Get(ac.options.DeploymentName, metav1.GetOptions{})
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
		configuredWebhook, err := client.Get(ac.options.WebhookName, metav1.GetOptions{})
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

func (ac *ResourceAdmissionController) mutate(ctx context.Context, req *admissionv1beta1.AdmissionRequest) ([]byte, error) {
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
		return nil, fmt.Errorf("unhandled kind: %v", gvk)
	}

	// nil values denote absence of `old` (create) or `new` (delete) objects.
	var oldObj, newObj GenericCRD

	if len(newBytes) != 0 {
		newObj = handler.DeepCopyObject().(GenericCRD)
		newDecoder := json.NewDecoder(bytes.NewBuffer(newBytes))
		if ac.disallowUnknownFields {
			newDecoder.DisallowUnknownFields()
		}
		if err := newDecoder.Decode(&newObj); err != nil {
			return nil, fmt.Errorf("cannot decode incoming new object: %v", err)
		}
	}
	if len(oldBytes) != 0 {
		oldObj = handler.DeepCopyObject().(GenericCRD)
		oldDecoder := json.NewDecoder(bytes.NewBuffer(oldBytes))
		if ac.disallowUnknownFields {
			oldDecoder.DisallowUnknownFields()
		}
		if err := oldDecoder.Decode(&oldObj); err != nil {
			return nil, fmt.Errorf("cannot decode incoming old object: %v", err)
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
			return nil, fmt.Errorf("cannot create patch for round tripped newBytes: %v", err)
		}
		patches = append(patches, rtp...)
	}

	// Set up the context for defaulting and validation
	if oldObj != nil {
		// Copy the old object and set defaults so that we don't reject our own
		// defaulting done earlier in the webhook.
		oldObj = oldObj.DeepCopyObject().(GenericCRD)
		oldObj.SetDefaults(ctx)

		s, ok := oldObj.(apis.HasSpec)
		if ok {
			SetUserInfoAnnotations(s, ctx, req.Resource.Group)
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
	if err := validate(ctx, newObj); err != nil {
		logger.Errorw("Failed the resource specific validation", zap.Error(err))
		// Return the error message as-is to give the validation callback
		// discretion over (our portion of) the message that the user sees.
		return nil, err
	}

	return json.Marshal(patches)
}

func (ac *ResourceAdmissionController) setUserInfoAnnotations(ctx context.Context, patches duck.JSONPatch, new GenericCRD, groupName string) (duck.JSONPatch, error) {
	if new == nil {
		return patches, nil
	}
	nh, ok := new.(apis.HasSpec)
	if !ok {
		return patches, nil
	}

	b, a := new.DeepCopyObject().(apis.HasSpec), nh

	SetUserInfoAnnotations(nh, ctx, groupName)

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
		return nil, fmt.Errorf("cannot marshal interface: %v", err)
	}
	return jsonpatch.CreatePatch(bytes, marshaledBytes)
}

// validate performs validation on the provided "new" CRD.
// For legacy purposes, this also does apis.Immutable validation,
// which is deprecated and will be removed in a future release.
func validate(ctx context.Context, new apis.Validatable) error {
	if apis.IsInUpdate(ctx) {
		old := apis.GetBaseline(ctx)
		if immutableNew, ok := new.(apis.Immutable); ok {
			immutableOld, ok := old.(apis.Immutable)
			if !ok {
				return fmt.Errorf("unexpected type mismatch %T vs. %T", old, new)
			}
			if err := immutableNew.CheckImmutableFields(ctx, immutableOld); err != nil {
				return err
			}
		}
	}

	// Can't just `return new.Validate()` because it doesn't properly nil-check.
	if err := new.Validate(ctx); err != nil {
		return err
	}

	return nil
}

// setDefaults simply leverages apis.Defaultable to set defaults.
func setDefaults(ctx context.Context, patches duck.JSONPatch, crd GenericCRD) (duck.JSONPatch, error) {
	before, after := crd.DeepCopyObject(), crd
	after.SetDefaults(ctx)

	patch, err := duck.CreatePatch(before, after)
	if err != nil {
		return nil, err
	}

	return append(patches, patch...), nil
}
