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

package run

import (
	"context"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	runreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1alpha1/run"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cache"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	_ "github.com/tektoncd/pipeline/pkg/taskrunmetrics/fake" // Make sure the taskrunmetrics are setup
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	cloudEventClient cloudevent.CEClient
	cacheClient      *lru.Cache
	waitGroup        *sync.WaitGroup
}

// Check that our Reconciler implements runreconciler.Interface
var (
	_ runreconciler.Interface = (*Reconciler)(nil)
)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Task Run
// resource with the current status of the resource.
func (c *Reconciler) ReconcileKind(ctx context.Context, run *v1alpha1.Run) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	configs := config.FromContextOrDefaults(ctx)
	ctx = cloudevent.ToContext(ctx, c.cloudEventClient)
	ctx = cache.ToContext(ctx, c.cacheClient)
	// ctx = cache.ToContext(ctx, c.cacheClient)
	logger.Infof("Reconciling %s", run.Name)

	// Create a copy of the run object, just in case, to avoid sync'ing changes
	runEvents := *run.DeepCopy()

	if configs.FeatureFlags.SendCloudEventsForRuns {
		// Custom task controllers may be sending events for "Runs" associated
		// to the custom tasks they control. To avoid sending duplicate events,
		// CloudEvents for "Runs" are only sent when enabled

		// Read and log the condition
		condition := runEvents.Status.GetCondition(apis.ConditionSucceeded)
		logger.Debugf("Emitting cloudevent for %s, condition: %s", runEvents.Name, condition)

		events.EmitCloudEvents(ctx, &runEvents, c.waitGroup)
	}

	return nil
}
