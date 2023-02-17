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

package validation

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/json"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var errMissingNewObject = errors.New("the new object may not be nil")

// Callback is a generic function to be called by a consumer of validation
type Callback struct {
	// function is the callback to be invoked
	function func(ctx context.Context, unstructured *unstructured.Unstructured) error

	// supportedVerbs are the verbs supported for the callback.
	supportedVerbs map[webhook.Operation]struct{}
}

// NewCallback creates a new callback function to be invoked on supported verbs.
func NewCallback(function func(context.Context, *unstructured.Unstructured) error, supportedVerbs ...webhook.Operation) Callback {
	m := make(map[webhook.Operation]struct{})
	for _, op := range supportedVerbs {
		if _, has := m[op]; has {
			panic("duplicate verbs not allowed")
		}
		m[op] = struct{}{}
	}
	return Callback{function: function, supportedVerbs: m}
}

var _ webhook.AdmissionController = (*reconciler)(nil)

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) (resp *admissionv1.AdmissionResponse) {
	if ac.withContext != nil {
		ctx = ac.withContext(ctx)
	}

	kind := request.Kind
	gvk := schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}

	ctx, resource, err := ac.decodeRequestAndPrepareContext(ctx, request, gvk)
	if err != nil {
		return webhook.MakeErrorStatus("decoding request failed: %v", err)
	}

	errors, warnings := validate(ctx, resource, request)
	if warnings != nil {
		// If there were warnings, then keep processing things, but augment
		// whatever AdmissionResponse we send with the warnings.  We cannot
		// simply set `resp.Warnings` directly here because the return paths
		// below all overwrite `resp`, but the `defer` affords us one final
		// crack at things.
		defer func() {
			resp.Warnings = make([]string, 0, len(warnings))
			for _, w := range warnings {
				resp.Warnings = append(resp.Warnings, w.Error())
			}
		}()
	}
	if errors != nil {
		return webhook.MakeErrorStatus("validation failed: %v", errors)
	}

	if err := ac.callback(ctx, request, gvk); err != nil {
		return webhook.MakeErrorStatus("validation callback failed: %v", err)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}
}

// decodeRequestAndPrepareContext deserializes the old and new GenericCrds from the incoming request and sets up the context.
// nil oldObj or newObj denote absence of `old` (create) or `new` (delete) objects.
func (ac *reconciler) decodeRequestAndPrepareContext(
	ctx context.Context,
	req *admissionv1.AdmissionRequest,
	gvk schema.GroupVersionKind) (context.Context, resourcesemantics.GenericCRD, error) {

	logger := logging.FromContext(ctx)
	handler, ok := ac.handlers[gvk]
	if !ok {
		logger.Error("Unhandled kind: ", gvk)
		return ctx, nil, fmt.Errorf("unhandled kind: %v", gvk)
	}

	newBytes := req.Object.Raw
	oldBytes := req.OldObject.Raw

	// Decode json to a GenericCRD
	var newObj resourcesemantics.GenericCRD
	if len(newBytes) != 0 {
		newObj = handler.DeepCopyObject().(resourcesemantics.GenericCRD)
		err := json.Decode(newBytes, newObj, ac.disallowUnknownFields)
		if err != nil {
			return ctx, nil, fmt.Errorf("cannot decode incoming new object: %w", err)
		}
	}

	var oldObj resourcesemantics.GenericCRD
	if len(oldBytes) != 0 {
		oldObj = handler.DeepCopyObject().(resourcesemantics.GenericCRD)
		err := json.Decode(oldBytes, oldObj, ac.disallowUnknownFields)
		if err != nil {
			return ctx, nil, fmt.Errorf("cannot decode incoming old object: %w", err)
		}
	}

	ctx = apis.WithUserInfo(ctx, &req.UserInfo)
	ctx = context.WithValue(ctx, kubeclient.Key{}, ac.client)
	if req.DryRun != nil && *req.DryRun {
		ctx = apis.WithDryRun(ctx)
	}

	switch req.Operation {
	case admissionv1.Update:
		if req.SubResource != "" {
			ctx = apis.WithinSubResourceUpdate(ctx, oldObj, req.SubResource)
		} else {
			ctx = apis.WithinUpdate(ctx, oldObj)
		}
	case admissionv1.Create:
		ctx = apis.WithinCreate(ctx)
	case admissionv1.Delete:
		ctx = apis.WithinDelete(ctx)
		return ctx, oldObj, nil
	}

	return ctx, newObj, nil
}

func validate(ctx context.Context, resource resourcesemantics.GenericCRD, req *admissionv1.AdmissionRequest) (err error, warn []error) { //nolint
	logger := logging.FromContext(ctx)

	// Only run validation for supported create and update validation.
	switch req.Operation {
	case admissionv1.Create, admissionv1.Update:
		// Supported verbs
	case admissionv1.Delete:
		return nil, nil // Validation handled by optional Callback, but not validatable.
	default:
		logger.Info("Unhandled webhook validation operation, letting it through ", req.Operation)
		return nil, nil
	}

	// None of the validators will accept a nil value for newObj.
	if resource == nil {
		return errMissingNewObject, nil
	}

	if result := resource.Validate(ctx); result != nil {
		logger.Errorw("Failed the resource specific validation", zap.Error(err))
		// While we have the strong typing of apis.FieldError, partition the
		// returned error into the error-level diagnostics and warning-level
		// diagnostics, so that the admission response can embed things into
		// the appropriate portions of the response.
		// This is expanded like to to avoid problems with typed nils.
		if errorResult := result.Filter(apis.ErrorLevel); errorResult != nil {
			err = errorResult
		}
		if warningResult := result.Filter(apis.WarningLevel); warningResult != nil {
			ws := warningResult.WrappedErrors()
			warn = make([]error, 0, len(ws))
			for _, w := range ws {
				warn = append(warn, w)
			}
		}
	}
	return err, warn
}

// callback runs optional callbacks on admission
func (ac *reconciler) callback(ctx context.Context, req *admissionv1.AdmissionRequest, gvk schema.GroupVersionKind) error {
	var toDecode []byte
	if req.Operation == admissionv1.Delete {
		toDecode = req.OldObject.Raw
	} else {
		toDecode = req.Object.Raw
	}
	if toDecode == nil {
		logger := logging.FromContext(ctx)
		logger.Errorf("No incoming object found: %v for verb %v", gvk, req.Operation)
		return nil
	}

	// Generically callback if any are provided for the resource.
	if c, ok := ac.callbacks[gvk]; ok {
		if _, supported := c.supportedVerbs[req.Operation]; supported {
			unstruct := &unstructured.Unstructured{}
			if err := json.Unmarshal(toDecode, unstruct); err != nil {
				return fmt.Errorf("cannot decode incoming new object: %w", err)
			}

			return c.function(ctx, unstruct)
		}
	}

	return nil
}
