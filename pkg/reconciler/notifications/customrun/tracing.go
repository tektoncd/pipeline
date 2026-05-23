/*
Copyright 2026 The Tekton Authors

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

package customrun

import (
	"context"
	"encoding/json"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"knative.dev/pkg/logging"
)

const (
	// TracerName is the name of the tracer.
	TracerName = "CustomRunNotificationsReconciler"
	// TracerProviderName is the service name registered with the tracing provider.
	TracerProviderName = "customrun-notifications-reconciler"
	// SpanContextAnnotation is the annotation key used to propagate span context.
	SpanContextAnnotation = "tekton.dev/customrunSpanContext"
)

// initTracing initializes tracing by extracting or creating a root span and
// injecting the span context into the returned context. Span context is
// propagated via the SpanContextAnnotation annotation on the CustomRun.
func initTracing(ctx context.Context, tracerProvider trace.TracerProvider, cr *v1beta1.CustomRun) context.Context {
	logger := logging.FromContext(ctx)
	pro := otel.GetTextMapPropagator()

	spanContext := make(map[string]string)

	// SpanContext was propagated through annotations.
	if cr.Annotations != nil && cr.Annotations[SpanContextAnnotation] != "" {
		if err := json.Unmarshal([]byte(cr.Annotations[SpanContextAnnotation]), &spanContext); err != nil {
			logger.Error("unable to unmarshal spancontext, err: %s", err)
		}
		return pro.Extract(ctx, propagation.MapCarrier(spanContext))
	}

	// No parent span context — create a new root span.
	ctxWithTrace, span := tracerProvider.Tracer(TracerName).Start(ctx, "CustomRun:Reconciler")
	defer span.End()
	span.SetAttributes(attribute.String("customrun", cr.Name), attribute.String("namespace", cr.Namespace))

	pro.Inject(ctxWithTrace, propagation.MapCarrier(spanContext))

	logger.Debug("got tracing carrier", spanContext)
	if len(spanContext) == 0 {
		logger.Debug("tracerProvider doesn't provide a traceId, tracing is disabled")
		return ctx
	}

	return ctxWithTrace
}
