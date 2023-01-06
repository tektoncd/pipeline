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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"knative.dev/pkg/logging"
)

const (
	// Tracer is the name of the tracer
	Tracer = "TaskRunReconciler"
	// SpanContextAnnotation is the name of the Annotation used for propogating SpanContext
	SpanContextAnnotation = "tekton.dev/taskrunSpanContext"
)

// initialize tracing by creating the root span and injecting the spanContext
// spanContext is propogated through annotations in the CR
func initTracing(ctx context.Context, tracerProvider trace.TracerProvider, tr *v1beta1.TaskRun) context.Context {
	logger := logging.FromContext(ctx)
	carrier := make(map[string]string)

	pro := otel.GetTextMapPropagator()
	trAnnotations := tr.Annotations
	if trAnnotations == nil {
		trAnnotations = make(map[string]string)
	}

	// if spanContext is not present in the CR, create new parent trace and
	// add the marshalled spancontext to the annotations
	if _, e := trAnnotations[SpanContextAnnotation]; !e {
		ctxWithTrace, span := tracerProvider.Tracer(Tracer).Start(ctx, "TaskRun:Reconciler")
		defer span.End()
		span.SetAttributes(attribute.String("task", tr.Name), attribute.String("namespace", tr.Namespace))

		pro.Inject(ctxWithTrace, propagation.MapCarrier(carrier))

		logger.Info("got carrier", carrier)
		if len(carrier) == 0 {
			logger.Debug("tracerProvider doesn't provide a traceId, tracing is disabled")
			return ctx
		}

		marshalled, err := json.Marshal(carrier)
		if err != nil {
			logger.Error("unable to marshal spancontext, err: %s", err)
			return ctx
		}
		logger.Debug("adding spancontext", "ctx", string(marshalled))
		trAnnotations[SpanContextAnnotation] = string(marshalled)
		tr.Annotations = trAnnotations
		span.AddEvent("updating TaskRun CR with SpanContext annotations")
		return ctxWithTrace
	}

	err := json.Unmarshal([]byte(tr.Annotations[SpanContextAnnotation]), &carrier)
	if err != nil {
		logger.Error("unable to unmarshal spancontext, err: %s", err)
	}

	return pro.Extract(ctx, propagation.MapCarrier(carrier))
}
