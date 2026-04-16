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

	bc "github.com/allegro/bigcache/v3"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	customrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/customrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const tracerName = "CustomRunNotificationsReconciler"

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

// ReconcileKind oberves the resource conditions and triggers notifications accordingly
func (c *Reconciler) ReconcileKind(ctx context.Context, customRun *v1beta1.CustomRun) pkgreconciler.Event {
	// Custom task controllers may be sending events for "CustomRuns" associated
	// to the custom tasks they control. To avoid sending duplicate events,
	// CloudEvents for "CustomRuns" are only sent when enabled via send-cloudevents-for-runs.
	configs := config.FromContextOrDefaults(ctx)
	if !configs.FeatureFlags.SendCloudEventsForRuns {
		return nil
	}

	ctx, span := otel.Tracer(tracerName).Start(ctx, "ReconcileKind")
	defer span.End()
	span.SetAttributes(
		attribute.String("customrun", customRun.Name),
		attribute.String("namespace", customRun.Namespace),
	)

	return notifications.ReconcileRunObject(ctx, c, customRun)
}
