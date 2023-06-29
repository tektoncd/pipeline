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

	lru "github.com/hashicorp/golang-lru"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	customrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/customrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	cloudEventClient cloudevent.CEClient
	cacheClient      *lru.Cache
}

func (c *Reconciler) GetCloudEventsClient() cloudevent.CEClient {
	return c.cloudEventClient
}

func (c *Reconciler) GetCacheClient() *lru.Cache {
	return c.cacheClient
}

func (c *Reconciler) SetCloudEventsClient(client cloudevent.CEClient) {
	c.cloudEventClient = client
}

func (c *Reconciler) SetCacheClient(client *lru.Cache) {
	c.cacheClient = client
}

// Check that our Reconciler implements customrunreconciler.Interface
var (
	_ customrunreconciler.Interface = (*Reconciler)(nil)
)

// ReconcileKind oberves the resource conditions and triggers notifications accordingly
func (c *Reconciler) ReconcileKind(ctx context.Context, customRun *v1beta1.CustomRun) pkgreconciler.Event {
	configs := config.FromContextOrDefaults(ctx)
	if configs.FeatureFlags.SendCloudEventsForRuns {
		// Custom task controllers may be sending events for "CustomRuns" associated
		// to the custom tasks they control. To avoid sending duplicate events,
		// CloudEvents for "CustomRuns" are only sent when enabled

		return notifications.ReconcileRuntimeObject(ctx, c, customRun)
	}
	return nil
}
