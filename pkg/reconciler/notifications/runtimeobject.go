/*
Copyright 2023 The Tekton Authors

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

package notifications

import (
	"context"

	bc "github.com/allegro/bigcache/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cache"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
)

const (
	// TracerName is the name of the tracer for the notifications reconciler.
	TracerName = "Notifications"
)

// EventClientsProvider provides read access to cloud event dependencies
type EventClientsProvider interface {
	GetCloudEventsClient() cloudevent.CEClient
	GetCacheClient() *bc.BigCache
}

// ReconcileRunObject observes a v1beta1.RunObject and triggers notifications.
func ReconcileRunObject(ctx context.Context, e EventClientsProvider, readOnlyRun v1beta1.RunObject) pkgreconciler.Event {
	var span trace.Span
	ctx, span = otel.GetTracerProvider().Tracer(TracerName).Start(ctx, "ReconcileRunObject")
	defer span.End()
	logger := logging.FromContext(ctx)
	ctx = cloudevent.ToContext(ctx, e.GetCloudEventsClient())
	ctx = cache.ToContext(ctx, e.GetCacheClient())

	logger.Infof("reconciling %s", readOnlyRun.GetObjectMeta().GetName())

	condition := readOnlyRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
	logger.Debugf("%s %s, condition: %s", readOnlyRun.GetObjectKind().GroupVersionKind().Kind, readOnlyRun.GetObjectMeta().GetName(), condition)

	cloudevent.EmitCloudEvents(ctx, readOnlyRun)
	return nil
}
