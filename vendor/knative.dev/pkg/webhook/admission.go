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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
)

const (
	// AdmissionReviewUID is the key used to represent the admission review
	// request/response UID in logs
	AdmissionReviewUID = "admissionreview/uid"

	// AdmissionReviewAllowed is the key used to represent whether or not
	// the admission request was permitted in logs
	AdmissionReviewAllowed = "admissionreview/allowed"

	// AdmissionReviewResult is the key used to represent extra details into
	// why an admission request was denied in logs
	AdmissionReviewResult = "admissionreview/result"

	// AdmissionReviewPatchType is the key used to represent the type of Patch in logs
	AdmissionReviewPatchType = "admissionreview/patchtype"
)

// AdmissionController provides the interface for different admission controllers
type AdmissionController interface {
	// Path returns the path that this particular admission controller serves on.
	Path() string

	// Admit is the callback which is invoked when an HTTPS request comes in on Path().
	Admit(context.Context, *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse
}

// StatelessAdmissionController is implemented by AdmissionControllers where Admit may be safely
// called before informers have finished syncing.  This is implemented by inlining
// StatelessAdmissionImpl in your Go type.
type StatelessAdmissionController interface {
	// A silly name that should avoid collisions.
	ThisTypeDoesNotDependOnInformerState()
}

// MakeErrorStatus creates an 'BadRequest' error AdmissionResponse
func MakeErrorStatus(reason string, args ...interface{}) *admissionv1.AdmissionResponse {
	result := apierrors.NewBadRequest(fmt.Sprintf(reason, args...)).Status()
	return &admissionv1.AdmissionResponse{
		Result:  &result,
		Allowed: false,
	}
}

func admissionHandler(rootLogger *zap.SugaredLogger, stats StatsReporter, c AdmissionController, synced <-chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := c.(StatelessAdmissionController); ok {
			// Stateless admission controllers do not require Informers to have
			// finished syncing before Admit is called.
		} else {
			// Don't allow admission control requests through until we have been
			// notified that informers have been synchronized.
			<-synced
		}

		var ttStart = time.Now()
		logger := rootLogger
		logger.Infof("Webhook ServeHTTP request=%#v", r)

		var review admissionv1.AdmissionReview
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			http.Error(w, fmt.Sprintf("could not decode body: %v", err), http.StatusBadRequest)
			return
		}

		logger = logger.With(
			logkey.Kind, review.Request.Kind.String(),
			logkey.Namespace, review.Request.Namespace,
			logkey.Name, review.Request.Name,
			logkey.Operation, string(review.Request.Operation),
			logkey.Resource, review.Request.Resource.String(),
			logkey.SubResource, review.Request.SubResource,
			logkey.UserInfo, fmt.Sprint(review.Request.UserInfo))

		ctx := logging.WithLogger(r.Context(), logger)

		response := admissionv1.AdmissionReview{
			// Use the same type meta as the request - this is required by the K8s API
			// note: v1beta1 & v1 AdmissionReview shapes are identical so even though
			// we're using v1 types we still support v1beta1 admission requests
			TypeMeta: review.TypeMeta,
		}

		reviewResponse := c.Admit(ctx, review.Request)
		var patchType string
		if reviewResponse.PatchType != nil {
			patchType = string(*reviewResponse.PatchType)
		}

		if !reviewResponse.Allowed || reviewResponse.PatchType != nil || response.Response == nil {
			response.Response = reviewResponse
		}
		response.Response.UID = review.Request.UID

		logger = logger.With(
			AdmissionReviewUID, string(reviewResponse.UID),
			AdmissionReviewAllowed, reviewResponse.Allowed,
			AdmissionReviewResult, reviewResponse.Result.String())

		logger.Infof("remote admission controller audit annotations=%#v", reviewResponse.AuditAnnotations)
		logger.Debugf("AdmissionReview patch={ type: %s, body: %s }", patchType, string(reviewResponse.Patch))

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
			return
		}

		if stats != nil {
			// Only report valid requests
			stats.ReportRequest(review.Request, response.Response, time.Since(ttStart))
		}
	}
}

// Inline this type to implement StatelessAdmissionController.
type StatelessAdmissionImpl struct{}

func (sai StatelessAdmissionImpl) ThisTypeDoesNotDependOnInformerState() {}
