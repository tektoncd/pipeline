/*
Copyright 2019 The Tekton Authors

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

	bc "github.com/allegro/bigcache/v3"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	customrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/customrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/logging"
)

const (
	// TracerName is the name of the tracer for CustomRun notifications
	TracerName = "CustomRunNotificationsReconciler"
	// SpanContextAnnotation is the annotation key for span context propagation
	SpanContextAnnotation = "tekton.dev/customrunSpanContext"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	cloudEventClient cloudevent.CEClient
	cacheClient      *bc.BigCache
}

// NewReconciler creates a new Reconciler with the given clients.
func NewReconciler(ceClient cloudevent.CEClient, cacheClient *bc.BigCache) *Reconciler {
	return &Reconciler{
		cloudEventClient: ceClient,
		cacheClient:      cacheClient,
	}
}

func (c *Reconciler) GetCloudEventsClient() cloudevent.CEClient {
	return c.cloudEventClient
}

func (c *Reconciler) GetCacheClient() *bc.BigCache {
	return c.cacheClient
}

// Check that our Reconciler implements customrunreconciler.Interface
var (
	_ customrunreconciler.Interface = (*Reconciler)(nil)
)

// InitTracing extracts span context from CustomRun annotations and returns a context with tracing initialized.
// If span context exists in annotations, it extracts and propagates it to the context.
// Otherwise, it creates a new root span for the reconciliation.
func InitTracing(ctx context.Context, tp trace.TracerProvider, customRun *v1beta1.CustomRun) context.Context {
	logger := logging.FromContext(ctx)
	pro := otel.GetTextMapPropagator()

	spanContextMap := make(map[string]string)

	// Check if span context was propagated through annotations
	if customRun.Annotations != nil && customRun.Annotations[SpanContextAnnotation] != "" {
		err := json.Unmarshal([]byte(customRun.Annotations[SpanContextAnnotation]), &spanContextMap)
		if err != nil {
			logger.Error("unable to unmarshal spancontext from annotation, err: %v", err)
			// Fall through to create a new span
		} else {
			// Successfully extracted span context from annotations
			return pro.Extract(ctx, propagation.MapCarrier(spanContextMap))
		}
	}

	// Create a new root span since there was no parent spanContext provided through annotations
	ctxWithTrace, span := tp.Tracer(TracerName).Start(ctx, "CustomRunNotifications:Reconciler")
	defer span.End()
	span.SetAttributes(
		attribute.String("customrun", customRun.Name),
		attribute.String("namespace", customRun.Namespace),
	)

	// Inject the new span context into the annotations for downstream propagation
	pro.Inject(ctxWithTrace, propagation.MapCarrier(spanContextMap))

	if len(spanContextMap) == 0 {
		logger.Debug("tracerProvider doesn't provide a traceId, tracing is disabled")
		return ctx
	}

	return ctxWithTrace
}

// ReconcileKind oberves the resource conditions and triggers notifications accordingly
func (c *Reconciler) ReconcileKind(ctx context.Context, customRun *v1beta1.CustomRun) pkgreconciler.Event {
	// Custom task controllers may be sending events for "CustomRuns" associated
	// to the custom tasks they control. To avoid sending duplicate events,
	// CloudEvents for "CustomRuns" are only sent when enabled via send-cloudevents-for-runs.

	// Initialize tracing by extracting span context from annotations if present
	ctx = InitTracing(ctx, otel.GetTracerProvider(), customRun)

	configs := config.FromContextOrDefaults(ctx)
	if !configs.FeatureFlags.SendCloudEventsForRuns {
		return nil
	}

	ctx, span := otel.Tracer(TracerName).Start(ctx, "ReconcileKind")
	defer span.End()
	span.SetAttributes(
		attribute.String("customrun", customRun.Name),
		attribute.String("namespace", customRun.Namespace),
	)

	return notifications.ReconcileRunObject(ctx, c, customRun)
}
