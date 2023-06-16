/*
Copyright 2022 The Tekton Authors

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

package taskrun

import (
	"context"
	"encoding/json"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"knative.dev/pkg/logging"
)

const (
	// TracerName is the name of the tracer
	TracerName = "TaskRunReconciler"
	// SpanContextAnnotation is the name of the Annotation used for propogating SpanContext
	SpanContextAnnotation = "tekton.dev/taskrunSpanContext"
)

// initialize tracing by creating the root span and injecting the
// spanContext is propogated through annotations in the CR
func initTracing(ctx context.Context, tracerProvider trace.TracerProvider, tr *v1.TaskRun) context.Context {
	logger := logging.FromContext(ctx)
	pro := otel.GetTextMapPropagator()

	// SpanContext was created already
	if tr.Status.SpanContext != nil && len(tr.Status.SpanContext) > 0 {
		return pro.Extract(ctx, propagation.MapCarrier(tr.Status.SpanContext))
	}

	spanContext := make(map[string]string)

	// SpanContext was propogated through annotations
	if tr.Annotations != nil && tr.Annotations[SpanContextAnnotation] != "" {
		err := json.Unmarshal([]byte(tr.Annotations[SpanContextAnnotation]), &spanContext)
		if err != nil {
			logger.Error("unable to unmarshal spancontext, err: %s", err)
		}

		tr.Status.SpanContext = spanContext
		return pro.Extract(ctx, propagation.MapCarrier(tr.Status.SpanContext))
	}

	// Create a new root span since there was no parent spanContext provided through annotations
	ctxWithTrace, span := tracerProvider.Tracer(TracerName).Start(ctx, "TaskRun:Reconciler")
	defer span.End()
	span.SetAttributes(attribute.String("taskrun", tr.Name), attribute.String("namespace", tr.Namespace))

	pro.Inject(ctxWithTrace, propagation.MapCarrier(spanContext))

	logger.Debug("got tracing carrier", spanContext)
	if len(spanContext) == 0 {
		logger.Debug("tracerProvider doesn't provide a traceId, tracing is disabled")
		return ctx
	}

	span.AddEvent("updating TaskRun status with SpanContext")
	tr.Status.SpanContext = spanContext
	return ctxWithTrace
}
