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
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cache"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	cloudEventClient cloudevent.CEClient
	cacheClient      *lru.Cache
}

// Check that our Reconciler implements customrunreconciler.Interface
var (
	_ customrunreconciler.Interface = (*Reconciler)(nil)
)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CustomRun
// resource with the current status of the resource.
func (c *Reconciler) ReconcileKind(ctx context.Context, customRun *v1beta1.CustomRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	configs := config.FromContextOrDefaults(ctx)
	ctx = cloudevent.ToContext(ctx, c.cloudEventClient)
	ctx = cache.ToContext(ctx, c.cacheClient)
	logger.Infof("Reconciling %s", customRun.Name)

	// Create a copy of the CustomRun object, just in case, to avoid sync'ing changes
	customRunEvents := *customRun.DeepCopy()

	if configs.FeatureFlags.SendCloudEventsForRuns {
		// Custom task controllers may be sending events for "CustomRuns" associated
		// to the custom tasks they control. To avoid sending duplicate events,
		// CloudEvents for "CustomRuns" are only sent when enabled

		// Read and log the condition
		condition := customRunEvents.Status.GetCondition(apis.ConditionSucceeded)
		logger.Debugf("Emitting cloudevent for %s, condition: %s", customRunEvents.Name, condition)

		events.EmitCloudEvents(ctx, &customRunEvents)
	}

	return nil
}
