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
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

// ConversionController provides the interface for different conversion controllers
type ConversionController interface {
	// Path returns the path that this particular conversion controller serves on.
	Path() string

	// Convert is the callback which is invoked when an HTTPS request comes in on Path().
	Convert(context.Context, *apixv1.ConversionRequest) *apixv1.ConversionResponse
}

func conversionHandler(wh *Webhook, c ConversionController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := wh.Logger
		logger.Infof("Webhook ServeHTTP request=%#v", r)

		span := trace.SpanFromContext(r.Context())
		// otelhttp middleware creates the labeler
		labeler, _ := otelhttp.LabelerFromContext(r.Context())

		defer func() {
			// otelhttp doesn't add labeler attributes to spans
			// so we have to do it manually
			span.SetAttributes(labeler.Get()...)
		}()

		var review apixv1.ConversionReview
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			msg := fmt.Sprint("could not decode body:", err)
			span.SetStatus(codes.Error, msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		gv, err := parseAPIVersion(review.Request.DesiredAPIVersion)
		if err != nil {
			msg := fmt.Sprint("could parse desired api version:", err)
			span.SetStatus(codes.Error, msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		labeler.Add(
			WebhookTypeAttr.With(WebhookTypeConversion),
			GroupAttr.With(gv.Group),
			VersionAttr.With(gv.Version),
		)

		logger = logger.With(
			zap.String("uid", string(review.Request.UID)),
			zap.String("desiredAPIVersion", review.Request.DesiredAPIVersion),
		)

		ctx := logging.WithLogger(r.Context(), logger)
		ctx = apis.WithHTTPRequest(ctx, r)

		ttStart := time.Now()
		response := apixv1.ConversionReview{
			// Use the same type meta as the request - this is required by the K8s API
			// note: v1beta1 & v1 ConversionReview shapes are identical so even though
			// we're using v1 types we still support v1beta1 conversion requests
			TypeMeta: review.TypeMeta,
			Response: c.Convert(ctx, review.Request),
		}

		labeler.Add(
			StatusAttr.With(strings.ToLower(response.Response.Result.Status)),
		)

		if response.Response.Result.Status == metav1.StatusFailure {
			span.SetStatus(codes.Error, response.Response.Result.Message)
		}

		wh.metrics.recordHandlerDuration(ctx, time.Since(ttStart),
			metric.WithAttributes(labeler.Get()...),
		)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprint("could not encode response:", err), http.StatusInternalServerError)
			return
		}

		span.SetStatus(codes.Ok, "")
	}
}

func parseAPIVersion(apiVersion string) (schema.GroupVersion, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		err = fmt.Errorf("desired API version %q is not valid", apiVersion)
		return schema.GroupVersion{}, err
	}

	if !isValidGV(gv) {
		err = fmt.Errorf("desired API version %q is not valid", apiVersion)
		return schema.GroupVersion{}, err
	}

	return gv, nil
}

func isValidGV(gk schema.GroupVersion) bool {
	return gk.Group != "" && gk.Version != ""
}
