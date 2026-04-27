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

package taskrun

import (
	"context"

	bc "github.com/allegro/bigcache/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	taskrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1/taskrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// TracerName is the name of the tracer for the taskrun notifications reconciler.
	TracerName = "TaskRunNotificationsReconciler"
)

// Reconciler implements controller.Reconciler for TaskRun resources.
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

// GetCloudEventsClient returns the cloud event client.

func (r *Reconciler) GetCloudEventsClient() cloudevent.CEClient {
	return r.cloudEventClient
}

// GetCacheClient returns the cache client.
func (r *Reconciler) GetCacheClient() *bc.BigCache {
	return r.cacheClient
}

// Check that our Reconciler implements taskrunreconciler.Interface
var _ taskrunreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, tr *v1.TaskRun) pkgreconciler.Event {

	ctx, span := r.tracerProvider.Tracer(TracerName).Start(ctx, "ReconcileKind",
		trace.WithAttributes(
			attribute.String("taskrun.name", tr.Name),
			attribute.String("taskrun.namespace", tr.Namespace),
		),
	)
	defer span.End()

	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling TaskRun notifications for %s/%s", tr.Namespace, tr.Name)

	// We pass 'r' as it satisfies the EventClientsProvider interface.
	err := notifications.ReconcileRunObject(ctx, r, tr)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}
