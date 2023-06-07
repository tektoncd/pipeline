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

package pipelinerun

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"knative.dev/pkg/logging"
)

const (
	// TracerName is the name of the tracer
	TracerName = "PipelineRunReconciler"
	// SpanContextAnnotation is the name of the Annotation for storing SpanContext
	SpanContextAnnotation = "tekton.dev/pipelinerunSpanContext"
	// TaskRunSpanContextAnnotation is the name of the Annotation used for propogating SpanContext to TaskRun
	TaskRunSpanContextAnnotation = "tekton.dev/taskrunSpanContext"
)

// initialize tracing by creating the root span and injecting the
// spanContext is propogated through annotations in the CR
func initTracing(ctx context.Context, tracerProvider trace.TracerProvider, pr *v1.PipelineRun) context.Context {
	logger := logging.FromContext(ctx)
	pro := otel.GetTextMapPropagator()

	// SpanContext was created already
	if pr.Status.SpanContext != nil && len(pr.Status.SpanContext) > 0 {
		return pro.Extract(ctx, propagation.MapCarrier(pr.Status.SpanContext))
	}

	spanContext := make(map[string]string)

	// SpanContext was propogated through annotations
	if pr.Annotations != nil && pr.Annotations[SpanContextAnnotation] != "" {
		err := json.Unmarshal([]byte(pr.Annotations[SpanContextAnnotation]), &spanContext)
		if err != nil {
			logger.Error("unable to unmarshal spancontext, err: %s", err)
		}

		pr.Status.SpanContext = spanContext
		return pro.Extract(ctx, propagation.MapCarrier(pr.Status.SpanContext))
	}

	// Create a new root span since there was no parent spanContext provided through annotations
	ctxWithTrace, span := tracerProvider.Tracer(TracerName).Start(ctx, "PipelineRun:Reconciler")
	defer span.End()
	span.SetAttributes(attribute.String("pipelinerun", pr.Name), attribute.String("namespace", pr.Namespace))

	pro.Inject(ctxWithTrace, propagation.MapCarrier(spanContext))

	logger.Debug("got tracing carrier", spanContext)
	if len(spanContext) == 0 {
		logger.Debug("tracerProvider doesn't provide a traceId, tracing is disabled")
		return ctx
	}

	span.AddEvent("updating PipelineRun status with SpanContext")
	pr.Status.SpanContext = spanContext
	return ctxWithTrace
}

// Extract spanContext from the context and return it as json encoded string
func getMarshalledSpanFromContext(ctx context.Context) (string, error) {
	carrier := make(map[string]string)
	pro := otel.GetTextMapPropagator()

	pro.Inject(ctx, propagation.MapCarrier(carrier))

	if len(carrier) == 0 {
		return "", fmt.Errorf("spanContext not present in the context, unable to marshall")
	}

	marshalled, err := json.Marshal(carrier)
	if err != nil {
		return "", err
	}
	if len(marshalled) >= 1024 {
		return "", fmt.Errorf("marshalled spanContext size is too big")
	}
	return string(marshalled), nil
}
