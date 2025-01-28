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
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

func conversionHandler(rootLogger *zap.SugaredLogger, stats StatsReporter, c ConversionController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ttStart := time.Now()
		logger := rootLogger
		logger.Infof("Webhook ServeHTTP request=%#v", r)

		var review apixv1.ConversionReview
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			http.Error(w, fmt.Sprint("could not decode body:", err), http.StatusBadRequest)
			return
		}

		logger = logger.With(
			zap.String("uid", string(review.Request.UID)),
			zap.String("desiredAPIVersion", review.Request.DesiredAPIVersion),
		)

		ctx := logging.WithLogger(r.Context(), logger)
		ctx = apis.WithHTTPRequest(ctx, r)

		response := apixv1.ConversionReview{
			// Use the same type meta as the request - this is required by the K8s API
			// note: v1beta1 & v1 ConversionReview shapes are identical so even though
			// we're using v1 types we still support v1beta1 conversion requests
			TypeMeta: review.TypeMeta,
			Response: c.Convert(ctx, review.Request),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprint("could not encode response:", err), http.StatusInternalServerError)
			return
		}

		if stats != nil {
			// Only report valid requests
			stats.ReportConversionRequest(review.Request, response.Response, time.Since(ttStart))
		}
	}
}
