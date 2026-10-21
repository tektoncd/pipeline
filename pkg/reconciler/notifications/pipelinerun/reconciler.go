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

package pipelinerun

import (
	"context"

	bc "github.com/allegro/bigcache/v3"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1/pipelinerun"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// TracerName is the name of the tracer for the pipelinerun notifications reconciler.
	TracerName = "PipelineRunNotificationsReconciler"
)

// Reconciler implements controller.Reconciler for PipelineRun resources.
type Reconciler struct {
	cloudEventClient cloudevent.CEClient
	cacheClient      *bc.BigCache

	// tracerProvider is used to create spans for distributed tracing.
	tracerProvider trace.TracerProvider
}

// NewReconciler creates a new Reconciler with the given clients.
func NewReconciler(ceClient cloudevent.CEClient, cacheClient *bc.BigCache) *Reconciler {
	return &Reconciler{
		cloudEventClient: ceClient,
		cacheClient:      cacheClient,
		tracerProvider:   otel.GetTracerProvider(),
	}
}

func (c *Reconciler) GetCloudEventsClient() cloudevent.CEClient {
	return c.cloudEventClient
}

func (c *Reconciler) GetCacheClient() *bc.BigCache {
	return c.cacheClient
}

// Check that our Reconciler implements pipelinerunreconciler.Interface
var _ pipelinerunreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind observes the resource conditions and triggers notifications accordingly.
func (c *Reconciler) ReconcileKind(ctx context.Context, pr *v1.PipelineRun) pkgreconciler.Event {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "ReconcileKind",
		trace.WithAttributes(
			attribute.String("pipelinerun.name", pr.Name),
			attribute.String("pipelinerun.namespace", pr.Namespace),
		),
	)
	defer span.End()

	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling PipelineRun notifications for %s/%s", pr.Namespace, pr.Name)

	err := notifications.ReconcileRunObject(ctx, c, pr)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	return err
}
