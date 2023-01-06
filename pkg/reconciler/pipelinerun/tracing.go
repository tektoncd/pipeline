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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"knative.dev/pkg/logging"
)

const (
	// Tracer is the name of the tracer
	Tracer = "PipelineRunReconciler"
	// SpanContextAnnotation is the name of the Annotation for storing SpanContext
	SpanContextAnnotation = "tekton.dev/pipelinerunSpanContext"
	// TaskRunSpanContextAnnotation is the name of the Annotation used for propogating SpanContext to TaskRun
	TaskRunSpanContextAnnotation = "tekton.dev/taskrunSpanContext"
)

// initialize tracing by creating the root span and injecting the spanContext
// spanContext is propogated through annotations in the CR
func initTracing(ctx context.Context, tracerProvider trace.TracerProvider, pr *v1beta1.PipelineRun) context.Context {
	logger := logging.FromContext(ctx)
	carrier := make(map[string]string)

	pro := otel.GetTextMapPropagator()
	prAnnotations := pr.Annotations
	if prAnnotations == nil {
		prAnnotations = make(map[string]string)
	}

	// if spanContext is not present in the CR, create new parent trace and
	// add the marshalled spancontext to the annotations
	if _, e := prAnnotations[SpanContextAnnotation]; !e {
		ctxWithTrace, span := tracerProvider.Tracer(Tracer).Start(ctx, "PipelineRun:Reconciler")
		defer span.End()
		span.SetAttributes(attribute.String("pipeline", pr.Name), attribute.String("namespace", pr.Namespace))

		if !span.SpanContext().HasTraceID() {
			logger.Debug("tracerProvider doesn't provide a traceId, tracing is disabled")
			return ctx
		}

		marshalled, err := getMarshalledSpanFromContext(ctxWithTrace)
		if err != nil {
			logger.Error("Unable to marshal spancontext, err: %s", err)
			return ctx
		}
		logger.Debug("adding spancontext", marshalled)
		prAnnotations[SpanContextAnnotation] = marshalled
		pr.Annotations = prAnnotations
		span.AddEvent("updating TaskRun CR with SpanContext annotations")
		return ctxWithTrace
	}

	err := json.Unmarshal([]byte(pr.Annotations[SpanContextAnnotation]), &carrier)
	if err != nil {
		logger.Error("unable to unmarshal spancontext, err: %s", err)
	}

	logger.Debug("got traceContext %s", carrier)

	return pro.Extract(ctx, propagation.MapCarrier(carrier))
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
